package goplt

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pelletier/go-toml/v2"
)

// VariableKind represents the type of a template variable.
type VariableKind string

const (
	KindInput        VariableKind = "input"        // single-line text input; use Required = true for mandatory fields
	KindBool         VariableKind = "bool"         // yes/no confirm
	KindStringChoice VariableKind = "stringChoice" // select from list; first item is selected by default
	KindStringList   VariableKind = "stringList"   // comma-separated list of strings
	KindInt          VariableKind = "int"          // free integer input
	KindIntChoice    VariableKind = "intChoice"    // select from a list of integers; first item is selected by default

	// KindText is an alias for KindInput kept for backward compatibility with
	// library consumers that reference the old constant name.
	KindText = KindInput
)

// Variable describes a single template variable from goplt.toml.
type Variable struct {
	Name string // PascalCase
	Kind VariableKind
	// Typed value: string, bool, []string, int, or []int — depends on Kind.
	Value       any
	Required    bool   // KindInput and KindStringList: validation fails when user submits empty value
	Description string // optional; shown as subtitle in the TUI
	Min         *int   // KindInt only: inclusive lower bound (nil = no constraint)
	Max         *int   // KindInt only: inclusive upper bound (nil = no constraint)
}

// PostGenHooks is a list of post-generation shell commands.
type PostGenHooks []string

// Hooks holds the hook commands declared under [hooks] in goplt.toml.
type Hooks struct {
	PostGenHooks PostGenHooks `mapstructure:"post-generate"`
}

// Manifest holds the parsed content of a goplt.toml file.
type Manifest struct {
	Description string // required one-line summary of what this template generates
	Tags        []string // optional; informational labels for filtering and discovery
	Authors     []string // optional; human-readable author names or handles
	Variables   []Variable
	// unrendered path prefix → Go template boolean expression
	Conditions map[string]string
	Hooks      Hooks
	// optional Go template expression; rendered against vars to determine output subdirectory
	TargetDir string
	// template action delimiters; defaults to ["{{", "}}"]
	Delimiters [2]string
	// path pattern (contains {{.item}}) → [PascalCase varName]; single-element array in v1
	Loops map[string][]string
}

// NormalizeKey converts hyphen-case, snake_case, or camelCase to PascalCase.
// "with-connect", "with_connect", and "withConnect" all produce "WithConnect".
func NormalizeKey(s string) string {
	if s == "" {
		return s
	}

	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})

	if len(parts) <= 1 {
		r, size := utf8.DecodeRuneInString(s)
		return string(unicode.ToUpper(r)) + s[size:]
	}

	var b strings.Builder

	for _, p := range parts {
		if p == "" {
			continue
		}

		r, size := utf8.DecodeRuneInString(p)
		_, _ = b.WriteRune(unicode.ToUpper(r))
		_, _ = b.WriteString(p[size:])
	}

	return b.String()
}

// rawManifest is the intermediate representation decoded from goplt.toml.
type rawManifest struct {
	Description string              `mapstructure:"description"`
	Tags        []string            `mapstructure:"tags"`
	Authors     []string            `mapstructure:"authors"`
	Variables   map[string]any      `mapstructure:"variables"`
	Conditions  map[string]string   `mapstructure:"conditions"`
	Hooks       rawHooks            `mapstructure:"hooks"`
	TargetDir   string              `mapstructure:"target-dir"`
	Delimiters  []string            `mapstructure:"delimiters"`
	Loops       map[string][]string `mapstructure:"loops"`
}

type rawHooks struct {
	PostGenerate []string `mapstructure:"post-generate"`
}

