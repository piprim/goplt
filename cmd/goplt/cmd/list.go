// cmd/goplt/cmd/list.go
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/piprim/goplt"
	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

type templateEntry struct {
	modPath         string
	versions        []string
	tags            []string
	authors         []string
	metadataVersion string // semver of the version whose manifest was loaded
}

func newListCmd() *cobra.Command {
	var tags []string
	var author string
	var orMode bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List locally cached remote templates",
		RunE: func(c *cobra.Command, _ []string) error {
			return runList(c.Context(), tags, author, orMode)
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Filter by tag (repeatable; AND by default)")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author (case-insensitive substring)")
	cmd.Flags().BoolVar(&orMode, "or", false, "Match any tag instead of all tags (OR mode)")

	return cmd
}

func runList(ctx context.Context, tags []string, author string, orMode bool) error {
	gomodcache, err := getGoModCache(ctx)
	if err != nil {
		return err
	}

	entries, err := scanForTemplates(gomodcache)
	if err != nil {
		return err
	}

	entries = filterTemplates(entries, tags, author, orMode)

	printTemplates(entries)

	return nil
}

// getGoModCache retrieves the GOMODCACHE environment variable path.
func getGoModCache(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "env", "GOMODCACHE")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get GOMODCACHE: %w", err)
	}

	gomodcache := strings.TrimSpace(string(out))
	if gomodcache == "" {
		return "", errors.New("GOMODCACHE is empty")
	}

	return gomodcache, nil
}

// scanForTemplates searches the module cache for valid remote templates.
func scanForTemplates(gomodcache string) ([]templateEntry, error) {
	dirEntries, err := os.ReadDir(gomodcache)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read GOMODCACHE: %w", err)
	}

	byPath := make(map[string]*templateEntry)

	for _, entry := range dirEntries {
		if !entry.IsDir() || entry.Name() == "cache" {
			continue
		}

		basePath := filepath.Join(gomodcache, entry.Name())
		_ = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || !d.IsDir() {
				return nil
			}

			if strings.Contains(d.Name(), "@") {
				extractTemplateData(gomodcache, path, byPath)

				return fs.SkipDir
			}

			return nil
		})
	}

	result := make([]templateEntry, 0, len(byPath))
	for _, e := range byPath {
		result = append(result, *e)
	}

	return result, nil
}

// extractTemplateData loads the manifest and registers the version into byPath.
// Silently skips paths without a valid goplt.toml.
func extractTemplateData(gomodcache, path string, byPath map[string]*templateEntry) {
	tomlPath := filepath.Join(path, "goplt.toml")
	if _, err := os.Stat(tomlPath); err != nil {
		return
	}

	rel, err := filepath.Rel(gomodcache, path)
	if err != nil {
		return
	}

	rel = filepath.ToSlash(rel)
	modPathEscaped, version, found := strings.Cut(rel, "@")
	if !found {
		return
	}

	modPath, unescErr := module.UnescapePath(modPathEscaped)
	if unescErr != nil {
		modPath = modPathEscaped
	}

	e, exists := byPath[modPath]
	isNewer := exists && semver.Compare(version, e.metadataVersion) > 0

	if exists && !isNewer {
		e.versions = append(e.versions, version)

		return
	}

	m, loadErr := goplt.LoadManifest(os.DirFS(path))
	if loadErr != nil {
		if exists {
			e.versions = append(e.versions, version)
		}

		return
	}

	if !exists {
		e = &templateEntry{modPath: modPath}
		byPath[modPath] = e
	}

	e.tags = m.Tags
	e.authors = m.Authors
	e.metadataVersion = version
	e.versions = append(e.versions, version)
}

func filterTemplates(entries []templateEntry, tags []string, author string, orMode bool) []templateEntry {
	if len(tags) == 0 && author == "" {
		return entries
	}

	out := make([]templateEntry, 0, len(entries))

	for _, e := range entries {
		if !matchesTags(e.tags, tags, orMode) {
			continue
		}

		if !matchesAuthor(e.authors, author) {
			continue
		}

		out = append(out, e)
	}

	return out
}

func matchesTags(have, want []string, orMode bool) bool {
	if len(want) == 0 {
		return true
	}

	if orMode {
		for _, w := range want {
			if slices.Contains(have, w) {
				return true
			}
		}

		return false
	}

	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}

	return true
}

func matchesAuthor(authors []string, want string) bool {
	if want == "" {
		return true
	}

	wantLower := strings.ToLower(want)

	for _, a := range authors {
		if strings.Contains(strings.ToLower(a), wantLower) {
			return true
		}
	}

	return false
}

// printTemplates sorts and displays the discovered templates with metadata.
func printTemplates(entries []templateEntry) {
	if len(entries) == 0 {
		fmt.Println("No remote templates cached locally.")

		return
	}

	slices.SortFunc(entries, func(a, b templateEntry) int {
		return strings.Compare(a.modPath, b.modPath)
	})

	for _, e := range entries {
		semver.Sort(e.versions)
		meta := formatMeta(e.tags, e.authors)
		fmt.Printf("%s@latest%s\n", e.modPath, meta)

		for i := len(e.versions) - 2; i >= 0; i-- {
			fmt.Printf("  %s@%s%s\n", e.modPath, e.versions[i], meta)
		}
	}
}

func formatMeta(tags, authors []string) string {
	var parts []string

	if len(tags) > 0 {
		parts = append(parts, "["+strings.Join(tags, ", ")+"]")
	}

	if len(authors) > 0 {
		parts = append(parts, "("+strings.Join(authors, ", ")+")")
	}

	if len(parts) == 0 {
		return ""
	}

	return "  " + strings.Join(parts, "  ")
}
