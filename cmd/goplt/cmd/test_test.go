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

func TestBuildDefaultVars(t *testing.T) {
	t.Run("empty_list", func(t *testing.T) {
		result := buildDefaultVars(nil)
		assert.Empty(t, result)
	})

	t.Run("kind_text/with_default", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "OrgPrefix", Kind: goplt.KindText, Value: "github.com/acme"},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, "github.com/acme", result["OrgPrefix"])
	})

	t.Run("kind_text/empty_default_uses_name", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "Name", Kind: goplt.KindText, Required: true},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, "Name", result["Name"])
	})

	t.Run("kind_bool/true_value", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "WithDocker", Kind: goplt.KindBool, Value: true},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, true, result["WithDocker"])
	})

	t.Run("kind_bool/wrong_type_falls_back_to_false", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "Flag", Kind: goplt.KindBool, Value: "not-a-bool"},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, false, result["Flag"])
	})

	t.Run("kind_string_choice/first_element", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "License", Kind: goplt.KindStringChoice, Value: []string{"MIT", "Apache-2.0"}},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, "MIT", result["License"])
	})

	t.Run("kind_string_choice/empty_choices_falls_back_to_empty", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "License", Kind: goplt.KindStringChoice, Value: []string{}},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, "", result["License"])
	})

	t.Run("kind_string_list/with_suggestions", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "Packages", Kind: goplt.KindStringList, Value: []string{"auth", "hash"}},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, []string{"auth", "hash"}, result["Packages"])
	})

	t.Run("kind_string_list/no_suggestions_uses_name", func(t *testing.T) {
		vars := []goplt.Variable{
			{Name: "Packages", Kind: goplt.KindStringList},
		}

		result := buildDefaultVars(vars)
		assert.Equal(t, []string{"Packages"}, result["Packages"])
	})
}

func TestBuildScript(t *testing.T) {
	t.Run("no_hooks", func(t *testing.T) {
		script := buildScript(nil)
		assert.Contains(t, script, "set -e")
		assert.Contains(t, script, "tar xf -")
		assert.Contains(t, script, "go build ./...")
		assert.Contains(t, script, "go test ./...")
	})

	t.Run("hooks_appear_before_build", func(t *testing.T) {
		script := buildScript([]string{"go mod tidy", "git init"})
		assert.Contains(t, script, "go mod tidy")
		assert.Contains(t, script, "git init")
		tidyIdx := strings.Index(script, "go mod tidy")
		buildIdx := strings.Index(script, "go build ./...")
		assert.Less(t, tidyIdx, buildIdx, "hooks must appear before go build")
	})

	t.Run("preamble_order", func(t *testing.T) {
		script := buildScript(nil)
		eIdx := strings.Index(script, "set -e")
		tarIdx := strings.Index(script, "tar xf -")
		buildIdx := strings.Index(script, "go build ./...")
		testIdx := strings.Index(script, "go test ./...")
		assert.Less(t, eIdx, tarIdx, "set -e must precede tar")
		assert.Less(t, tarIdx, buildIdx, "tar must precede go build")
		assert.Less(t, buildIdx, testIdx, "go build must precede go test")
	})
}

// readTar decodes a tar archive into a name→content map.
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

func TestBuildTar(t *testing.T) {
	t.Run("single_file_content_preserved", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0600))

		data, err := buildTar(dir)
		require.NoError(t, err)

		files := readTar(t, data)
		require.Contains(t, files, "main.go")
		assert.Equal(t, "package main", string(files["main.go"]))
	})

	t.Run("directory_structure_forward_slashes", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "root"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "root", "root.go"), []byte("package root"), 0600))

		data, err := buildTar(dir)
		require.NoError(t, err)

		// Header names must use forward slashes regardless of OS.
		files := readTar(t, data)
		assert.Contains(t, files, "cmd/root/root.go")
		assert.Equal(t, "package root", string(files["cmd/root/root.go"]))
	})

	t.Run("multiple_files", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0600))

		data, err := buildTar(dir)
		require.NoError(t, err)

		files := readTar(t, data)
		assert.Equal(t, "a", string(files["a.go"]))
		assert.Equal(t, "b", string(files["b.go"]))
	})

	t.Run("empty_dir_produces_valid_tar", func(t *testing.T) {
		dir := t.TempDir()

		data, err := buildTar(dir)
		require.NoError(t, err)

		// A valid (empty) tar archive must still be readable.
		files := readTar(t, data)
		assert.Empty(t, files)
	})

	t.Run("nonexistent_dir_returns_error", func(t *testing.T) {
		_, err := buildTar(filepath.Join(t.TempDir(), "does-not-exist"))
		require.Error(t, err)
	})
}