// LoadManifest reads and parses goplt.toml from fsys.
// Variable names are normalised to PascalCase via NormalizeKey.
func LoadManifest(fsys fs.FS) (*Manifest, error) {
	data, err := fs.ReadFile(fsys, "goplt.toml")
	if err != nil {
		return nil, fmt.Errorf("read goplt.toml: %w", err)
	}

	var intermediate map[string]any

	if err := toml.Unmarshal(data, &intermediate); err != nil {
		return nil, fmt.Errorf("parse goplt.toml: %w", err)
	}

	var raw rawManifest

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &raw,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create decoder: %w", err)
	}

	if err := decoder.Decode(intermediate); err != nil {
		return nil, fmt.Errorf("decode goplt.toml: %w", err)
	}

	if raw.Description == "" {
		return nil, errors.New("goplt.toml: missing required field \"description\"")
	}

	tags := raw.Tags
	if tags == nil {
		tags = []string{}
	}

	authors := raw.Authors
	if authors == nil {
		authors = []string{}
	}

	m := &Manifest{
		Description: raw.Description,
		Tags:        tags,
		Authors:     authors,
		Conditions:  make(map[string]string, len(raw.Conditions)),
		Hooks: Hooks{
			PostGenHooks: PostGenHooks(raw.Hooks.PostGenerate),
		},
		TargetDir: raw.TargetDir,
		Loops:     make(map[string][]string, len(raw.Loops)),
	}

	maps.Copy(m.Conditions, raw.Conditions)
	maps.Copy(m.Loops, raw.Loops)

	// Normalize varName strings in each loops entry to PascalCase (matching Variable.Name).
	for pattern, names := range m.Loops {
		normalized := make([]string, len(names))
		for i, n := range names {
			normalized[i] = NormalizeKey(n)
		}
		m.Loops[pattern] = normalized
	}

	delimiters, err := parseDelimiters(raw.Delimiters)
	if err != nil {
		return nil, err
	}
	m.Delimiters = delimiters

	for rawName, val := range raw.Variables {
		v, err := parseVariable(rawName, val)
		if err != nil {
			return nil, err
		}

		m.Variables = append(m.Variables, v)
	}

	slices.SortFunc(m.Variables, func(a, b Variable) int {
		return strings.Compare(a.Name, b.Name)
	})

	if err := validateIntBounds(m.Variables); err != nil {
		return nil, err
	}

	if err := validateLoops(m.Loops, m.Variables, m.Delimiters); err != nil {
		return nil, err
	}

	return m, nil
}

// validateIntBounds checks that KindInt variables have consistent min/max and
// that the default value falls within declared bounds.
func validateIntBounds(vars []Variable) error {
	for _, v := range vars {
		if v.Kind != KindInt {
			continue
		}

		intVal, _ := v.Value.(int)

		if v.Min != nil && v.Max != nil && *v.Min > *v.Max {
			return fmt.Errorf("variable %q: min (%d) must be ≤ max (%d)", v.Name, *v.Min, *v.Max)
		}

		if v.Min != nil && intVal < *v.Min {
			return fmt.Errorf("variable %q: default value (%d) is below min (%d)", v.Name, intVal, *v.Min)
		}

		if v.Max != nil && intVal > *v.Max {
			return fmt.Errorf("variable %q: default value (%d) exceeds max (%d)", v.Name, intVal, *v.Max)
		}
	}

	return nil
}

func parseVariable(rawName string, val any) (Variable, error) {
	v := Variable{Name: NormalizeKey(rawName)}

	switch tv := val.(type) {
	case string:
		v.Kind = KindInput
		v.Value = tv
		if tv == "" {
			v.Required = true
		}

	case bool:
		v.Kind = KindBool
		v.Value = tv

	case int64:
		v.Kind = KindInt
		v.Value = int(tv)

	case []any:
		return parseAnySlice(rawName, tv)

	case map[string]any:
		parsed, err := parseSubTableVariable(rawName, tv)
		if err != nil {
			return Variable{}, err
		}

		v = parsed

	default:
		return Variable{}, fmt.Errorf(
			"variable %q: unsupported type %T (use string, bool, integer, []string, []integer, or sub-table with kind)",
			rawName, val,
		)
	}

	return v, nil
}

// parseAnySlice handles the []any case in parseVariable: detects whether the
// slice holds integers (→ KindIntChoice) or strings (→ KindStringChoice).
func parseAnySlice(rawName string, tv []any) (Variable, error) {
	v := Variable{Name: NormalizeKey(rawName)}

	if len(tv) == 0 {
		v.Kind = KindStringChoice
		v.Value = []string{}

		return v, nil
	}

	switch tv[0].(type) {
	case int64:
		choices := make([]int, len(tv))

		for i, c := range tv {
			n, ok := c.(int64)
			if !ok {
				return Variable{}, fmt.Errorf("variable %q: mixed-type array; all elements must be integers", rawName)
			}

			choices[i] = int(n)
		}

		v.Kind = KindIntChoice
		v.Value = choices

	case string:
		choices := make([]string, len(tv))

		for i, c := range tv {
			s, ok := c.(string)
			if !ok {
				return Variable{}, fmt.Errorf("variable %q: mixed-type array; all elements must be strings", rawName)
			}

			choices[i] = s
		}

		v.Kind = KindStringChoice
		v.Value = choices

	default:
		return Variable{}, fmt.Errorf("variable %q: choice values must be strings or integers, got %T", rawName, tv[0])
	}

	return v, nil
}

