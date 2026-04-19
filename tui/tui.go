// Package tui provides an interactive TUI form for collecting goplt template variables.
package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/piprim/goplt"
)

// CollectVars runs an interactive TUI form for all variables declared in m.
// It returns a PascalCase-keyed map of the collected values, ready to pass
// to goplt.Generate.
func CollectVars(m *goplt.Manifest) (map[string]any, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest is nil")
	}

	vars := make(map[string]any, len(m.Variables))
	for _, v := range m.Variables {
		vars[v.Name] = v.Default
	}

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

	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return nil, fmt.Errorf("tui form: %w", err)
	}

	for _, b := range bindings {
		b.apply()
	}

	return vars, nil
}

// binding pairs a variable name with a function that writes the collected value into vars.
type binding struct {
	name  string
	apply func()
}

// buildField constructs a huh.Field and its binding for a single manifest variable.
func buildField(v goplt.Variable, vars map[string]any) (huh.Field, binding) {
	name := v.Name

	switch v.Kind {
	case goplt.KindText:
		val := ""
		if s, ok := v.Default.(string); ok {
			val = s
		}

		ptr := &val
		field := huh.NewInput().
			Title(name).
			Value(ptr).
			Validate(func(s string) error {
				if def, _ := v.Default.(string); def == "" && s == "" {
					return fmt.Errorf("%s is required", name)
				}
				return nil
			})

		return field, binding{name: name, apply: func() { vars[name] = *ptr }}

	case goplt.KindBool:
		val := false
		if b, ok := v.Default.(bool); ok {
			val = b
		}

		ptr := &val
		field := huh.NewConfirm().Title(name).Value(ptr)

		return field, binding{name: name, apply: func() { vars[name] = *ptr }}

	case goplt.KindChoiceString:
		choices, _ := v.Default.([]string)
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

		return field, binding{name: name, apply: func() { vars[name] = *ptr }}

	default:
		return nil, binding{}
	}
}
