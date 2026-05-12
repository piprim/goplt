// cmd/goplt/cmd/list_test.go
package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterTemplates_NoFilters(t *testing.T) {
	entries := []templateEntry{
		{modPath: "a/tmpl", tags: []string{"go", "cli"}, authors: []string{"Alice"}},
		{modPath: "b/tmpl", tags: []string{"go", "web"}, authors: []string{"Bob"}},
	}

	got := filterTemplates(entries, nil, "", false)
	assert.Equal(t, entries, got)
}

func TestFilterTemplates_TagAndMode(t *testing.T) {
	entries := []templateEntry{
		{modPath: "a/tmpl", tags: []string{"go", "cli"}},
		{modPath: "b/tmpl", tags: []string{"go", "web"}},
		{modPath: "c/tmpl", tags: []string{"rust"}},
	}

	t.Run("all_tags_present", func(t *testing.T) {
		got := filterTemplates(entries, []string{"go", "cli"}, "", false)
		assert.Len(t, got, 1)
		assert.Equal(t, "a/tmpl", got[0].modPath)
	})

	t.Run("single_tag_matches_multiple", func(t *testing.T) {
		got := filterTemplates(entries, []string{"go"}, "", false)
		assert.Len(t, got, 2)
	})

	t.Run("no_entry_has_all_tags", func(t *testing.T) {
		got := filterTemplates(entries, []string{"go", "rust"}, "", false)
		assert.Empty(t, got)
	})
}

func TestFilterTemplates_TagOrMode(t *testing.T) {
	entries := []templateEntry{
		{modPath: "a/tmpl", tags: []string{"go", "cli"}},
		{modPath: "b/tmpl", tags: []string{"go", "web"}},
		{modPath: "c/tmpl", tags: []string{"rust"}},
	}

	got := filterTemplates(entries, []string{"cli", "rust"}, "", true)
	assert.Len(t, got, 2)
	assert.Equal(t, "a/tmpl", got[0].modPath)
	assert.Equal(t, "c/tmpl", got[1].modPath)
}

func TestFilterTemplates_AuthorFilter(t *testing.T) {
	entries := []templateEntry{
		{modPath: "a/tmpl", authors: []string{"Alice"}},
		{modPath: "b/tmpl", authors: []string{"Bob"}},
		{modPath: "c/tmpl", authors: []string{"Alice", "Carol"}},
	}

	t.Run("exact_match", func(t *testing.T) {
		got := filterTemplates(entries, nil, "Bob", false)
		assert.Len(t, got, 1)
		assert.Equal(t, "b/tmpl", got[0].modPath)
	})

	t.Run("case_insensitive", func(t *testing.T) {
		got := filterTemplates(entries, nil, "ALICE", false)
		assert.Len(t, got, 2) // a/tmpl and c/tmpl
	})

	t.Run("substring_match", func(t *testing.T) {
		// "alic" matches "Alice" in a/tmpl and c/tmpl
		got := filterTemplates(entries, nil, "alic", false)
		assert.Len(t, got, 2)
	})

	t.Run("no_match", func(t *testing.T) {
		got := filterTemplates(entries, nil, "dave", false)
		assert.Empty(t, got)
	})
}

func TestFilterTemplates_TagAndAuthorCombined(t *testing.T) {
	entries := []templateEntry{
		{modPath: "a/tmpl", tags: []string{"go", "cli"}, authors: []string{"Alice"}},
		{modPath: "b/tmpl", tags: []string{"go", "web"}, authors: []string{"Alice"}},
		{modPath: "c/tmpl", tags: []string{"go", "cli"}, authors: []string{"Bob"}},
	}

	got := filterTemplates(entries, []string{"go", "cli"}, "alice", false)
	assert.Len(t, got, 1)
	assert.Equal(t, "a/tmpl", got[0].modPath)
}

func TestRefreshRegistry_allSuccess(t *testing.T) {
	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{
			{Module: "github.com/a/one"},
			{Module: "github.com/b/two"},
		},
	}))

	resolve := func(_ context.Context, module string) (string, string, error) {
		return "/cache/" + module, "v1.2.3", nil
	}

	var stderr bytes.Buffer
	refreshed, failed, err := refreshRegistry(context.Background(), store, resolve, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 2, refreshed)
	assert.Equal(t, 0, failed)
	assert.Contains(t, stderr.String(), "refreshed github.com/a/one@v1.2.3")
	assert.Contains(t, stderr.String(), "refreshed github.com/b/two@v1.2.3")
	assert.NotContains(t, stderr.String(), "WARN")
}

