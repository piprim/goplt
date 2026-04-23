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
	assert.Equal(t, "", name.Value)
	assert.True(t, name.Required)
	assert.Equal(t, "Go module name, e.g. my-service", name.Description)

	docker := byName["WithDocker"]
	assert.Equal(t, goplt.KindBool, docker.Kind)
	assert.Equal(t, true, docker.Value)
	assert.Equal(t, "Generate a Dockerfile", docker.Description)

	license := byName["License"]
	assert.Equal(t, goplt.KindStringChoice, license.Kind)
	assert.Equal(t, []string{"MIT", "Apache-2.0"}, license.Value)
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

func TestLoadManifest_delimiters_parsed(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
delimiters = ["[[", "]]"]
[variables]
name = ""
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Equal(t, [2]string{"[[", "]]"}, m.Delimiters)
}

func TestLoadManifest_delimiters_absent_defaultsToStandard(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables]
name = ""
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Equal(t, [2]string{"{{", "}}"}, m.Delimiters)
}

func TestLoadManifest_delimiters_invalid_emptyString(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
delimiters = ["", "]]"]
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty")
}

func TestLoadManifest_delimiters_invalid_identical(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
delimiters = ["{{", "{{"]
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "differ")
}

func TestLoadManifest_delimiters_invalid_wrongCount(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
delimiters = ["[["]
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2")
}

func TestLoadManifest_newSyntax_input_required(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.name]
kind        = "input"
required    = true
description = "Library name"
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Variables, 1)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindInput, v.Kind)
	assert.True(t, v.Required)
	assert.Equal(t, "Library name", v.Description)
}

func TestLoadManifest_newSyntax_input_withDefault(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.org-prefix]
kind  = "input"
value = "github.com/acme"
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Variables, 1)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindInput, v.Kind)
	assert.Equal(t, "github.com/acme", v.Value)
	assert.False(t, v.Required)
}

func TestLoadManifest_newSyntax_stringChoice(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.license]
kind  = "stringChoice"
value = ["MIT", "Apache-2.0"]
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Variables, 1)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindStringChoice, v.Kind)
	assert.Equal(t, []string{"MIT", "Apache-2.0"}, v.Value)
}

func TestLoadManifest_newSyntax_bool(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.with-connect]
kind  = "bool"
value = true
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindBool, v.Kind)
	assert.Equal(t, true, v.Value)
}

func TestLoadManifest_newSyntax_stringList(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.packages]
kind     = "stringList"
required = true
description = "Internal packages"
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Variables, 1)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindStringList, v.Kind)
	assert.True(t, v.Required)
	assert.Equal(t, "Internal packages", v.Description)
}

func TestLoadManifest_newSyntax_stringList_withSuggestions(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.packages]
kind  = "stringList"
value = ["core", "errors"]
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindStringList, v.Kind)
	assert.Equal(t, []string{"core", "errors"}, v.Value)
}

func TestLoadManifest_newSyntax_unknownKind_error(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.x]
kind = "banana"
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "banana")
}

func TestLoadManifest_backwardCompat_defaultEmpty_becomesRequired(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.name]
default     = ""
description = "Library name"
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	require.Len(t, m.Variables, 1)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindInput, v.Kind)
	assert.True(t, v.Required)
	assert.Equal(t, "Library name", v.Description)
}

func TestLoadManifest_backwardCompat_defaultNonEmpty(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.org-prefix]
default = "github.com/acme"
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	v := m.Variables[0]
	assert.Equal(t, goplt.KindInput, v.Kind)
	assert.Equal(t, "github.com/acme", v.Value)
	assert.False(t, v.Required)
}

func TestLoadManifest_loops_parsed(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.packages]
kind     = "stringList"
required = true

[loops]
"internal/{{.item}}/" = ["Packages"]
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Equal(t, []string{"Packages"}, m.Loops["internal/{{.item}}/"])
}

func TestLoadManifest_loops_multipleEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.packages]
kind = "stringList"

[variables.commands]
kind = "stringList"

[loops]
"internal/{{.item}}/" = ["Packages"]
"cmd/{{.item}}/"      = ["Commands"]
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Equal(t, []string{"Packages"}, m.Loops["internal/{{.item}}/"])
	assert.Equal(t, []string{"Commands"}, m.Loops["cmd/{{.item}}/"])
}

func TestLoadManifest_loops_absent_nilMap(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`name = ""`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.NotNil(t, m.Loops)
	assert.Len(t, m.Loops, 0)
}

func TestLoadManifest_loops_nestedNotSupported(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.packages]
kind = "stringList"

[variables.subpkgs]
kind = "stringList"

[loops]
"internal/{{.item}}/" = ["Packages", "Subpkgs"]
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nested loops")
}

func TestLoadManifest_loops_undeclaredVariable_error(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[loops]
"internal/{{.item}}/" = ["Packages"]
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Packages")
}

func TestLoadManifest_loops_nonListVariable_error(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.packages]
kind = "input"

[loops]
"internal/{{.item}}/" = ["Packages"]
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stringList")
}

func TestLoadManifest_loops_missingItemPlaceholder_error(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.packages]
kind = "stringList"

[loops]
"internal/pkg/" = ["Packages"]
`)},
	}
	_, err := goplt.LoadManifest(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{{.item}}")
}

func TestLoadManifest_loops_varNameNormalized(t *testing.T) {
	// Verify that loop varName in TOML (kebab-case) is normalized to PascalCase.
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables.my-packages]
kind = "stringList"

[loops]
"internal/{{.item}}/" = ["my-packages"]
`)},
	}
	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	// The varName stored in m.Loops must be PascalCase, matching Variable.Name.
	assert.Equal(t, []string{"MyPackages"}, m.Loops["internal/{{.item}}/"])
}
