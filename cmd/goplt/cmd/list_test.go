// cmd/goplt/cmd/list_test.go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
