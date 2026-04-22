// cmd/goplt/cmd/generate_test.go
package cmd

import (
	"context"
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

func TestApplyTargetDir_appended(t *testing.T) {
	vars := map[string]any{"Name": "myapp"}
	base := "/tmp/out"

	result, err := applyTargetDir("{{.Name}}", base, vars, false, [2]string{"{{", "}}"})
	require.NoError(t, err)
	assert.Equal(t, "/tmp/out/myapp", result)
}

func TestApplyTargetDir_skippedWhenExplicit(t *testing.T) {
	vars := map[string]any{"Name": "myapp"}

	result, err := applyTargetDir("{{.Name}}", "/tmp/out", vars, true, [2]string{"{{", "}}"})
	require.NoError(t, err)
	assert.Equal(t, "/tmp/out", result)
}

func TestApplyTargetDir_skippedWhenEmpty(t *testing.T) {
	vars := map[string]any{"Name": "myapp"}

	result, err := applyTargetDir("", "/tmp/out", vars, false, [2]string{"{{", "}}"})
	require.NoError(t, err)
	assert.Equal(t, "/tmp/out", result)
}

func TestApplyTargetDir_traversalRejected(t *testing.T) {
	vars := map[string]any{}

	_, err := applyTargetDir("../../etc", "/tmp/out", vars, false, [2]string{"{{", "}}"})
	assert.ErrorContains(t, err, "escapes the output directory")
}

func TestApplyTargetDir_templateRendered(t *testing.T) {
	vars := map[string]any{"Name": "payment", "OrgPrefix": "github.com/acme"}

	result, err := applyTargetDir("{{.Name}}-svc", "/tmp/out", vars, false, [2]string{"{{", "}}"})
	require.NoError(t, err)
	assert.Equal(t, "/tmp/out/payment-svc", result)
}

func TestRunGenerate_remoteRefReturnsWrappedError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	err := runGenerate(context.Background(), "example.com/definitely/doesnotexist@v0.0.1", t.TempDir(), false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve remote template")
}
