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
		{"with--connect", "WithConnect"},         // consecutive separators
		{"-name-", "-name-"},                     // leading/trailing separator: document actual behaviour
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, goplt.NormalizeKey(tc.input))
		})
	}
}

func TestLoadManifest_Description(t *testing.T) {
	t.Run("parsed_correctly", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "A fine library"
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.Equal(t, "A fine library", m.Description)
	})

	t.Run("missing_returns_error", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
[variables]
name = ""
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "description")
	})
}

func TestLoadManifest_Errors(t *testing.T) {
	t.Run("missing_file", func(t *testing.T) {
		_, err := goplt.LoadManifest(fstest.MapFS{})
		require.Error(t, err)
	})

	t.Run("malformed_toml", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`this is not valid toml ===`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
	})

	t.Run("unsupported_variable_type", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables]
count = 42
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "count")
	})
}

func TestLoadManifest_TargetDir(t *testing.T) {
	t.Run("parsed", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
target-dir = "{{.Name}}"

[variables]
name = ""
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.Equal(t, "{{.Name}}", m.TargetDir)
	})

	t.Run("absent", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables]
name = ""
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.Empty(t, m.TargetDir)
	})
}

func TestLoadManifest_Variables(t *testing.T) {
	t.Run("with_description", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("subtable_without_description", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.name]
default = ""
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		require.Len(t, m.Variables, 1)
		assert.Equal(t, "", m.Variables[0].Description)
	})

	t.Run("subtable_missing_default_or_kind_errors", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.name]
description = "The module name"
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "default")
	})
}

func TestLoadManifest_Conditions(t *testing.T) {
	t.Run("parsed", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[conditions]
"internal/connect" = "{{if .WithConnect}}true{{end}}"
"contracts/proto"  = "{{if and .WithConnect .WithContract}}true{{end}}"
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.Equal(t, "{{if .WithConnect}}true{{end}}", m.Conditions["internal/connect"])
		assert.Equal(t, "{{if and .WithConnect .WithContract}}true{{end}}", m.Conditions["contracts/proto"])
	})
}

func TestLoadManifest_Hooks(t *testing.T) {
	t.Run("preserve_order", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})
}

func TestLoadManifest_Delimiters(t *testing.T) {
	t.Run("parsed", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
delimiters = ["[[", "]]"]

[variables]
name = ""
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.Equal(t, [2]string{"[[", "]]"}, m.Delimiters)
	})

	t.Run("absent_defaults_to_standard", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables]
name = ""
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.Equal(t, [2]string{"{{", "}}"}, m.Delimiters)
	})

	t.Run("invalid_empty_string", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
delimiters = ["", "]]"]
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty")
	})

	t.Run("invalid_identical", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
delimiters = ["{{", "{{"]
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "differ")
	})

	t.Run("invalid_wrong_count", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
delimiters = ["[["]
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "2")
	})
}

func TestLoadManifest_NewSyntax(t *testing.T) {
	t.Run("input_required", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("input_with_default", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("string_choice", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("bool", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("string_list", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.packages]
kind        = "stringList"
required    = true
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
	})

	t.Run("string_list_with_suggestions", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("unknown_kind_error", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.x]
kind = "banana"
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "banana")
	})
}

func TestLoadManifest_BackwardCompat(t *testing.T) {
	t.Run("empty_default_becomes_required", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("non_empty_default", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})
}

func TestLoadManifest_Loops(t *testing.T) {
	t.Run("parsed", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("multiple_entries", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("absent_produces_empty_map", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
name = ""
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.NotNil(t, m.Loops)
		assert.Len(t, m.Loops, 0)
	})

	t.Run("nested_not_supported", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

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
	})

	t.Run("undeclared_variable_error", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[loops]
"internal/{{.item}}/" = ["Packages"]
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Packages")
	})

	t.Run("non_list_variable_error", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.packages]
kind = "input"

[loops]
"internal/{{.item}}/" = ["Packages"]
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stringList")
	})

	t.Run("missing_item_placeholder_error", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.packages]
kind = "stringList"

[loops]
"internal/pkg/" = ["Packages"]
`)},
		}

		_, err := goplt.LoadManifest(fsys)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "{{.item}}")
	})

	t.Run("var_name_normalized_to_pascal_case", func(t *testing.T) {
		// Verify that a kebab-case varName in TOML is normalized to PascalCase.
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.my-packages]
kind = "stringList"

[loops]
"internal/{{.item}}/" = ["my-packages"]
`)},
		}

		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)
		assert.Equal(t, []string{"MyPackages"}, m.Loops["internal/{{.item}}/"])
	})
}
