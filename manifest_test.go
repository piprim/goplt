package goplt_test

import (
	"testing"
	"testing/fstest"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"with-connect", "WithConnect"},
		{"with_connect", "WithConnect"},
		{"withConnect", "WithConnect"},
		{"name", "Name"},
		{"org-prefix", "OrgPrefix"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, goplt.NormalizeKey(tc.input))
		})
	}
}

func TestLoadManifest_targetDir_parsed(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
target-dir = "{{.Name}}"

[variables]
name = ""
`)},
	}

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Equal(t, "{{.Name}}", m.TargetDir)
}

func TestLoadManifest_targetDir_absent(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables]
name = ""
`)},
	}

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Empty(t, m.TargetDir)
}

func TestLoadManifest_variableWithDescription(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.name]
default     = ""
description = "Go module name, e.g. my-service"

[variables.with-docker]
default     = true
description = "Generate a Dockerfile"

[variables.license]
default     = ["MIT", "Apache-2.0"]
description = "License to apply"
`)},
	}

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Variables, 3)

	byName := make(map[string]goplt.Variable, len(m.Variables))
	for _, v := range m.Variables {
		byName[v.Name] = v
	}

	name := byName["Name"]
	assert.Equal(t, goplt.KindText, name.Kind)
	assert.Equal(t, "", name.Default)
	assert.Equal(t, "Go module name, e.g. my-service", name.Description)

	docker := byName["WithDocker"]
	assert.Equal(t, goplt.KindBool, docker.Kind)
	assert.Equal(t, true, docker.Default)
	assert.Equal(t, "Generate a Dockerfile", docker.Description)

	license := byName["License"]
	assert.Equal(t, goplt.KindChoiceString, license.Kind)
	assert.Equal(t, []string{"MIT", "Apache-2.0"}, license.Default)
	assert.Equal(t, "License to apply", license.Description)
}

func TestLoadManifest_variableSubtableWithoutDescription(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.name]
default = ""
`)},
	}

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Variables, 1)
	assert.Equal(t, "", m.Variables[0].Description)
}

func TestLoadManifest_variableSubtableMissingDefault(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.name]
description = "The module name"
`)},
	}

	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default")
}
