// cmd/goplt/cmd/generate_test.go
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathGuard_sameDir(t *testing.T) {
	dir := t.TempDir()
	err := pathGuard(dir, dir)
	assert.ErrorContains(t, err, "overlap")
}

func TestPathGuard_outputInsideTemplate(t *testing.T) {
	tmplDir := t.TempDir()
	output := filepath.Join(tmplDir, "out")
	require.NoError(t, os.MkdirAll(output, 0755))

	err := pathGuard(tmplDir, output)
	assert.ErrorContains(t, err, "overlap")
}

func TestPathGuard_templateInsideOutput(t *testing.T) {
	output := t.TempDir()
	tmplDir := filepath.Join(output, "tpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))

	err := pathGuard(tmplDir, output)
	assert.ErrorContains(t, err, "overlap")
}

func TestPathGuard_disjointDirs(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	err := pathGuard(a, b)
	assert.NoError(t, err)
}

func TestPathGuard_outputNotExistYet(t *testing.T) {
	tmplDir := t.TempDir()
	output := filepath.Join(t.TempDir(), "new-dir") // does not exist

	err := pathGuard(tmplDir, output)
	assert.NoError(t, err)
}
