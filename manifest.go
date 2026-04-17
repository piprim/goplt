package goplt

import (
	"strings"
	"unicode"
	"unicode/utf8"
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
	Name    string       // PascalCase
	Default any          // string | bool | []string
	Kind    VariableKind
}

// PostGenHooks is a list of post-generation shell commands.
type PostGenHooks []string

// Hooks holds the hook commands declared under [hooks] in template.toml.
type Hooks struct {
	PostGenHooks PostGenHooks `mapstructure:"post_generate"`
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
