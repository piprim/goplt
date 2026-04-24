// cmd/goplt/cmd/generate_test.go
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathGuard(t *testing.T) {
	t.Run("same_dir", func(t *testing.T) {
		dir := t.TempDir()
		err := pathGuard(dir, dir)
		assert.ErrorContains(t, err, "overlap")
	})

	t.Run("output_inside_template", func(t *testing.T) {
		tmplDir := t.TempDir()
		output := filepath.Join(tmplDir, "out")
		require.NoError(t, os.MkdirAll(output, 0755))

		err := pathGuard(tmplDir, output)
		assert.ErrorContains(t, err, "overlap")
	})

	t.Run("template_inside_output", func(t *testing.T) {
		output := t.TempDir()
		tmplDir := filepath.Join(output, "tpl")
		require.NoError(t, os.MkdirAll(tmplDir, 0755))

		err := pathGuard(tmplDir, output)
		assert.ErrorContains(t, err, "overlap")
	})

	t.Run("disjoint_dirs", func(t *testing.T) {
		a := t.TempDir()
		b := t.TempDir()
		err := pathGuard(a, b)
		assert.NoError(t, err)
	})

	t.Run("output_not_exist_yet", func(t *testing.T) {
		tmplDir := t.TempDir()
		output := filepath.Join(t.TempDir(), "new-dir") // does not exist

		err := pathGuard(tmplDir, output)
		assert.NoError(t, err)
	})
}

func TestApplyTargetDir(t *testing.T) {
	t.Run("appended", func(t *testing.T) {
		vars := map[string]any{"Name": "myapp"}

		result, err := applyTargetDir("{{.Name}}", "/tmp/out", vars, false, [2]string{"{{", "}}"})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/out/myapp", result)
	})

	t.Run("skipped_when_explicit", func(t *testing.T) {
		vars := map[string]any{"Name": "myapp"}

		result, err := applyTargetDir("{{.Name}}", "/tmp/out", vars, true, [2]string{"{{", "}}"})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/out", result)
	})

	t.Run("skipped_when_empty", func(t *testing.T) {
		vars := map[string]any{"Name": "myapp"}

		result, err := applyTargetDir("", "/tmp/out", vars, false, [2]string{"{{", "}}"})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/out", result)
	})

	t.Run("traversal_rejected", func(t *testing.T) {
		vars := map[string]any{}

		_, err := applyTargetDir("../../etc", "/tmp/out", vars, false, [2]string{"{{", "}}"})
		assert.ErrorContains(t, err, "escapes the output directory")
	})

	t.Run("template_rendered", func(t *testing.T) {
		vars := map[string]any{"Name": "payment", "OrgPrefix": "github.com/acme"}

		result, err := applyTargetDir("{{.Name}}-svc", "/tmp/out", vars, false, [2]string{"{{", "}}"})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/out/payment-svc", result)
	})
}

func TestConfirmAndRunHooks(t *testing.T) {
	t.Run("no_hooks_is_noop", func(t *testing.T) {
		err := confirmAndRunHooks(&goplt.Manifest{}, t.TempDir(), true)
		assert.NoError(t, err)
	})

	t.Run("simple_command", func(t *testing.T) {
		dir := t.TempDir()
		m := &goplt.Manifest{
			Hooks: goplt.Hooks{PostGenHooks: goplt.PostGenHooks{"touch hook_ran.txt"}},
		}

		require.NoError(t, confirmAndRunHooks(m, dir, true))

		_, err := os.Stat(filepath.Join(dir, "hook_ran.txt"))
		assert.NoError(t, err, "hook must have created the file")
	})

	t.Run("quoted_argument_splits_correctly", func(t *testing.T) {
		// Regression: strings.Fields broke quoted arguments — "touch 'a b.txt'"
		// would produce args ["touch", "'a", "b.txt'"] instead of ["touch", "a b.txt"].
		// shlex.Split handles POSIX quoting so the file name is passed intact.
		dir := t.TempDir()
		m := &goplt.Manifest{
			Hooks: goplt.Hooks{PostGenHooks: goplt.PostGenHooks{`sh -c "touch 'file with spaces.txt'"`}},
		}

		require.NoError(t, confirmAndRunHooks(m, dir, true))

		_, err := os.Stat(filepath.Join(dir, "file with spaces.txt"))
		assert.NoError(t, err, "file with spaces must be created by the quoted hook argument")
	})

	t.Run("stops_on_first_failure", func(t *testing.T) {
		dir := t.TempDir()
		m := &goplt.Manifest{
			Hooks: goplt.Hooks{PostGenHooks: goplt.PostGenHooks{
				"false",
				"touch second_ran.txt",
			}},
		}

		err := confirmAndRunHooks(m, dir, true)
		assert.ErrorContains(t, err, "post-generate hooks")

		_, statErr := os.Stat(filepath.Join(dir, "second_ran.txt"))
		assert.True(t, os.IsNotExist(statErr), "second hook must not run after first fails")
	})
}

func TestRunGenerate(t *testing.T) {
	t.Run("remote_ref_returns_wrapped_error", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping network test in short mode")
		}

		err := runGenerate(context.Background(), "example.com/definitely/doesnotexist@v0.0.1", t.TempDir(), false, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolve remote template")
	})
}
