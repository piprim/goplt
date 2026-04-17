package goplt_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultCobraVars returns a full set of variables for the cli-cobra template.
func defaultCobraVars() map[string]any {
	return map[string]any{
		"Name":        "myapp",
		"Short":       "A sample CLI",
		"Description": "myapp is a CLI built with Cobra and Viper",
		"OrgPrefix":   "github.com/acme",
		"Author":      "Test Author",
		"Version":     "0.1.0",
		"WithConfig":  true,
		"WithDocker":  true,
		"Toolchain":   "make",
		"License":     "MIT",
	}
}

// generateCobra runs goplt.Generate with the cli-cobra template into a temp dir.
func generateCobra(t *testing.T, vars map[string]any) string {
	t.Helper()
	fsys := os.DirFS("_templates/cli-cobra")
	out := t.TempDir()
	require.NoError(t, goplt.Generate(fsys, out, vars))
	return out
}

func TestCliCobraTemplate_allOptions(t *testing.T) {
	out := generateCobra(t, defaultCobraVars())

	expected := []string{
		"go.mod",
		"Makefile",
		"README.md",
		"LICENSE",
		".gitignore",
		"cmd/main.go",
		"cmd/cmd/root.go",
		"cmd/cmd/version/version.go",
		"cmd/cmd/hello/hello.go",
		"cmd/cmd/completion/completion.go",
		"internal/config/config.go",
		"configs/config.toml",
		"Dockerfile",
		".dockerignore",
	}
	for _, f := range expected {
		_, err := os.Stat(filepath.Join(out, f))
		assert.NoError(t, err, "expected file %s to exist", f)
	}

	_, err := os.Stat(filepath.Join(out, "mise.toml"))
	assert.True(t, os.IsNotExist(err), "mise.toml must not exist when toolchain=make")
}

func TestCliCobraTemplate_toolchainMise(t *testing.T) {
	vars := defaultCobraVars()
	vars["Toolchain"] = "mise"
	out := generateCobra(t, vars)

	_, err := os.Stat(filepath.Join(out, "mise.toml"))
	assert.NoError(t, err, "mise.toml must exist when toolchain=mise")

	_, err = os.Stat(filepath.Join(out, "Makefile"))
	assert.True(t, os.IsNotExist(err), "Makefile must not exist when toolchain=mise")
}

func TestCliCobraTemplate_withoutConfig(t *testing.T) {
	vars := defaultCobraVars()
	vars["WithConfig"] = false
	out := generateCobra(t, vars)

	for _, absent := range []string{"internal", "configs"} {
		_, err := os.Stat(filepath.Join(out, absent))
		assert.True(t, os.IsNotExist(err), "%s must not exist when WithConfig=false", absent)
	}

	content, err := os.ReadFile(filepath.Join(out, "cmd/cmd/root.go"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "config.Load", "root.go must not call config.Load when WithConfig=false")
}

func TestCliCobraTemplate_withoutDocker(t *testing.T) {
	vars := defaultCobraVars()
	vars["WithDocker"] = false
	out := generateCobra(t, vars)

	for _, absent := range []string{"Dockerfile", ".dockerignore"} {
		_, err := os.Stat(filepath.Join(out, absent))
		assert.True(t, os.IsNotExist(err), "%s must not exist when WithDocker=false", absent)
	}

	content, err := os.ReadFile(filepath.Join(out, "Makefile"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "docker/build", "Makefile must not contain docker targets when WithDocker=false")
}

func TestCliCobraTemplate_namesRendered(t *testing.T) {
	out := generateCobra(t, defaultCobraVars())

	gomod, err := os.ReadFile(filepath.Join(out, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(gomod), "module github.com/acme/myapp")

	root, err := os.ReadFile(filepath.Join(out, "cmd/cmd/root.go"))
	require.NoError(t, err)
	assert.Contains(t, string(root), `Use:   "myapp"`)
	assert.Contains(t, string(root), "github.com/acme/myapp/cmd/cmd/version")
}

func TestCliCobraTemplate_versionVarsRendered(t *testing.T) {
	out := generateCobra(t, defaultCobraVars())

	ver, err := os.ReadFile(filepath.Join(out, "cmd/cmd/version/version.go"))
	require.NoError(t, err)
	assert.Contains(t, string(ver), `Version   = "0.1.0"`)
}

func TestCliCobraTemplate_miseEnvEscaped(t *testing.T) {
	vars := defaultCobraVars()
	vars["Toolchain"] = "mise"
	out := generateCobra(t, vars)

	content, err := os.ReadFile(filepath.Join(out, "mise.toml"))
	require.NoError(t, err)
	// The rendered file must contain mise's {{env.BIN}} syntax (not the escaped form)
	assert.Contains(t, string(content), "{{env.BIN}}")
	assert.Contains(t, string(content), "{{env.MODULE}}")
	assert.Contains(t, string(content), "{{env.TARGET}}")
}

// TestCliCobraTemplate_compile generates a project and verifies it compiles.
// Requires network access for `go mod tidy`. Skip with -short.
func TestCliCobraTemplate_compile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compile test in short mode")
	}

	out := generateCobra(t, defaultCobraVars())

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = out
	tidy.Stdout = os.Stdout
	tidy.Stderr = os.Stderr
	require.NoError(t, tidy.Run(), "go mod tidy failed")

	build := exec.Command("go", "build", "./...")
	build.Dir = out
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	require.NoError(t, build.Run(), "go build ./... failed")
}
