// cmd/goplt/cmd/init_test.go
package cmd

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/piprim/goplt"
	"github.com/piprim/goplt/cmd/goplt/inittempl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goLibrarySimpleFS returns the embedded go-library-simple sub-FS.
func goLibrarySimpleFS(t *testing.T) fs.FS {
	t.Helper()
	subFS, err := fs.Sub(inittempl.FS, "templates/go-library-simple")
	require.NoError(t, err)
	return subFS
}

// runInitWithVars bypasses TUI: loads the meta-manifest and calls Generate directly.
func runInitWithVars(t *testing.T, vars map[string]any) string {
	t.Helper()
	subFS := goLibrarySimpleFS(t)
	m, err := goplt.LoadManifest(subFS)
	require.NoError(t, err)
	out := t.TempDir()
	require.NoError(t, goplt.Generate(subFS, m, out, vars))
	return out
}

// defaultInitVars returns a complete variable map for the meta-template.
func defaultInitVars(complexity, toolchain string) map[string]any {
	return map[string]any{
		"Name":        "mylib",
		"Description": "A test library",
		"Complexity":  complexity,
		"License":     "MIT",
		"Toolchain":   toolchain,
	}
}

func TestRunInit_minimal_fileSet(t *testing.T) {
	out := runInitWithVars(t, defaultInitVars("minimal", "make"))

	present := []string{
		"{{.Name}}.go.tmpl",
		"{{.Name}}_test.go.tmpl",
		"go.mod.tmpl",
		"README.md.tmpl",
		".gitignore",
		"template.toml",
	}
	for _, f := range present {
		_, err := os.Stat(filepath.Join(out, f))
		assert.NoError(t, err, "expected %s to exist for minimal", f)
	}

	absent := []string{
		".golangci.yml",
		"{{.Name}}_example_test.go.tmpl",
		"Makefile.tmpl",
		"mise.toml.tmpl",
	}
	for _, f := range absent {
		_, err := os.Stat(filepath.Join(out, f))
		assert.True(t, os.IsNotExist(err), "%s must not exist for minimal", f)
	}

	_, err := os.Stat(filepath.Join(out, "internal"))
	assert.True(t, os.IsNotExist(err), "internal/ must not exist for minimal")
}

func TestRunInit_standard_fileSet(t *testing.T) {
	out := runInitWithVars(t, defaultInitVars("standard", "make"))

	present := []string{
		"{{.Name}}.go.tmpl",
		"{{.Name}}_test.go.tmpl",
		"{{.Name}}_example_test.go.tmpl",
		"go.mod.tmpl",
		".golangci.yml",
		"README.md.tmpl",
		".gitignore",
		"template.toml",
	}
	for _, f := range present {
		_, err := os.Stat(filepath.Join(out, f))
		assert.NoError(t, err, "expected %s to exist for standard", f)
	}

	absent := []string{"Makefile.tmpl", "mise.toml.tmpl"}
	for _, f := range absent {
		_, err := os.Stat(filepath.Join(out, f))
		assert.True(t, os.IsNotExist(err), "%s must not exist for standard", f)
	}

	_, err := os.Stat(filepath.Join(out, "internal"))
	assert.True(t, os.IsNotExist(err), "internal/ must not exist for standard")
}

func TestRunInit_advanced_make_fileSet(t *testing.T) {
	out := runInitWithVars(t, defaultInitVars("advanced", "make"))

	present := []string{
		"{{.Name}}.go.tmpl",
		"{{.Name}}_test.go.tmpl",
		"{{.Name}}_example_test.go.tmpl",
		"go.mod.tmpl",
		".golangci.yml",
		"Makefile.tmpl",
		"template.toml",
	}
	for _, f := range present {
		_, err := os.Stat(filepath.Join(out, f))
		assert.NoError(t, err, "expected %s to exist for advanced+make", f)
	}

	_, err := os.Stat(filepath.Join(out, "internal"))
	assert.NoError(t, err, "internal/ must exist for advanced")

	_, err = os.Stat(filepath.Join(out, "mise.toml.tmpl"))
	assert.True(t, os.IsNotExist(err), "mise.toml.tmpl must not exist for toolchain=make")
}

