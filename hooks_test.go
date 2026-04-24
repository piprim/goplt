package goplt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHooks(t *testing.T) {
	t.Run("empty_manifest_no_error", func(t *testing.T) {
		m := &goplt.Manifest{}
		err := goplt.RunHooks(t.Context(), m, t.TempDir())
		assert.NoError(t, err)
	})

	t.Run("creates_file", func(t *testing.T) {
		dir := t.TempDir()
		m := &goplt.Manifest{
			Hooks: goplt.Hooks{
				PostGenHooks: goplt.PostGenHooks{"touch hook_ran.txt"},
			},
		}

		err := goplt.RunHooks(t.Context(), m, dir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dir, "hook_ran.txt"))
		assert.NoError(t, err, "hook must have created the file")
	})

	t.Run("quoted_argument", func(t *testing.T) {
		dir := t.TempDir()
		m := &goplt.Manifest{
			Hooks: goplt.Hooks{
				PostGenHooks: goplt.PostGenHooks{`sh -c "touch 'file with spaces.txt'"`},
			},
		}

		err := goplt.RunHooks(t.Context(), m, dir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dir, "file with spaces.txt"))
		assert.NoError(t, err, "hook with quoted argument must create the file")
	})

	t.Run("stops_on_error", func(t *testing.T) {
		dir := t.TempDir()
		m := &goplt.Manifest{
			Hooks: goplt.Hooks{
				PostGenHooks: goplt.PostGenHooks{
					"false",                // exits non-zero
					"touch second_ran.txt", // must not run
				},
			},
		}

		err := goplt.RunHooks(t.Context(), m, dir)
		assert.ErrorContains(t, err, "false")

		_, statErr := os.Stat(filepath.Join(dir, "second_ran.txt"))
		assert.True(t, os.IsNotExist(statErr), "second hook must not have run")
	})
}
