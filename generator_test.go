// generator_test.go
package goplt_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"text/template"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalTemplateFS builds a minimal in-memory template FS with a template.toml.
func minimalTemplateFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
description = "test template"

[variables]
name = ""
`)},
	}

	for path, content := range files {
		fsys[path] = &fstest.MapFile{Data: []byte(content)}
	}

	return fsys
}

// loopTemplateFS builds an in-memory FS for loop tests.
func loopTemplateFS(loopVarDecl, loopsSection string, files map[string]string) fstest.MapFS {
	toml := "description = \"test template\"\n\n[variables.name]\nkind = \"input\"\nrequired = true\n" + loopVarDecl + loopsSection
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(toml)},
	}
	for path, content := range files {
		fsys[path] = &fstest.MapFile{Data: []byte(content)}
	}

	return fsys
}

func TestGenerate(t *testing.T) {
	t.Run("renders_file", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"hello.txt": "Hello, {{.Name}}!",
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "world"}))

		content, err := os.ReadFile(filepath.Join(out, "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "Hello, world!", string(content))
	})

	t.Run("renders_path_with_var", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"modules/{{.Name}}/main.go": "package main",
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "payment"}))

		_, err = os.Stat(filepath.Join(out, "modules/payment/main.go"))
		assert.NoError(t, err)
	})

	t.Run("strips_tmpl_extension", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"go.mod.tmpl": "module example.com/{{.Name}}",
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "myapp"}))

		_, err = os.Stat(filepath.Join(out, "go.mod"))
		assert.NoError(t, err)

		_, err = os.Stat(filepath.Join(out, "go.mod.tmpl"))
		assert.True(t, os.IsNotExist(err), "original .tmpl file must not be written to output")
	})

	t.Run("skips_template_toml", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"hello.txt": "hi",
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "x"}))

		_, err = os.Stat(filepath.Join(out, "template.toml"))
		assert.True(t, os.IsNotExist(err), "template.toml must not be copied to output")
	})

	t.Run("skips_go_mod", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"go.mod":  "module example.com/should-not-appear",
			"main.go": "package main",
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "x"}))

		_, err = os.Stat(filepath.Join(out, "go.mod"))
		assert.True(t, os.IsNotExist(err), "go.mod must not be copied to output")

		_, err = os.Stat(filepath.Join(out, "main.go"))
		assert.NoError(t, err, "main.go must be written")
	})

	t.Run("skips_go_sum", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"go.sum":  "github.com/example/mod v1.0.0 h1:abc=\n",
			"main.go": "package main",
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "x"}))

		_, err = os.Stat(filepath.Join(out, "go.sum"))
		assert.True(t, os.IsNotExist(err), "go.sum must not be copied to output")

		_, err = os.Stat(filepath.Join(out, "main.go"))
		assert.NoError(t, err, "main.go must be written")
	})

	t.Run("condition_skips_dir", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables]
name         = ""
with-connect = false

[conditions]
"adapters/connect" = "{{if .WithConnect}}true{{end}}"
`)},
			"adapters/connect/handler.go": &fstest.MapFile{Data: []byte("package connect")},
			"main.go":                     &fstest.MapFile{Data: []byte("package main")},
		}
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "x", "WithConnect": false}))

		_, err = os.Stat(filepath.Join(out, "adapters/connect/handler.go"))
		assert.True(t, os.IsNotExist(err), "conditioned-out file must not be written")

		_, err = os.Stat(filepath.Join(out, "main.go"))
		assert.NoError(t, err, "unconditional file must be written")
	})

	t.Run("builtin_funcs_available", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"hello.txt": `{{.Name | snake}}`,
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "MyApp"}))

		content, err := os.ReadFile(filepath.Join(out, "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "my_app", string(content))
	})

	t.Run("custom_delimiters/substitution", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
delimiters = ["[[", "]]"]

[variables]
name = ""
`)},
			"hello.txt": &fstest.MapFile{Data: []byte("Hello, [[.Name]]!")},
		}
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "world"}))

		content, err := os.ReadFile(filepath.Join(out, "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "Hello, world!", string(content))
	})

	t.Run("custom_delimiters/brace_passthrough", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
delimiters = ["[[", "]]"]

[variables]
name = ""
`)},
			"template.go.tmpl": &fstest.MapFile{Data: []byte("package {{snake .Name}}")},
		}
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "mylib"}))

		content, err := os.ReadFile(filepath.Join(out, "template.go"))
		require.NoError(t, err)
		assert.Equal(t, "package {{snake .Name}}", string(content))
	})

	t.Run("custom_delimiters/condition_respected", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"
delimiters = ["[[", "]]"]

[variables]
with-feature = false

[conditions]
"feature" = "[[if .WithFeature]]true[[end]]"
`)},
			"feature/impl.go": &fstest.MapFile{Data: []byte("package feature")},
			"main.go":         &fstest.MapFile{Data: []byte("package main")},
		}
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"WithFeature": false}))

		_, err = os.Stat(filepath.Join(out, "feature/impl.go"))
		assert.True(t, os.IsNotExist(err), "conditioned-out dir must not exist when WithFeature=false")

		_, err = os.Stat(filepath.Join(out, "main.go"))
		assert.NoError(t, err, "unconditional file must exist")
	})
}

func TestGenerator_WithFuncs(t *testing.T) {
	t.Run("adds_custom_func", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"hello.txt": `{{.Name | shout}}`,
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		err = goplt.NewGenerator().
			WithFuncs(template.FuncMap{
				"shout": func(s string) string { return s + "!" },
			}).
			Generate(fsys, m, out, map[string]any{"Name": "hello"})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(out, "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello!", string(content))
	})

	t.Run("overrides_builtin", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"hello.txt": `{{.Name | upper}}`,
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		err = goplt.NewGenerator().
			WithFuncs(template.FuncMap{
				"upper": func(s string) string { return "OVERRIDDEN" },
			}).
			Generate(fsys, m, out, map[string]any{"Name": "hello"})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(out, "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "OVERRIDDEN", string(content))
	})
}

func TestGenerate_Loop(t *testing.T) {
	t.Run("produces_one_subtree_per_item", func(t *testing.T) {
		fsys := loopTemplateFS(
			"[variables.packages]\nkind = \"stringList\"\nrequired = true\n",
			"[loops]\n\"internal/{{.item}}/\" = [\"Packages\"]\n",
			map[string]string{
				"internal/{{.item}}/doc.go": "package {{.item}}",
			},
		)
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{
			"Name":     "mylib",
			"Packages": []string{"auth", "hash"},
		}))

		for _, pkg := range []string{"auth", "hash"} {
			content, err := os.ReadFile(filepath.Join(out, "internal", pkg, "doc.go"))
			require.NoError(t, err, "expected internal/%s/doc.go", pkg)
			assert.Equal(t, "package "+pkg, string(content))
		}
	})

	t.Run("item_and_vars_available_in_content", func(t *testing.T) {
		fsys := loopTemplateFS(
			"[variables.packages]\nkind = \"stringList\"\nrequired = true\n",
			"[loops]\n\"internal/{{.item}}/\" = [\"Packages\"]\n",
			map[string]string{
				"internal/{{.item}}/{{.item}}.go": "// part of {{.Name}}\npackage {{.item}}",
			},
		)
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{
			"Name":     "mylib",
			"Packages": []string{"auth"},
		}))

		content, err := os.ReadFile(filepath.Join(out, "internal", "auth", "auth.go"))
		require.NoError(t, err)
		assert.Equal(t, "// part of mylib\npackage auth", string(content))
	})

	t.Run("no_items_writes_nothing", func(t *testing.T) {
		fsys := loopTemplateFS(
			"[variables.packages]\nkind = \"stringList\"\n",
			"[loops]\n\"internal/{{.item}}/\" = [\"Packages\"]\n",
			map[string]string{
				"internal/{{.item}}/doc.go": "package {{.item}}",
			},
		)
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{
			"Name":     "mylib",
			"Packages": []string{},
		}))

		_, err = os.Stat(filepath.Join(out, "internal"))
		assert.True(t, os.IsNotExist(err), "internal/ must not exist with no packages")
	})

	t.Run("condition_gates_entire_loop", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.with-internal]
kind  = "bool"
value = false

[variables.packages]
kind = "stringList"

[conditions]
"internal/" = "{{if .WithInternal}}true{{end}}"

[loops]
"internal/{{.item}}/" = ["Packages"]
`)},
			"internal/{{.item}}/doc.go": &fstest.MapFile{Data: []byte("package {{.item}}")},
		}
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{
			"WithInternal": false,
			"Packages":     []string{"auth", "hash"},
		}))

		_, err = os.Stat(filepath.Join(out, "internal"))
		assert.True(t, os.IsNotExist(err), "internal/ must not exist when WithInternal=false")
	})

	t.Run("per_item_condition", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.toml": &fstest.MapFile{Data: []byte(`
description = "test"

[variables.packages]
kind = "stringList"

[loops]
"internal/{{.item}}/" = ["Packages"]

[conditions]
"internal/{{.item}}/" = "{{if ne .item \"skip\"}}true{{end}}"
`)},
			"internal/{{.item}}/doc.go": &fstest.MapFile{Data: []byte("package {{.item}}")},
		}
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{
			"Packages": []string{"auth", "skip", "hash"},
		}))

		for _, pkg := range []string{"auth", "hash"} {
			_, err = os.Stat(filepath.Join(out, "internal", pkg, "doc.go"))
			assert.NoError(t, err, "expected internal/%s/doc.go", pkg)
		}

		_, err = os.Stat(filepath.Join(out, "internal", "skip", "doc.go"))
		assert.True(t, os.IsNotExist(err), "internal/skip/doc.go must not exist")
	})

	t.Run("no_loops_existing_behaviour_unchanged", func(t *testing.T) {
		fsys := minimalTemplateFS(map[string]string{
			"hello.txt": "Hello, {{.Name}}!",
		})
		m, err := goplt.LoadManifest(fsys)
		require.NoError(t, err)

		out := t.TempDir()
		require.NoError(t, goplt.Generate(fsys, m, out, map[string]any{"Name": "world"}))

		content, err := os.ReadFile(filepath.Join(out, "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "Hello, world!", string(content))
	})
}
