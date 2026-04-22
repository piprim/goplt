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

func TestLoadManifest_missingFile(t *testing.T) {
	_, err := goplt.LoadManifest(fstest.MapFS{})
	require.Error(t, err)
}

func TestLoadManifest_malformedTOML(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`this is not valid toml ===`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
}

func TestLoadManifest_unsupportedVariableType(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables]
count = 42
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count")
}

func TestLoadManifest_conditionsParsed(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[conditions]
"internal/connect" = "{{if .WithConnect}}true{{end}}"
"contracts/proto"  = "{{if and .WithConnect .WithContract}}true{{end}}"
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Equal(t, "{{if .WithConnect}}true{{end}}", m.Conditions["internal/connect"])
	assert.Equal(t, "{{if and .WithConnect .WithContract}}true{{end}}", m.Conditions["contracts/proto"])
}

func TestLoadManifest_hooksPreserveOrder(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[hooks]
post-generate = ["go mod tidy", "git init", "git add ."]
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Hooks.PostGenHooks, 3)
	assert.Equal(t, "go mod tidy", m.Hooks.PostGenHooks[0])
	assert.Equal(t, "git init", m.Hooks.PostGenHooks[1])
	assert.Equal(t, "git add .", m.Hooks.PostGenHooks[2])
}

func TestNormalizeKey_consecutiveSeparators(t *testing.T) {
	// Double separators should not produce empty word segments
	result := goplt.NormalizeKey("with--connect")
	assert.NotEmpty(t, result)
	assert.Equal(t, "WithConnect", result)
}

func TestNormalizeKey_leadingTrailingSeparator_notStripped(t *testing.T) {
	// NormalizeKey does not strip leading/trailing separators — document actual behaviour.
	assert.Equal(t, "-name-", goplt.NormalizeKey("-name-"))
}
