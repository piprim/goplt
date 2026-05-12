// cmd/goplt/cmd/list.go
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	var (
		tags   []string
		author string
		orMode bool
		remote bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List locally cached remote templates",
		RunE: func(c *cobra.Command, _ []string) error {
			return runList(c, tags, author, orMode, remote)
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Filter by tag (repeatable; AND by default)")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author (case-insensitive substring)")
	cmd.Flags().BoolVar(&orMode, "or", false, "Match any tag instead of all tags (OR mode)")
	cmd.Flags().BoolVar(&remote, "remote", false, "Refresh registered collections before listing")

	return cmd
}

// remoteResolverFn is the injectable seam used by --remote. Tests replace it.
var remoteResolverFn remoteResolver = defaultRemoteResolver

func runList(c *cobra.Command, tags []string, author string, orMode, remote bool) error {
	ctx := c.Context()
	stderr := c.ErrOrStderr()

	if remote {
		if err := doRemoteRefresh(ctx, stderr); err != nil {
			return err
		}
	}

	gomodcache, err := getGoModCache(ctx)
	if err != nil {
		return err
	}

	entries, err := scanForTemplates(gomodcache)
	if err != nil {
		return err
	}

	entries = filterTemplates(entries, tags, author, orMode)

	printTemplatesTo(c.OutOrStdout(), entries)

	return nil
}

// doRemoteRefresh loads the registry and refreshes every entry. Empty registry
// is informational (hint to stderr, no error). Total failure is fatal.
func doRemoteRefresh(ctx context.Context, stderr io.Writer) error {
	store, err := registryStoreFactory()
	if err != nil {
		return fmt.Errorf("registry store: %w", err)
	}

	r, err := store.Load()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	if len(r.Collections) == 0 {
		fmt.Fprintln(stderr, "no registered collections; use 'goplt registry add' to register one")

		return nil
	}

	refreshed, failed, err := refreshRegistry(ctx, store, remoteResolverFn, stderr)
	if err != nil {
		return err
	}

	if refreshed == 0 && failed > 0 {
		return fmt.Errorf("all %d registered collections failed to refresh", failed)
	}

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

// printTemplatesTo sorts and writes the discovered templates with metadata to w.
func printTemplatesTo(w io.Writer, entries []templateEntry) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "No remote templates cached locally.")

		return
	}

	slices.SortFunc(entries, func(a, b templateEntry) int {
		return strings.Compare(a.modPath, b.modPath)
	})

	for _, e := range entries {
		semver.Sort(e.versions)
		meta := formatMeta(e.tags, e.authors)
		fmt.Fprintf(w, "%s@latest%s\n", e.modPath, meta)

		for i := len(e.versions) - 2; i >= 0; i-- {
			fmt.Fprintf(w, "  %s@%s%s\n", e.modPath, e.versions[i], meta)
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

// remoteResolver fetches a Go module at @latest and returns the cache dir and
// the resolved version. It is the seam used by refreshRegistry; production
// callers pass defaultRemoteResolver, tests pass a stub.
type remoteResolver func(ctx context.Context, mod string) (dir, version string, err error)

// defaultRemoteResolver wraps resolveRemote, always requesting @latest, and
// discards the cache dir — refreshRegistry only cares that the download
// succeeded and what version was pulled.
func defaultRemoteResolver(ctx context.Context, mod string) (dir, version string, err error) {
	return resolveRemote(ctx, mod+"@latest")
}

// refreshRegistry loads the registry from store and calls resolve for every
// entry in file order. For each entry it writes either
//
//	"refreshed <module>@<version>"   (success)
//
// or
//
//	"WARN: failed to refresh <module>: <reason>"   (failure)
//
// to stderr. Per-entry failures do not abort the loop. Returns counts of
// successes and failures; err is non-nil only when the registry itself cannot
// be loaded.
func refreshRegistry(
	ctx context.Context,
	store *goplt.RegistryStore,
	resolve remoteResolver,
	stderr io.Writer,
) (refreshed, failed int, err error) {
	r, err := store.Load()
	if err != nil {
		return 0, 0, fmt.Errorf("load registry: %w", err)
	}

	for _, e := range r.Collections {
		_, version, resErr := resolve(ctx, e.Module)
		if resErr != nil {
			fmt.Fprintf(stderr, "WARN: failed to refresh %s: %s\n", e.Module, resErr)
			failed++

			continue
		}

		fmt.Fprintf(stderr, "refreshed %s@%s\n", e.Module, version)
		refreshed++
	}

	return refreshed, failed, nil
}
