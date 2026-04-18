// generator_test.go
package goplt_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalTemplateFS builds a minimal in-memory template FS with a template.toml.
func minimalTemplateFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables]
name = ""
`)},
	}

	for path, content := range files {
		fsys[path] = &fstest.MapFile{Data: []byte(content)}
	}

	return fsys
}

func TestGenerate_rendersFile(t *testing.T) {
	fsys := minimalTemplateFS(map[string]string{
		"hello.txt": "Hello, {{.Name}}!",
	})

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)

	out := t.TempDir()
	err = goplt.Generate(fsys, m, out, map[string]any{"Name": "world"})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(out, "hello.txt"))
	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", string(content))
}

func TestGenerate_rendersPathWithVar(t *testing.T) {
	fsys := minimalTemplateFS(map[string]string{
		"modules/{{.Name}}/main.go": "package main",
	})

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)

	out := t.TempDir()
	err = goplt.Generate(fsys, m, out, map[string]any{"Name": "payment"})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(out, "modules/payment/main.go"))
	assert.NoError(t, err)
}

func TestGenerate_stripsTmplExtension(t *testing.T) {
	fsys := minimalTemplateFS(map[string]string{
		"go.mod.tmpl": "module example.com/{{.Name}}",
	})

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)

	out := t.TempDir()
	err = goplt.Generate(fsys, m, out, map[string]any{"Name": "myapp"})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(out, "go.mod"))
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(out, "go.mod.tmpl"))
	assert.True(t, os.IsNotExist(err), "original .tmpl file must not be written to output")
}

func TestGenerate_skipsTemplateToml(t *testing.T) {
	fsys := minimalTemplateFS(map[string]string{
		"hello.txt": "hi",
	})

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)

	out := t.TempDir()
	err = goplt.Generate(fsys, m, out, map[string]any{"Name": "x"})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(out, "template.toml"))
	assert.True(t, os.IsNotExist(err), "template.toml must not be copied to output")
}

func TestGenerate_skipsGoMod(t *testing.T) {
	fsys := minimalTemplateFS(map[string]string{
		"go.mod":  "module example.com/should-not-appear",
		"main.go": "package main",
	})

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)

	out := t.TempDir()
	err = goplt.Generate(fsys, m, out, map[string]any{"Name": "x"})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(out, "go.mod"))
	assert.True(t, os.IsNotExist(err), "go.mod must not be copied to output")

	_, err = os.Stat(filepath.Join(out, "main.go"))
	assert.NoError(t, err, "main.go must be written")
}

func TestGenerate_skipsGoSum(t *testing.T) {
	fsys := minimalTemplateFS(map[string]string{
		"go.sum":  "github.com/example/mod v1.0.0 h1:abc=\n",
		"main.go": "package main",
	})

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)

	out := t.TempDir()
	err = goplt.Generate(fsys, m, out, map[string]any{"Name": "x"})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(out, "go.sum"))
	assert.True(t, os.IsNotExist(err), "go.sum must not be copied to output")

	_, err = os.Stat(filepath.Join(out, "main.go"))
	assert.NoError(t, err, "main.go must be written")
}

func TestGenerate_conditionSkipsDir(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
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
	err = goplt.Generate(fsys, m, out, map[string]any{"Name": "x", "WithConnect": false})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(out, "adapters/connect/handler.go"))
	assert.True(t, os.IsNotExist(err), "conditioned-out file must not be written")

	_, err = os.Stat(filepath.Join(out, "main.go"))
	assert.NoError(t, err, "unconditional file must be written")
}
