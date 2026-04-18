package goplt

import (
	"fmt"
	"io/fs"
	"log"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pelletier/go-toml/v2"
)

// VariableKind represents the type of a template variable.
type VariableKind int

const (
	KindText         VariableKind = iota // string input; empty default = required
	KindBool                             // yes/no confirm
	KindChoiceString                     // select from list; first item = default
)

// Variable describes a single template variable from template.toml.
type Variable struct {
	Name    string // PascalCase
	Default any    // string | bool | []string
	Kind    VariableKind
}

// PostGenHooks is a list of post-generation shell commands.
type PostGenHooks []string

// Hooks holds the hook commands declared under [hooks] in template.toml.
type Hooks struct {
	PostGenHooks PostGenHooks `mapstructure:"post-generate"`
}

// Manifest holds the parsed content of a template.toml file.
type Manifest struct {
	Variables  []Variable
	Conditions map[string]string // unrendered path prefix → Go template boolean expression
	Hooks      Hooks
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

	format := `Key "%s" normalized to "%s"` + "\n"

	if len(parts) <= 1 {
		r, size := utf8.DecodeRuneInString(s)
		out := string(unicode.ToUpper(r)) + s[size:]

		if out != s {
			log.Printf(format, s, out)

			return out
		}

		log.Printf(`Key "%s" already normalized`+"\n", s)

		return out
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

	out := b.String()
	log.Printf(format, s, out)

	return out
}

// rawManifest is the intermediate representation decoded from template.toml.
type rawManifest struct {
	Variables  map[string]any    `mapstructure:"variables"`
	Conditions map[string]string `mapstructure:"conditions"`
	Hooks      rawHooks          `mapstructure:"hooks"`
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
	}

	maps.Copy(m.Conditions, raw.Conditions)

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

	return m, nil
}

func parseVariable(rawName string, val any) (Variable, error) {
	v := Variable{Name: NormalizeKey(rawName)}

	switch tv := val.(type) {
	case string:
		v.Kind = KindText
		v.Default = tv

	case bool:
		v.Kind = KindBool
		v.Default = tv

	case []any:
		choices := make([]string, len(tv))

		for i, c := range tv {
			s, ok := c.(string)
			if !ok {
				return Variable{}, fmt.Errorf("variable %q: choice values must be strings, got %T", rawName, c)
			}

			choices[i] = s
		}

		v.Kind = KindChoiceString
		v.Default = choices

	default:
		return Variable{}, fmt.Errorf("variable %q: unsupported type %T (use string, bool, or []string)", rawName, val)
	}

	return v, nil
}
