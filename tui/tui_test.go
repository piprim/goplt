// tui/tui_test.go
package tui

import (
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildField(t *testing.T) {
	t.Run("kind_text/returns_field", func(t *testing.T) {
		v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Required: true}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "Name", b.name)
	})

	t.Run("kind_text/apply_writes_value", func(t *testing.T) {
		v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Value: "alice"}
		vars := map[string]any{"Name": "alice"}

		_, b := buildField(&v, vars)
		b.apply()
		assert.Equal(t, "alice", vars["Name"])
	})

	t.Run("kind_text/with_description", func(t *testing.T) {
		v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Required: true, Description: "The module name"}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "Name", b.name)
	})

	t.Run("kind_bool/returns_field", func(t *testing.T) {
		v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Value: true}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "WithDocker", b.name)
	})

	t.Run("kind_bool/apply_writes_value", func(t *testing.T) {
		v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Value: true}
		vars := map[string]any{"WithDocker": true}

		_, b := buildField(&v, vars)
		b.apply()
		val, ok := vars["WithDocker"].(bool)
		require.True(t, ok)
		assert.True(t, val)
	})

	t.Run("kind_bool/with_description", func(t *testing.T) {
		v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Value: true, Description: "Add a Dockerfile"}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "WithDocker", b.name)
	})

	t.Run("kind_string_choice/returns_field", func(t *testing.T) {
		v := goplt.Variable{Name: "License", Kind: goplt.KindStringChoice, Value: []string{"MIT", "Apache-2.0"}}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "License", b.name)
	})

	t.Run("kind_string_choice/apply_writes_default", func(t *testing.T) {
		v := goplt.Variable{Name: "License", Kind: goplt.KindStringChoice, Value: []string{"MIT", "Apache-2.0"}}
		vars := map[string]any{"License": "MIT"}

		_, b := buildField(&v, vars)
		b.apply()
		assert.Equal(t, "MIT", vars["License"])
	})

	t.Run("kind_string_choice/with_description", func(t *testing.T) {
		v := goplt.Variable{Name: "License", Kind: goplt.KindStringChoice, Value: []string{"MIT", "Apache-2.0"}, Description: "License to apply"}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "License", b.name)
	})

	t.Run("kind_string_list/returns_field", func(t *testing.T) {
		v := goplt.Variable{Name: "Packages", Kind: goplt.KindStringList, Required: true}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "Packages", b.name)
	})

	t.Run("kind_string_list/apply_writes_slice", func(t *testing.T) {
		v := goplt.Variable{Name: "Packages", Kind: goplt.KindStringList}
		vars := map[string]any{"Packages": []string{}}

		_, b := buildField(&v, vars)
		b.apply()
		_, ok := vars["Packages"].([]string)
		require.True(t, ok)
	})

	t.Run("kind_string_list/with_suggestions_initial_value", func(t *testing.T) {
		// huh.Input does not expose a getter, so we only verify the field is non-nil.
		v := goplt.Variable{
			Name:  "Packages",
			Kind:  goplt.KindStringList,
			Value: []string{"core", "errors"},
		}
		vars := map[string]any{}

		field, _ := buildField(&v, vars)
		require.NotNil(t, field)
	})

	t.Run("unknown_kind/returns_nil", func(t *testing.T) {
		v := goplt.Variable{Name: "X", Kind: goplt.VariableKind("unknown")}
		vars := map[string]any{}

		field, _ := buildField(&v, vars)
		assert.Nil(t, field)
	})
}

func TestNewGroup(t *testing.T) {
	field := huh.NewInput().Title("Name").Value(new(string))

	t.Run("with_description/returns_non_nil", func(t *testing.T) {
		g := newGroup("A fine library", field)
		assert.NotNil(t, g)
	})

	t.Run("empty_description/returns_non_nil", func(t *testing.T) {
		g := newGroup("", field)
		assert.NotNil(t, g)
	})
}

func TestParseListInput(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		assert.Equal(t, []string{"auth", "hash", "cache"}, parseListInput("auth, hash, cache"))
	})

	t.Run("trailing_comma", func(t *testing.T) {
		assert.Equal(t, []string{"auth", "hash"}, parseListInput("auth, hash,"))
	})

	t.Run("empty_string", func(t *testing.T) {
		assert.Empty(t, parseListInput(""))
	})

	t.Run("only_spaces_and_commas", func(t *testing.T) {
		assert.Empty(t, parseListInput("  ,  "))
	})
}