func TestRunInit_advanced_mise_fileSet(t *testing.T) {
	out := runInitWithVars(t, defaultInitVars("advanced", "mise"))

	_, err := os.Stat(filepath.Join(out, "mise.toml.tmpl"))
	assert.NoError(t, err, "mise.toml.tmpl must exist for toolchain=mise")

	_, err = os.Stat(filepath.Join(out, "Makefile.tmpl"))
	assert.True(t, os.IsNotExist(err), "Makefile.tmpl must not exist for toolchain=mise")
}

func TestRunInit_literalBraceDelimiters(t *testing.T) {
	out := runInitWithVars(t, defaultInitVars("minimal", "make"))

	content, err := os.ReadFile(filepath.Join(out, "{{.Name}}.go.tmpl"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "{{snake .Name}}", "generated .go.tmpl must contain literal {{ }} template syntax")
}

func TestRunInit_targetDir_rendersWithMetaDelimiters(t *testing.T) {
	subFS := goLibrarySimpleFS(t)
	m, err := goplt.LoadManifest(subFS)
	require.NoError(t, err)

	base := t.TempDir()
	vars := map[string]any{"Name": "mylib"}
	result, err := applyTargetDir(m.TargetDir, base, vars, false, m.Delimiters)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(base, "mylib"), result)
}

func TestRunInit_templateToml_present(t *testing.T) {
	out := runInitWithVars(t, defaultInitVars("minimal", "make"))

	content, err := os.ReadFile(filepath.Join(out, "template.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "[variables]")
}

// defaultVarsFromManifest collects default variable values from a manifest without
// a TUI, following the same rules as goplt test: text variables with an empty
// default use the variable name itself as the value.
func defaultVarsFromManifest(m *goplt.Manifest) map[string]any {
	vars := make(map[string]any, len(m.Variables))
	for _, v := range m.Variables {
		switch v.Kind {
		case goplt.KindInput: // KindText is an alias, both work
			if s, _ := v.Value.(string); s != "" {
				vars[v.Name] = s
			} else {
				vars[v.Name] = v.Name
			}
		case goplt.KindBool:
			if b, ok := v.Value.(bool); ok {
				vars[v.Name] = b
			} else {
				vars[v.Name] = false
			}
		case goplt.KindStringChoice:
			choices, _ := v.Value.([]string)
			if len(choices) > 0 {
				vars[v.Name] = choices[0]
			}
		case goplt.KindStringList:
			if items, ok := v.Value.([]string); ok && len(items) > 0 {
				vars[v.Name] = items
			} else {
				vars[v.Name] = []string{v.Name}
			}
		}
	}
	return vars
}

func TestGoLibrarySimple_endToEnd(t *testing.T) {
	cases := []struct {
		complexity string
		toolchain  string
	}{
		{"minimal", "make"},
		{"standard", "make"},
		{"advanced", "make"},
		{"advanced", "mise"},
	}

	for _, tc := range cases {
		t.Run(tc.complexity+"/"+tc.toolchain, func(t *testing.T) {
			t.Parallel()

			// Step 1: generate the template skeleton (goplt init output).
			skeletonDir := runInitWithVars(t, defaultInitVars(tc.complexity, tc.toolchain))

			// Step 2: load the skeleton's template.toml and collect defaults.
			skeletonFS := os.DirFS(skeletonDir)
			skeletonM, err := goplt.LoadManifest(skeletonFS)
			require.NoError(t, err)
			finalVars := defaultVarsFromManifest(skeletonM)

			// Step 3: generate the final Go project from the skeleton.
			finalDir := t.TempDir()
			require.NoError(t, goplt.Generate(skeletonFS, skeletonM, finalDir, finalVars))

			// Step 4: the generated project must build and its tests must pass.
			cmd := exec.CommandContext(t.Context(), "go", "test", "./...")
			cmd.Dir = finalDir
			out, err := cmd.CombinedOutput()
			assert.NoError(t, err, "go test ./... failed (complexity=%s toolchain=%s):\n%s",
				tc.complexity, tc.toolchain, out)
		})
	}
}
