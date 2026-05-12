// registry_test.go
package goplt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRegistryModule(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid_github_path", "github.com/piprim/goplt-tmpl", false},
		{"valid_with_subpath", "github.com/acme/templates/collection", false},
		{"empty", "", true},
		{"leading_dot", "./local", true},
		{"leading_slash", "/abs/path", true},
		{"leading_tilde", "~/home", true},
		{"contains_at_version", "github.com/foo/bar@v1.0.0", true},
		{"no_dot_in_host", "localpkg", true},
		{"no_dot_in_first_segment", "single/segment", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRegistryModule(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestRegistry_emptyByDefault(t *testing.T) {
	var r Registry
	assert.Empty(t, r.Collections)
	assert.False(t, r.Has("github.com/foo/bar"))
}

func TestRegistry_Add(t *testing.T) {
	t.Run("adds_to_empty", func(t *testing.T) {
		var r Registry
		got, err := r.Add("github.com/piprim/goplt-tmpl")
		require.NoError(t, err)
		require.Len(t, got.Collections, 1)
		assert.Equal(t, "github.com/piprim/goplt-tmpl", got.Collections[0].Module)
	})

	t.Run("appends_in_order", func(t *testing.T) {
		var r Registry
		r, _ = r.Add("github.com/a/one")
		r, err := r.Add("github.com/b/two")
		require.NoError(t, err)
		require.Len(t, r.Collections, 2)
		assert.Equal(t, "github.com/a/one", r.Collections[0].Module)
		assert.Equal(t, "github.com/b/two", r.Collections[1].Module)
	})

	t.Run("rejects_duplicate", func(t *testing.T) {
		r := Registry{Collections: []RegistryEntry{{Module: "github.com/foo/bar"}}}
		_, err := r.Add("github.com/foo/bar")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("rejects_invalid", func(t *testing.T) {
		var r Registry
		_, err := r.Add("./local")
		require.Error(t, err)
	})

	t.Run("input_not_mutated_on_failure", func(t *testing.T) {
		r := Registry{Collections: []RegistryEntry{{Module: "github.com/foo/bar"}}}
		_, _ = r.Add("github.com/foo/bar")
		assert.Len(t, r.Collections, 1)
	})
}

func TestRegistry_Remove(t *testing.T) {
	t.Run("removes_present_entry", func(t *testing.T) {
		r := Registry{Collections: []RegistryEntry{
			{Module: "github.com/a/one"},
			{Module: "github.com/b/two"},
		}}
		got, err := r.Remove("github.com/a/one")
		require.NoError(t, err)
		require.Len(t, got.Collections, 1)
		assert.Equal(t, "github.com/b/two", got.Collections[0].Module)
	})

	t.Run("rejects_absent", func(t *testing.T) {
		r := Registry{Collections: []RegistryEntry{{Module: "github.com/a/one"}}}
		_, err := r.Remove("github.com/b/two")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})

	t.Run("rejects_empty_registry", func(t *testing.T) {
		var r Registry
		_, err := r.Remove("github.com/foo/bar")
		require.Error(t, err)
	})
}

func TestRegistryStore_Path(t *testing.T) {
	store := NewRegistryStore("/tmp/example/registry.toml")
	assert.Equal(t, "/tmp/example/registry.toml", store.Path())
}

func TestRegistryStore_Load_missingFile(t *testing.T) {
	store := NewRegistryStore(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	r, err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, r.Collections)
}

func TestRegistryStore_Load_parseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	require.NoError(t, os.WriteFile(path, []byte("this is = not valid TOML ["), 0o644))

	store := NewRegistryStore(path)
	_, err := store.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse registry")
}

func TestRegistryStore_Save_roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.toml")
	store := NewRegistryStore(path)

	in := Registry{Collections: []RegistryEntry{
		{Module: "github.com/a/one"},
		{Module: "github.com/b/two"},
	}}

	require.NoError(t, store.Save(in))

	got, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

func TestRegistryStore_Save_createsParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "registry.toml")
	store := NewRegistryStore(path)

	require.NoError(t, store.Save(Registry{}))

	info, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestDefaultRegistryStore_path(t *testing.T) {
	store, err := DefaultRegistryStore()
	require.NoError(t, err)
	require.NotNil(t, store)

	want := filepath.Join("goplt", "registry.toml")
	assert.True(t,
		filepath.Base(filepath.Dir(store.Path())) == "goplt" &&
			filepath.Base(store.Path()) == "registry.toml",
		"path %q does not end with %q", store.Path(), want,
	)
}
