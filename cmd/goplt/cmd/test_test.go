package cmd

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDefaultVars_kindText_withDefault(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "OrgPrefix", Kind: goplt.KindText, Default: "github.com/acme"},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, "github.com/acme", result["OrgPrefix"])
}

func TestBuildDefaultVars_kindText_emptyDefault_usesName(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "Name", Kind: goplt.KindText, Default: ""},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, "Name", result["Name"])
}

func TestBuildDefaultVars_kindBool(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "WithDocker", Kind: goplt.KindBool, Default: true},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, true, result["WithDocker"])
}

func TestBuildDefaultVars_kindChoiceString_firstElement(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "License", Kind: goplt.KindChoiceString, Default: []string{"MIT", "Apache-2.0"}},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, "MIT", result["License"])
}

func TestBuildScript_noHooks(t *testing.T) {
	script := buildScript(nil)
	assert.Contains(t, script, "set -e")
	assert.Contains(t, script, "tar xf -")
	assert.Contains(t, script, "go build ./...")
	assert.Contains(t, script, "go test ./...")
}

func TestBuildScript_withHooks_appearsBeforeBuild(t *testing.T) {
	script := buildScript([]string{"go mod tidy", "git init"})
	assert.Contains(t, script, "go mod tidy")
	assert.Contains(t, script, "git init")
	tidyIdx := strings.Index(script, "go mod tidy")
	buildIdx := strings.Index(script, "go build ./...")
	assert.Less(t, tidyIdx, buildIdx, "hooks must appear before go build")
}

func TestBuildScript_preambleOrder(t *testing.T) {
	script := buildScript(nil)
	eIdx := strings.Index(script, "set -e")
	tarIdx := strings.Index(script, "tar xf -")
	buildIdx := strings.Index(script, "go build ./...")
	testIdx := strings.Index(script, "go test ./...")
	assert.Less(t, eIdx, tarIdx, "set -e must precede tar")
	assert.Less(t, tarIdx, buildIdx, "tar must precede go build")
	assert.Less(t, buildIdx, testIdx, "go build must precede go test")
}

func TestBuildDefaultVars_emptyList(t *testing.T) {
	result := buildDefaultVars(nil)
	assert.Empty(t, result)
}

func TestBuildDefaultVars_kindBool_wrongType_fallsBackToFalse(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "Flag", Kind: goplt.KindBool, Default: "not-a-bool"},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, false, result["Flag"])
}

func TestBuildDefaultVars_kindChoiceString_emptyChoices_fallsBackToEmpty(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "License", Kind: goplt.KindChoiceString, Default: []string{}},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, "", result["License"])
}

// buildTar helpers

func readTar(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		content, err := io.ReadAll(tr)
		require.NoError(t, err)
		files[hdr.Name] = content
	}
	return files
}

func TestBuildTar_singleFile_contentPreserved(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0600))

	data, err := buildTar(dir)
	require.NoError(t, err)

	files := readTar(t, data)
	require.Contains(t, files, "main.go")
	assert.Equal(t, "package main", string(files["main.go"]))
}

func TestBuildTar_directoryStructure_forwardSlashes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "root"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "root", "root.go"), []byte("package root"), 0600))

	data, err := buildTar(dir)
	require.NoError(t, err)

	files := readTar(t, data)
	// Header names must use forward slashes regardless of OS.
	assert.Contains(t, files, "cmd/root/root.go")
	assert.Equal(t, "package root", string(files["cmd/root/root.go"]))
}

func TestBuildTar_multipleFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0600))

	data, err := buildTar(dir)
	require.NoError(t, err)

	files := readTar(t, data)
	assert.Equal(t, "a", string(files["a.go"]))
	assert.Equal(t, "b", string(files["b.go"]))
}

func TestBuildTar_emptyDir_producesValidTar(t *testing.T) {
	dir := t.TempDir()
	data, err := buildTar(dir)
	require.NoError(t, err)

	// A valid (empty) tar archive must still be readable.
	files := readTar(t, data)
	assert.Empty(t, files)
}

func TestBuildTar_nonexistentDir_returnsError(t *testing.T) {
	_, err := buildTar(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}