func TestRefreshRegistry_partialFailure(t *testing.T) {
	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{
			{Module: "github.com/a/one"},
			{Module: "github.com/b/two"},
		},
	}))

	resolve := func(_ context.Context, module string) (string, string, error) {
		if module == "github.com/b/two" {
			return "", "", errors.New("simulated failure")
		}

		return "/cache/" + module, "v1.0.0", nil
	}

	var stderr bytes.Buffer
	refreshed, failed, err := refreshRegistry(context.Background(), store, resolve, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 1, refreshed)
	assert.Equal(t, 1, failed)
	assert.Contains(t, stderr.String(), "refreshed github.com/a/one@v1.0.0")
	assert.Contains(t, stderr.String(), "WARN: failed to refresh github.com/b/two: simulated failure")
}

func TestRefreshRegistry_allFail(t *testing.T) {
	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{{Module: "github.com/a/one"}},
	}))

	resolve := func(_ context.Context, _ string) (string, string, error) {
		return "", "", errors.New("boom")
	}

	var stderr bytes.Buffer
	refreshed, failed, err := refreshRegistry(context.Background(), store, resolve, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, refreshed)
	assert.Equal(t, 1, failed)
	assert.Contains(t, stderr.String(), "WARN: failed to refresh github.com/a/one: boom")
}

func TestRefreshRegistry_emptyRegistry(t *testing.T) {
	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))

	resolve := func(_ context.Context, _ string) (string, string, error) {
		t.Fatal("resolve must not be called when registry is empty")
		return "", "", nil
	}

	var stderr bytes.Buffer
	refreshed, failed, err := refreshRegistry(context.Background(), store, resolve, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, refreshed)
	assert.Equal(t, 0, failed)
	assert.Empty(t, stderr.String())
}

func TestRefreshRegistry_loadError(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "registry.toml")
	require.NoError(t, os.WriteFile(bad, []byte("this is = not valid TOML ["), 0o644))

	store := goplt.NewRegistryStore(bad)

	resolve := func(_ context.Context, _ string) (string, string, error) {
		t.Fatal("resolve must not be called when load fails")
		return "", "", nil
	}

	var stderr bytes.Buffer
	refreshed, failed, err := refreshRegistry(context.Background(), store, resolve, &stderr)

	require.Error(t, err)
	assert.Equal(t, 0, refreshed)
	assert.Equal(t, 0, failed)
}

func TestRunList_remote_emptyRegistryPrintsHint(t *testing.T) {
	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))

	prevFactory := registryStoreFactory
	registryStoreFactory = func() (*goplt.RegistryStore, error) { return store, nil }
	t.Cleanup(func() { registryStoreFactory = prevFactory })

	prevResolver := remoteResolverFn
	remoteResolverFn = func(_ context.Context, _ string) (string, string, error) {
		t.Fatal("resolver must not be called for empty registry")

		return "", "", nil
	}
	t.Cleanup(func() { remoteResolverFn = prevResolver })

	root := NewRootCmd()
	root.SetArgs([]string{"list", "--remote"})

	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)

	require.NoError(t, root.Execute())

	assert.Contains(t, errBuf.String(), "no registered collections")
	assert.Contains(t, errBuf.String(), "goplt registry add")
}

func TestRunList_remote_allRefreshesFailExitsNonZero(t *testing.T) {
	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{{Module: "github.com/a/one"}},
	}))

	prevFactory := registryStoreFactory
	registryStoreFactory = func() (*goplt.RegistryStore, error) { return store, nil }
	t.Cleanup(func() { registryStoreFactory = prevFactory })

	prevResolver := remoteResolverFn
	remoteResolverFn = func(_ context.Context, _ string) (string, string, error) {
		return "", "", errors.New("network down")
	}
	t.Cleanup(func() { remoteResolverFn = prevResolver })

	root := NewRootCmd()
	root.SetArgs([]string{"list", "--remote"})

	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 1 registered collections failed to refresh")
	assert.Contains(t, errBuf.String(), "WARN: failed to refresh github.com/a/one")
}

func TestRunList_remote_atLeastOneSuccessExitsZero(t *testing.T) {
	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{
			{Module: "github.com/a/one"},
			{Module: "github.com/b/two"},
		},
	}))

	prevFactory := registryStoreFactory
	registryStoreFactory = func() (*goplt.RegistryStore, error) { return store, nil }
	t.Cleanup(func() { registryStoreFactory = prevFactory })

	prevResolver := remoteResolverFn
	remoteResolverFn = func(_ context.Context, module string) (string, string, error) {
		if module == "github.com/b/two" {
			return "", "", errors.New("boom")
		}

		return "/cache/" + module, "v1.0.0", nil
	}
	t.Cleanup(func() { remoteResolverFn = prevResolver })

	root := NewRootCmd()
	root.SetArgs([]string{"list", "--remote"})

	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)

	require.NoError(t, root.Execute())
	assert.Contains(t, errBuf.String(), "refreshed github.com/a/one@v1.0.0")
	assert.Contains(t, errBuf.String(), "WARN: failed to refresh github.com/b/two")
}
