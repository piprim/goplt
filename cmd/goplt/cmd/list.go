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

	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List locally cached remote templates",
		RunE: func(c *cobra.Command, _ []string) error {
			return runList(c.Context())
		},
	}
}

func runList(ctx context.Context) error {
	gomodcache, err := getGoModCache(ctx)
	if err != nil {
		return err
	}

	templates, err := scanForTemplates(gomodcache)
	if err != nil {
		return err
	}

	printTemplates(templates)

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
func scanForTemplates(gomodcache string) (map[string][]string, error) {
	entries, err := os.ReadDir(gomodcache)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Safe to return empty; handled by print logic
		}

		return nil, fmt.Errorf("read GOMODCACHE: %w", err)
	}

	templates := make(map[string][]string)

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "cache" {
			continue
		}

		basePath := filepath.Join(gomodcache, entry.Name())
		_ = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || !d.IsDir() {
				return nil
			}

			if strings.Contains(d.Name(), "@") {
				extractTemplateData(gomodcache, path, templates)
				return fs.SkipDir
			}

			return nil
		})
	}

	return templates, nil
}

// extractTemplateData validates the template and appends it to the map.
func extractTemplateData(gomodcache, path string, templates map[string][]string) {
	tomlPath := filepath.Join(path, "goplt.toml")
	if _, err := os.Stat(tomlPath); err != nil {
		return // goplt.toml not found, skip
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
		modPath = modPathEscaped // Fallback
	}

	templates[modPath] = append(templates[modPath], version)
}

// printTemplates sorts and displays the discovered templates.
func printTemplates(templates map[string][]string) {
	if len(templates) == 0 {
		fmt.Println("No remote templates cached locally.")
		return
	}

	var modPaths []string
	for p := range templates {
		modPaths = append(modPaths, p)
	}

	slices.Sort(modPaths)

	for _, p := range modPaths {
		versions := templates[p]
		semver.Sort(versions)

		fmt.Printf("%s@latest\n", p)
		for i := len(versions) - 2; i >= 0; i-- {
			fmt.Printf("  %s@%s\n", p, versions[i])
		}
	}
}
