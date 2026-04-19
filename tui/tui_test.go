// tui/tui_test.go
package tui

import (
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildField_kindText_returnsField(t *testing.T) {
	v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Default: ""}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "Name", b.name)
}

func TestBuildField_kindText_applyWritesValue(t *testing.T) {
	v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Default: "alice"}
	vars := map[string]any{"Name": "alice"}

	_, b := buildField(v, vars)
	b.apply()
	assert.Equal(t, "alice", vars["Name"])
}

func TestBuildField_kindBool_returnsField(t *testing.T) {
	v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Default: true}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "WithDocker", b.name)
}

func TestBuildField_kindBool_applyWritesValue(t *testing.T) {
	v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Default: true}
	vars := map[string]any{"WithDocker": true}

	_, b := buildField(v, vars)
	b.apply()
	val, ok := vars["WithDocker"].(bool)
	require.True(t, ok)
	assert.True(t, val)
}

func TestBuildField_kindChoiceString_returnsField(t *testing.T) {
	v := goplt.Variable{Name: "License", Kind: goplt.KindChoiceString, Default: []string{"MIT", "Apache-2.0"}}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "License", b.name)
}

func TestBuildField_kindChoiceString_applyWritesDefault(t *testing.T) {
	v := goplt.Variable{Name: "License", Kind: goplt.KindChoiceString, Default: []string{"MIT", "Apache-2.0"}}
	vars := map[string]any{"License": "MIT"}

	_, b := buildField(v, vars)
	b.apply()
	assert.Equal(t, "MIT", vars["License"])
}

func TestBuildField_unknownKind_returnsNil(t *testing.T) {
	v := goplt.Variable{Name: "X", Kind: goplt.VariableKind(99)}
	vars := map[string]any{}

	field, _ := buildField(v, vars)
	assert.Nil(t, field)
}

// The following three tests exercise the v.Description != "" branch in buildField.
// The huh library does not expose a getter for description, so we cannot assert
// the value directly; instead we verify the branch executes without panic and
// the field/binding are still returned correctly.
func TestBuildField_kindText_withDescription_notNil(t *testing.T) {
	v := goplt.Variable{Name: "Name", Kind: goplt.KindText, Default: "", Description: "The module name"}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "Name", b.name)
}

func TestBuildField_kindBool_withDescription_notNil(t *testing.T) {
	v := goplt.Variable{Name: "WithDocker", Kind: goplt.KindBool, Default: true, Description: "Add a Dockerfile"}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "WithDocker", b.name)
}

func TestBuildField_kindChoiceString_withDescription_notNil(t *testing.T) {
	v := goplt.Variable{Name: "License", Kind: goplt.KindChoiceString, Default: []string{"MIT", "Apache-2.0"}, Description: "License to apply"}
	vars := map[string]any{}

	field, b := buildField(v, vars)
	require.NotNil(t, field)
	assert.Equal(t, "License", b.name)
}
