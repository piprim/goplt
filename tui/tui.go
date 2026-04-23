// Package tui provides an interactive TUI form for collecting goplt template variables.
package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/piprim/goplt"
)

// initVars seeds vars from manifest defaults. Pure function, no I/O.
func initVars(m *goplt.Manifest) map[string]any {
	vars := make(map[string]any, len(m.Variables))
	for _, v := range m.Variables {
		switch v.Kind {
		case goplt.KindStringList:
			if items, ok := v.Value.([]string); ok {
				vars[v.Name] = slices.Clone(items)
			} else {
				vars[v.Name] = []string{}
			}
		default:
			vars[v.Name] = v.Value
		}
	}

	return vars
}

// CollectVars runs an interactive TUI form for all variables declared in m.
// It returns a PascalCase-keyed map of the collected values, ready to pass
// to goplt.Generate.
func CollectVars(m *goplt.Manifest) (map[string]any, error) {
	if m == nil {
		return nil, errors.New("manifest is nil")
	}

	vars := initVars(m)

	var bindings []binding
	var fields []huh.Field

	for i := range m.Variables {
		f, b := buildField(m.Variables[i], vars)
		if f != nil {
			fields = append(fields, f)
			bindings = append(bindings, b)
		}
	}

	if len(fields) == 0 {
		return vars, nil
	}

	if err := huh.NewForm(newGroup(m.Description, fields...)).Run(); err != nil {
		return nil, fmt.Errorf("tui form: %w", err)
	}

	for _, b := range bindings {
		b.apply()
	}

	return vars, nil
}

// newGroup creates a huh.Group from fields, setting the description when non-empty.
func newGroup(description string, fields ...huh.Field) *huh.Group {
	var style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#33DD33")).
		Padding(1).Align(lipgloss.Center)

	g := huh.NewGroup(fields...).WithTheme(huh.ThemeDracula())
	if description != "" {
		g = g.Description("\n" + style.Render(description))
	}

	return g
}

// binding pairs a variable name with a function that writes the collected value into vars.
type binding struct {
	name  string
	apply func()
}

// buildField constructs a huh.Field and its binding for a single manifest variable.
func buildField(v goplt.Variable, vars map[string]any) (huh.Field, binding) {
	switch v.Kind {
	case goplt.KindText:
		return buildInputField(v, vars)
	case goplt.KindBool:
		return buildBoolField(v, vars)
	case goplt.KindStringChoice:
		return buildStringChoiceField(v, vars)
	case goplt.KindStringList:
		return buildStringListField(v, vars)
	default:
		return nil, binding{}
	}
}

func buildInputField(v goplt.Variable, vars map[string]any) (huh.Field, binding) {
	name := v.Name
	val := ""
	if s, ok := v.Value.(string); ok {
		val = s
	}
	ptr := &val
	field := huh.NewInput().
		Title(name).
		Value(ptr).
		Validate(func(s string) error {
			if v.Required && s == "" {
				return fmt.Errorf("%s is required", name)
			}

			return nil
		})
	if v.Description != "" {
		field = field.Description(v.Description)
	}

	return field, binding{name: name, apply: func() { vars[name] = *ptr }}
}

func buildBoolField(v goplt.Variable, vars map[string]any) (huh.Field, binding) {
	name := v.Name
	val := false
	if b, ok := v.Value.(bool); ok {
		val = b
	}
	ptr := &val
	field := huh.NewConfirm().Title(name).Value(ptr)
	if v.Description != "" {
		field = field.Description(v.Description)
	}

	return field, binding{name: name, apply: func() { vars[name] = *ptr }}
}

func buildStringChoiceField(v goplt.Variable, vars map[string]any) (huh.Field, binding) {
	name := v.Name
	choices, _ := v.Value.([]string)
	opts := make([]huh.Option[string], len(choices))
	for j, c := range choices {
		opts[j] = huh.NewOption(c, c)
	}
	val := ""
	if len(choices) > 0 {
		val = choices[0]
	}
	ptr := &val
	field := huh.NewSelect[string]().Title(name).Options(opts...).Value(ptr)
	if v.Description != "" {
		field = field.Description(v.Description)
	}

	return field, binding{name: name, apply: func() { vars[name] = *ptr }}
}

func buildStringListField(v goplt.Variable, vars map[string]any) (huh.Field, binding) {
	name := v.Name
	suggestions, _ := v.Value.([]string)
	initial := strings.Join(suggestions, ", ")
	ptr := &initial
	field := huh.NewInput().
		Title(name).
		Value(ptr).
		Validate(func(s string) error {
			if v.Required && len(parseListInput(s)) == 0 {
				return fmt.Errorf("%s is required", name)
			}

			return nil
		})
	if v.Description != "" {
		field = field.Description(v.Description)
	}

	return field, binding{name: name, apply: func() { vars[name] = parseListInput(*ptr) }}
}

// parseListInput splits a comma-separated string into a trimmed, non-empty slice.
func parseListInput(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}