// parseDelimiters validates and returns the [2]string delimiter pair from the
// raw TOML slice. An empty slice returns the default ["{{", "}}"].
func parseDelimiters(raw []string) ([2]string, error) {
	switch len(raw) {
	case 0:
		return [2]string{"{{", "}}"}, nil
	case 2:
		if raw[0] == "" || raw[1] == "" {
			return [2]string{}, errors.New("delimiters: both values must be non-empty")
		}
		if raw[0] == raw[1] {
			return [2]string{}, fmt.Errorf("delimiters: left and right must differ, got %q", raw[0])
		}

		return [2]string{raw[0], raw[1]}, nil
	default:
		return [2]string{}, fmt.Errorf("delimiters: expected exactly 2 values, got %d", len(raw))
	}
}

// stringsFromAnySlice converts []any to []string, silently dropping non-string elements.
func stringsFromAnySlice(rawSlice []any) []string {
	out := make([]string, 0, len(rawSlice))
	for _, c := range rawSlice {
		if s, ok := c.(string); ok {
			out = append(out, s)
		}
	}

	return out
}

// parseSubTableVariable handles the map[string]any case in parseVariable:
// both the new explicit-kind syntax and the legacy "default" syntax.
func parseSubTableVariable(rawName string, tv map[string]any) (Variable, error) {
	var v Variable

	if kindStr, hasKind := tv["kind"].(string); hasKind {
		parsed, err := parseSubTableByKind(rawName, kindStr, tv)
		if err != nil {
			return Variable{}, err
		}

		v = parsed
	} else {
		// Old "default" syntax: map to new fields.
		defaultVal, ok := tv["default"]
		if !ok {
			return Variable{}, fmt.Errorf("variable %q: sub-table form requires a \"kind\" or \"default\" key", rawName)
		}

		format := "goplt: WARNING: variable %q uses deprecated \"default\" syntax;" +
			" use \"kind\", \"value\", and \"required\" instead\n"
		fmt.Fprintf(os.Stderr, format, rawName)

		inner, err := parseVariable(rawName, defaultVal)
		if err != nil {
			return Variable{}, err
		}

		v = inner
	}

	if desc, ok := tv["description"].(string); ok {
		v.Description = desc
	}

	return v, nil
}

// parseSubTableByKind parses a sub-table variable when an explicit kind= key is present.
func parseSubTableByKind(rawName, kindStr string, tv map[string]any) (Variable, error) {
	v := Variable{Name: NormalizeKey(rawName)}
	v.Kind = VariableKind(kindStr)
	v.Required, _ = tv["required"].(bool)

	switch v.Kind {
	case KindInput:
		v.Value, _ = tv["value"].(string)
	case KindBool:
		v.Value, _ = tv["value"].(bool)
	case KindStringChoice, KindStringList:
		rawSlice, _ := tv["value"].([]any)
		v.Value = stringsFromAnySlice(rawSlice)
	case KindInt:
		if n, ok := tv["value"].(int64); ok {
			v.Value = int(n)
		} else {
			v.Value = 0
		}
		if minVal, ok := tv["min"].(int64); ok {
			v.Min = new(int(minVal))
		}
		if maxVal, ok := tv["max"].(int64); ok {
			v.Max = new(int(maxVal))
		}
	case KindIntChoice:
		rawSlice, _ := tv["value"].([]any)
		choices := make([]int, 0, len(rawSlice))
		for _, c := range rawSlice {
			if n, ok := c.(int64); ok {
				choices = append(choices, int(n))
			}
		}
		v.Value = choices
	default:
		return Variable{}, fmt.Errorf("variable %q: unknown kind %q", rawName, kindStr)
	}

	return v, nil
}

// validateLoops checks that every [loops] entry is well-formed:
//   - exactly one variable name per entry (nested loops not yet supported)
//   - the referenced variable is declared and is KindListString
//   - the path pattern contains the {{.item}} placeholder (using configured delimiters)
func validateLoops(loops map[string][]string, vars []Variable, delims [2]string) error {
	if len(loops) == 0 {
		return nil
	}

	byName := make(map[string]Variable, len(vars))
	for _, v := range vars {
		byName[v.Name] = v
	}

	itemPlaceholder := delims[0] + ".item" + delims[1]

	for pattern, varNames := range loops {
		if len(varNames) != 1 {
			format := "loops: entry %q has %d variable names; nested loops are not yet supported (use exactly one)"
			return fmt.Errorf(format, pattern, len(varNames))
		}
		if !strings.Contains(pattern, itemPlaceholder) {
			return fmt.Errorf("loops: pattern %q must contain the item placeholder %q", pattern, itemPlaceholder)
		}
		v, ok := byName[varNames[0]]
		if !ok {
			return fmt.Errorf("loops: pattern %q references undeclared variable %q", pattern, varNames[0])
		}
		if v.Kind != KindStringList {
			format := "loops: pattern %q references variable %q of kind %q; must be %q"
			return fmt.Errorf(format, pattern, varNames[0], v.Kind, KindStringList)
		}
	}

	return nil
}
