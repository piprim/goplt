// cmd/goplt/cmd/registry_test.go
package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTempRegistry replaces registryStoreFactory with one pointed at t.TempDir()
// and restores the previous factory at the end of the test.
func withTempRegistry(t *testing.T) *goplt.RegistryStore {
	t.Helper()

	store := goplt.NewRegistryStore(filepath.Join(t.TempDir(), "registry.toml"))
	prev := registryStoreFactory
	registryStoreFactory = func() (*goplt.RegistryStore, error) { return store, nil }

	t.Cleanup(func() { registryStoreFactory = prev })

	return store
}

// withStubResolver replaces remoteResolverFn with resolve and restores the
// previous value at the end of the test. Used by registry-add tests so they
// do not hit the network.
func withStubResolver(t *testing.T, resolve remoteResolver) {
	t.Helper()

	prev := remoteResolverFn
	remoteResolverFn = resolve

	t.Cleanup(func() { remoteResolverFn = prev })
}

// stubResolverOK is the default success stub used by registry-add tests that
// only care about the persistence path.
func stubResolverOK(_ context.Context, module string) (string, string, error) {
	return "/cache/" + module, "v1.0.0", nil
}

func TestRegistryAdd_writesEntry(t *testing.T) {
	store := withTempRegistry(t)

	var calledWith string
	withStubResolver(t, func(_ context.Context, module string) (string, string, error) {
		calledWith = module

		return "/cache/" + module, "v1.0.0", nil
	})

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "add", "github.com/foo/bar"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	require.NoError(t, root.Execute())

	assert.Equal(t, "github.com/foo/bar", calledWith,
		"resolver must be called with the module path before it is persisted")

	r, err := store.Load()
	require.NoError(t, err)
	require.Len(t, r.Collections, 1)
	assert.Equal(t, "github.com/foo/bar", r.Collections[0].Module)
}

func TestRegistryAdd_rejectsDuplicate(t *testing.T) {
	store := withTempRegistry(t)
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{{Module: "github.com/foo/bar"}},
	}))

	withStubResolver(t, stubResolverOK)

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "add", "github.com/foo/bar"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistryAdd_rejectsUnresolvableModule(t *testing.T) {
	store := withTempRegistry(t)

	withStubResolver(t, func(_ context.Context, _ string) (string, string, error) {
		return "", "", errors.New("module does not exist")
	})

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "add", "github.com/totally/fake-nonexistent"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve module")
	assert.Contains(t, err.Error(), "github.com/totally/fake-nonexistent")

	r, err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, r.Collections, "module must NOT be persisted when resolution fails")
}

func TestRegistryAdd_rejectsInvalidFormatBeforeResolving(t *testing.T) {
	store := withTempRegistry(t)

	resolverCalled := false
	withStubResolver(t, func(_ context.Context, _ string) (string, string, error) {
		resolverCalled = true

		return "/cache/x", "v1.0.0", nil
	})

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "add", "./local-path"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate module")
	assert.False(t, resolverCalled, "resolver must not be called when format validation fails")

	r, err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, r.Collections)
}

func TestRegistryRemove_removesEntry(t *testing.T) {
	store := withTempRegistry(t)
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{{Module: "github.com/foo/bar"}},
	}))

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "remove", "github.com/foo/bar"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	require.NoError(t, root.Execute())

	r, err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, r.Collections)
}

func TestRegistryRemove_rejectsAbsent(t *testing.T) {
	_ = withTempRegistry(t)

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "remove", "github.com/foo/bar"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestRegistryList_printsEntriesInOrder(t *testing.T) {
	store := withTempRegistry(t)
	require.NoError(t, store.Save(goplt.Registry{
		Collections: []goplt.RegistryEntry{
			{Module: "github.com/a/one"},
			{Module: "github.com/b/two"},
		},
	}))

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "list"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	require.NoError(t, root.Execute())

	got := out.String()
	assert.Equal(t, "github.com/a/one\ngithub.com/b/two\n", got)
}

func TestRegistryList_emptyPrintsNothing(t *testing.T) {
	_ = withTempRegistry(t)

	root := NewRootCmd()
	root.SetArgs([]string{"registry", "list"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	require.NoError(t, root.Execute())

	assert.Empty(t, out.String())
}