func TestInitVars(t *testing.T) {
	t.Run("string_list/clones_value", func(t *testing.T) {
		original := []string{"core", "errors"}
		m := &goplt.Manifest{
			Variables: []goplt.Variable{
				{Name: "Packages", Kind: goplt.KindStringList, Value: original},
			},
		}

		vars := initVars(m)
		result, ok := vars["Packages"].([]string)
		require.True(t, ok)
		assert.Equal(t, original, result)

		// Mutating vars must not affect the original.
		result[0] = "mutated"
		assert.Equal(t, "core", original[0], "mutation must not propagate back to original")
	})

	t.Run("string_list/nil_value_returns_empty_slice", func(t *testing.T) {
		m := &goplt.Manifest{
			Variables: []goplt.Variable{
				{Name: "Packages", Kind: goplt.KindStringList},
			},
		}

		vars := initVars(m)
		result, ok := vars["Packages"].([]string)
		require.True(t, ok)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("other_kinds/uses_value_directly", func(t *testing.T) {
		m := &goplt.Manifest{
			Variables: []goplt.Variable{
				{Name: "Name", Kind: goplt.KindInput, Value: "mylib"},
				{Name: "WithDocker", Kind: goplt.KindBool, Value: true},
			},
		}

		vars := initVars(m)
		assert.Equal(t, "mylib", vars["Name"])
		assert.Equal(t, true, vars["WithDocker"])
	})
}

func TestValidateIntInput(t *testing.T) {
	t.Run("required_empty_returns_error", func(t *testing.T) {
		fn := validateIntInput("Port", true, nil, nil)
		err := fn("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})

	t.Run("not_required_empty_returns_nil", func(t *testing.T) {
		fn := validateIntInput("Port", false, nil, nil)
		assert.NoError(t, fn(""))
	})

	t.Run("non_integer_returns_error", func(t *testing.T) {
		fn := validateIntInput("Port", false, nil, nil)
		err := fn("abc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "integer")
	})

	t.Run("below_min_returns_error", func(t *testing.T) {
		minVal := 1
		fn := validateIntInput("Port", false, &minVal, nil)
		err := fn("0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "≥")
	})

	t.Run("above_max_returns_error", func(t *testing.T) {
		maxVal := 65535
		fn := validateIntInput("Port", false, nil, &maxVal)
		err := fn("65536")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "≤")
	})

	t.Run("valid_in_bounds", func(t *testing.T) {
		minVal, maxVal := 1, 65535
		fn := validateIntInput("Port", false, &minVal, &maxVal)
		assert.NoError(t, fn("8080"))
	})
}

func TestBuildField_IntKinds(t *testing.T) {
	t.Run("kind_int/returns_non_nil_field", func(t *testing.T) {
		minVal, maxVal := 1, 65535
		v := goplt.Variable{
			Name:  "Port",
			Kind:  goplt.KindInt,
			Value: 8080,
			Min:   &minVal,
			Max:   &maxVal,
		}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "Port", b.name)
	})

	t.Run("kind_int/apply_writes_int", func(t *testing.T) {
		v := goplt.Variable{Name: "Port", Kind: goplt.KindInt, Value: 8080}
		vars := map[string]any{"Port": 8080}

		_, b := buildField(&v, vars)
		b.apply()
		val, ok := vars["Port"].(int)
		require.True(t, ok)
		assert.Equal(t, 8080, val)
	})

	t.Run("kind_int/with_description", func(t *testing.T) {
		v := goplt.Variable{Name: "Port", Kind: goplt.KindInt, Value: 8080, Description: "Server port"}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "Port", b.name)
	})

	t.Run("kind_int_choice/returns_non_nil_field", func(t *testing.T) {
		v := goplt.Variable{
			Name:  "Workers",
			Kind:  goplt.KindIntChoice,
			Value: []int{1, 2, 4, 8},
		}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "Workers", b.name)
	})

	t.Run("kind_int_choice/apply_writes_first_item", func(t *testing.T) {
		v := goplt.Variable{Name: "Workers", Kind: goplt.KindIntChoice, Value: []int{1, 2, 4, 8}}
		vars := map[string]any{"Workers": 1}

		_, b := buildField(&v, vars)
		b.apply()
		val, ok := vars["Workers"].(int)
		require.True(t, ok)
		assert.Equal(t, 1, val)
	})

	t.Run("kind_int_choice/with_description", func(t *testing.T) {
		v := goplt.Variable{Name: "Workers", Kind: goplt.KindIntChoice, Value: []int{1, 2}, Description: "Worker count"}
		vars := map[string]any{}

		field, b := buildField(&v, vars)
		require.NotNil(t, field)
		assert.Equal(t, "Workers", b.name)
	})
}
