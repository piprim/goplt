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

	// KindText is an alias for KindInput kept for backward compatibility with
	// library consumers that reference the old constant name.
	KindText = KindInput
)

// Variable describes a single template variable from template.toml.
type Variable struct {
	Name string // PascalCase
	Kind VariableKind
	// string (KindInput) | bool (KindBool) | []string
	Value       any
	Required    bool   // KindInput and KindListString: validation fails when user submits empty value
	Description string // optional; shown as subtitle in the TUI
}

// PostGenHooks is a list of post-generation shell commands.
type PostGenHooks []string

// Hooks holds the hook commands declared under [hooks] in template.toml.
type Hooks struct {
	PostGenHooks PostGenHooks `mapstructure:"post-generate"`
}

// Manifest holds the parsed content of a template.toml file.
type Manifest struct {
	Variables []Variable
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

// rawManifest is the intermediate representation decoded from template.toml.
type rawManifest struct {
	Variables  map[string]any      `mapstructure:"variables"`
	Conditions map[string]string   `mapstructure:"conditions"`
	Hooks      rawHooks            `mapstructure:"hooks"`
	TargetDir  string              `mapstructure:"target-dir"`
	Delimiters []string            `mapstructure:"delimiters"`
	Loops      map[string][]string `mapstructure:"loops"`
}

type rawHooks struct {
	PostGenerate []string `mapstructure:"post-generate"`
}

// LoadManifest reads and parses template.toml from fsys.
// Variable names are normalised to PascalCase via NormalizeKey.
func LoadManifest(fsys fs.FS) (*Manifest, error) {
	data, err := fs.ReadFile(fsys, "template.toml")
	if err != nil {
		return nil, fmt.Errorf("read template.toml: %w", err)
	}

	var intermediate map[string]any

	if err := toml.Unmarshal(data, &intermediate); err != nil {
		return nil, fmt.Errorf("parse template.toml: %w", err)
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
		return nil, fmt.Errorf("decode template.toml: %w", err)
	}

	m := &Manifest{
		Conditions: make(map[string]string, len(raw.Conditions)),
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

	if err := validateLoops(m.Loops, m.Variables, m.Delimiters); err != nil {
		return nil, err
	}

	return m, nil
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

	case []any:
		choices := make([]string, len(tv))

		for i, c := range tv {
			s, ok := c.(string)
			if !ok {
				return Variable{}, fmt.Errorf("variable %q: choice values must be strings, got %T", rawName, c)
			}

			choices[i] = s
		}

		v.Kind = KindStringChoice
		v.Value = choices

	case map[string]any:
		parsed, err := parseSubTableVariable(rawName, tv)
		if err != nil {
			return Variable{}, err
		}
		v = parsed

	default:
		format := "variable %q: unsupported type %T (use string, bool, []string, or sub-table with kind)"
		return Variable{}, fmt.Errorf(format, rawName, val)
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
	v := Variable{Name: NormalizeKey(rawName)}

	if kindStr, hasKind := tv["kind"].(string); hasKind {
		// New explicit-kind syntax: kind + value + required.
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
		default:
			return Variable{}, fmt.Errorf("variable %q: unknown kind %q", rawName, kindStr)
		}
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
