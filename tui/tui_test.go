// tui/tui_test.go
package tui

import (
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildField_kindText_returnsField(t *testing.T) {
	v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Required: true}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "Name", b.name)
}

func TestBuildField_kindText_applyWritesValue(t *testing.T) {
	v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Value: "alice"}
	vars := map[string]any{"Name": "alice"}

	_, b := buildField(v, vars)
	b.apply()
	assert.Equal(t, "alice", vars["Name"])
}

func TestBuildField_kindBool_returnsField(t *testing.T) {
	v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Value: true}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "WithDocker", b.name)
}

func TestBuildField_kindBool_applyWritesValue(t *testing.T) {
	v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Value: true}
	vars := map[string]any{"WithDocker": true}

	_, b := buildField(v, vars)
	b.apply()
	val, ok := vars["WithDocker"].(bool)
	require.True(t, ok)
	assert.True(t, val)
}

func TestBuildField_kindChoiceString_returnsField(t *testing.T) {
	v := goplt.Variable{Name: "License", Kind: goplt.KindStringChoice, Value: []string{"MIT", "Apache-2.0"}}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "License", b.name)
}

func TestBuildField_kindChoiceString_applyWritesDefault(t *testing.T) {
	v := goplt.Variable{Name: "License", Kind: goplt.KindStringChoice, Value: []string{"MIT", "Apache-2.0"}}
	vars := map[string]any{"License": "MIT"}

	_, b := buildField(v, vars)
	b.apply()
	assert.Equal(t, "MIT", vars["License"])
}

func TestBuildField_unknownKind_returnsNil(t *testing.T) {
	v := goplt.Variable{Name: "X", Kind: goplt.VariableKind("unknown")}
	vars := map[string]any{}

	field, _ := buildField(v, vars)
	assert.Nil(t, field)
}

// The following three tests exercise the v.Description != "" branch in buildField.
// The huh library does not expose a getter for description, so we cannot assert
// the value directly; instead we verify the branch executes without panic and
// the field/binding are still returned correctly.
func TestBuildField_kindText_withDescription_notNil(t *testing.T) {
	v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Required: true, Description: "The module name"}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "Name", b.name)
}

func TestBuildField_kindBool_withDescription_notNil(t *testing.T) {
	v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Value: true, Description: "Add a Dockerfile"}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "WithDocker", b.name)
}

func TestBuildField_kindChoiceString_withDescription_notNil(t *testing.T) {
	v := goplt.Variable{Name: "License", Kind: goplt.KindStringChoice, Value: []string{"MIT", "Apache-2.0"}, Description: "License to apply"}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "License", b.name)
}

func TestParseListInput_basic(t *testing.T) {
	assert.Equal(t, []string{"auth", "hash", "cache"}, parseListInput("auth, hash, cache"))
}

func TestParseListInput_trailingComma(t *testing.T) {
	assert.Equal(t, []string{"auth", "hash"}, parseListInput("auth, hash,"))
}

func TestParseListInput_empty(t *testing.T) {
	assert.Empty(t, parseListInput(""))
	assert.Empty(t, parseListInput("  ,  "))
}

func TestBuildField_kindListString_returnsField(t *testing.T) {
	v := goplt.Variable{Name: "Packages", Kind: goplt.KindStringList, Required: true}
	vars := map[string]any{}
	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "Packages", b.name)
}

func TestBuildField_kindListString_applyWritesSlice(t *testing.T) {
	v := goplt.Variable{Name: "Packages", Kind: goplt.KindStringList}
	vars := map[string]any{"Packages": []string{}}
	_, b := buildField(v, vars)
	b.apply()
	result, ok := vars["Packages"].([]string)
	require.True(t, ok)
	_ = result // apply writes whatever the input holds (empty by default in unit test)
}

func TestBuildField_kindListString_withSuggestions_initialValue(t *testing.T) {
	v := goplt.Variable{
		Name:  "Packages",
		Kind:  goplt.KindStringList,
		Value: []string{"core", "errors"},
	}
	vars := map[string]any{}
	field, _ := buildField(v, vars)
	require.NotNil(t, field)
	// Field is initialized with "core, errors" as the starting text.
	// huh.Input does not expose a getter, so we only verify non-nil.
}

func TestInitVars_kindListString_clonesValue(t *testing.T) {
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
}

func TestInitVars_kindListString_nilValue_returnsEmptySlice(t *testing.T) {
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
}

func TestInitVars_otherKinds_usesValueDirectly(t *testing.T) {
	m := &goplt.Manifest{
		Variables: []goplt.Variable{
			{Name: "Name", Kind: goplt.KindInput, Value: "mylib"},
			{Name: "WithDocker", Kind: goplt.KindBool, Value: true},
		},
	}
	vars := initVars(m)
	assert.Equal(t, "mylib", vars["Name"])
	assert.Equal(t, true, vars["WithDocker"])
}
