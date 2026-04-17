package goplt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHooks_empty(t *testing.T) {
	m := &goplt.Manifest{}
	err := goplt.RunHooks(m, t.TempDir())
	assert.NoError(t, err)
}

func TestRunHooks_createsFile(t *testing.T) {
	dir := t.TempDir()
	m := &goplt.Manifest{
		Hooks: goplt.Hooks{
			PostGenHooks: goplt.PostGenHooks{"touch hook_ran.txt"},
		},
	}

	err := goplt.RunHooks(m, dir)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "hook_ran.txt"))
	assert.NoError(t, err, "hook must have created the file")
}

func TestRunHooks_stopsOnError(t *testing.T) {
	dir := t.TempDir()
	m := &goplt.Manifest{
		Hooks: goplt.Hooks{
			PostGenHooks: goplt.PostGenHooks{
				"false",                // exits non-zero
				"touch second_ran.txt", // must not run
			},
		},
	}

	err := goplt.RunHooks(m, dir)
	assert.ErrorContains(t, err, "false")

	_, statErr := os.Stat(filepath.Join(dir, "second_ran.txt"))
	assert.True(t, os.IsNotExist(statErr), "second hook must not have run")
}
