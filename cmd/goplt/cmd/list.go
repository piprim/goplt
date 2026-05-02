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
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List locally cached remote templates",
		RunE: func(c *cobra.Command, _ []string) error {
			return runList(c.Context())
		},
	}

	return cmd
}

func runList(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "go", "env", "GOMODCACHE")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("get GOMODCACHE: %w", err)
	}

	gomodcache := strings.TrimSpace(string(out))
	if gomodcache == "" {
		return errors.New("GOMODCACHE is empty")
	}

	entries, err := os.ReadDir(gomodcache)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No remote templates cached locally.")

			return nil
		}

		return fmt.Errorf("read GOMODCACHE: %w", err)
	}

	templates := make(map[string][]string)

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "cache" {
			continue // Skip 'cache' folder
		}

		if err := filepath.WalkDir(filepath.Join(gomodcache, entry.Name()),
			func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return nil // Skip errors
				}

				if d.IsDir() {
					// The module cache stores module roots as `path/to/module@version`.
					// When we encounter a directory with '@', it is a module root.
					if strings.Contains(d.Name(), "@") {
						tomlPath := filepath.Join(path, "goplt.toml")
						if _, err := os.Stat(tomlPath); err == nil {
							rel, err := filepath.Rel(gomodcache, path)
							if err == nil {
								rel = filepath.ToSlash(rel)
								parts := strings.SplitN(rel, "@", 2)
								if len(parts) == 2 {
									modPathEscaped := parts[0]
									modPath, unescErr := module.UnescapePath(modPathEscaped)
									if unescErr != nil {
										modPath = modPathEscaped // Fallback
									}
									version := parts[1]
									templates[modPath] = append(templates[modPath], version)
								}
							}
						}
						// A valid remote template must have goplt.toml at the root.
						// We don't need to scan subdirectories of the module, so skip them.
						return fs.SkipDir
					}

					return nil
				}

				return nil
			}); err != nil {
			debugf("walk %s: %v", entry.Name(), err)
		}
	}

	if len(templates) == 0 {
		fmt.Println("No remote templates cached locally.")
		return nil
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

	return nil
}
