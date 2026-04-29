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

func TestReadModulePath(t *testing.T) {
	t.Parallel()

	t.Run("reads module path from go.mod", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/acme/my-app\n\ngo 1.22\n"), 0600))

		got, err := goplt.ReadModulePath(dir)
		require.NoError(t, err)
		assert.Equal(t, "github.com/acme/my-app", got)
	})

	t.Run("errors when go.mod is absent", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		_, err := goplt.ReadModulePath(dir)
		assert.ErrorContains(t, err, "go.mod")
	})

	t.Run("errors when no module directive", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("go 1.22\n"), 0600))
		_, err := goplt.ReadModulePath(dir)
		assert.ErrorContains(t, err, "module directive")
	})
}

func TestBuildSubstitutions(t *testing.T) {
	t.Parallel()

	t.Run("derives all case forms", func(t *testing.T) {
		t.Parallel()
		subs := goplt.BuildSubstitutions("my-app", "github.com/acme", nil)

		values := make([]string, len(subs))
		for i, s := range subs {
			values[i] = s.Value
		}

		assert.Contains(t, values, "my-app")
		assert.Contains(t, values, "MyApp")
		assert.Contains(t, values, "myApp")
		assert.Contains(t, values, "my_app")
		assert.Contains(t, values, "github.com/acme")
	})

	t.Run("sorted by length descending", func(t *testing.T) {
		t.Parallel()
		subs := goplt.BuildSubstitutions("my-app", "github.com/acme", nil)
		for i := 1; i < len(subs); i++ {
			assert.GreaterOrEqual(t, len(subs[i-1].Value), len(subs[i].Value),
				"subs[%d].Value=%q shorter than subs[%d].Value=%q", i-1, subs[i-1].Value, i, subs[i].Value)
		}
	})

	t.Run("skip strings become identity pairs ordered before substitutions", func(t *testing.T) {
		t.Parallel()
		subs := goplt.BuildSubstitutions("app", "github.com/acme", []string{"application"})

		var appIdx, applicationIdx int
		for i, s := range subs {
			switch s.Value {
			case "app":
				appIdx = i
			case "application":
				applicationIdx = i
			}
		}
		assert.Less(t, applicationIdx, appIdx, "application must precede app")

		for _, s := range subs {
			if s.Value == "application" {
				assert.Equal(t, "application", s.Placeholder)
			}
		}
	})

	t.Run("deduplicates identical case forms", func(t *testing.T) {
		t.Parallel()
		subs := goplt.BuildSubstitutions("foo", "github.com/acme", nil)
		seen := map[string]int{}
		for _, s := range subs {
			seen[s.Value]++
		}
		for v, count := range seen {
			assert.Equal(t, 1, count, "duplicate value %q", v)
		}
	})

	t.Run("correct placeholder for each form", func(t *testing.T) {
		t.Parallel()
		subs := goplt.BuildSubstitutions("my-app", "github.com/acme", nil)
		byValue := map[string]string{}
		for _, s := range subs {
			byValue[s.Value] = s.Placeholder
		}
		assert.Equal(t, "{{.Name}}", byValue["my-app"])
		assert.Equal(t, "{{.Name | pascal}}", byValue["MyApp"])
		assert.Equal(t, "{{.Name | camel}}", byValue["myApp"])
		assert.Equal(t, "{{.Name | snake}}", byValue["my_app"])
		assert.Equal(t, "{{.OrgPrefix}}", byValue["github.com/acme"])
	})
}

func TestTemplatize(t *testing.T) {
	t.Parallel()

	subs := []goplt.Substitution{
		{Value: "my-app", Placeholder: "{{.Name}}"},
		{Value: "MyApp", Placeholder: "{{.Name | pascal}}"},
		{Value: "github.com/acme", Placeholder: "{{.OrgPrefix}}"},
	}

	t.Run("substitutes in file content", func(t *testing.T) {
		t.Parallel()
		fsys := fstest.MapFS{
			"go.mod": {Data: []byte("module github.com/acme/my-app\n")},
		}
		out := t.TempDir()
		report, err := goplt.Templatize(fsys, out, subs)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(out, "go.mod"))
		require.NoError(t, err)
		assert.Equal(t, "module {{.OrgPrefix}}/{{.Name}}\n", string(content))

		assert.Greater(t, len(report.Results), 0)
	})

	t.Run("substitutes in file path", func(t *testing.T) {
		t.Parallel()
		fsys := fstest.MapFS{
			"cmd/my-app/main.go": {Data: []byte("package main\n")},
		}
		out := t.TempDir()
		_, err := goplt.Templatize(fsys, out, subs)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(out, "cmd/{{.Name}}/main.go"))
		assert.NoError(t, err)
	})

	t.Run("skips .git directory", func(t *testing.T) {
		t.Parallel()
		fsys := fstest.MapFS{
			".git/HEAD": {Data: []byte("ref: refs/heads/main\n")},
			"main.go":   {Data: []byte("package main\n")},
		}
		out := t.TempDir()
		_, err := goplt.Templatize(fsys, out, subs)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(out, ".git"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("copies binary files verbatim", func(t *testing.T) {
		t.Parallel()
		binary := []byte{0x00, 0x01, 0x02, 0x03, 'm', 'y', '-', 'a', 'p', 'p'}
		fsys := fstest.MapFS{
			"asset.bin": {Data: binary},
		}
		out := t.TempDir()
		report, err := goplt.Templatize(fsys, out, subs)
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(out, "asset.bin"))
		require.NoError(t, err)
		assert.Equal(t, binary, got)
		assert.Contains(t, report.BinaryFiles, "asset.bin")
	})

	t.Run("report counts occurrences", func(t *testing.T) {
		t.Parallel()
		fsys := fstest.MapFS{
			"a.go": {Data: []byte("// my-app my-app\npackage my_app\n")},
		}
		out := t.TempDir()
		report, err := goplt.Templatize(fsys, out, subs)
		require.NoError(t, err)

		byFrom := map[string]int{}
		for _, r := range report.Results {
			byFrom[r.From] = r.Count
		}
		assert.Equal(t, 2, byFrom["my-app"])
	})

	t.Run("identity pairs protect strings from substitution", func(t *testing.T) {
		t.Parallel()
		skipSubs := []goplt.Substitution{
			{Value: "application", Placeholder: "application"},
			{Value: "app", Placeholder: "{{.Name}}"},
		}
		fsys := fstest.MapFS{
			"main.go": {Data: []byte("// application uses app\n")},
		}
		out := t.TempDir()
		report, err := goplt.Templatize(fsys, out, skipSubs)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(out, "main.go"))
		require.NoError(t, err)
		assert.Equal(t, "// application uses {{.Name}}\n", string(content))
		assert.Equal(t, 1, report.Skipped["application"])
	})
}
