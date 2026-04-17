package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var templateDir, outputDir string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate files from a template directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runGenerate(templateDir, outputDir)
		},
	}

	wd, _ := os.Getwd()
	cmd.Flags().StringVar(&templateDir, "template", wd, "Template directory containing template.toml (default: current directory)")
	cmd.Flags().StringVar(&outputDir, "output", wd, "Output directory for generated files (default: current directory)")

	return cmd
}

func runGenerate(templateDir, outputDir string) error {
	// fully wired in Task 8
	return fmt.Errorf("not yet implemented")
}

// pathGuard returns an error if templateDir and outputDir are the same or nested.
// Both paths are resolved to canonical absolute paths before comparison.
func pathGuard(templateDir, outputDir string) error {
	a, err := canonicalPath(templateDir)
	if err != nil {
		return fmt.Errorf("resolve template dir: %w", err)
	}

	b, err := canonicalPath(outputDir)
	if err != nil {
		return fmt.Errorf("resolve output dir: %w", err)
	}

	sep := string(os.PathSeparator)

	if a == b || strings.HasPrefix(a, b+sep) || strings.HasPrefix(b, a+sep) {
		return fmt.Errorf("output dir %q must not overlap with template dir %q", outputDir, templateDir)
	}

	return nil
}

// canonicalPath returns the absolute, symlink-resolved path.
// If the path does not exist yet (output dir to be created), returns the absolute path.
func canonicalPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}

	return resolved, nil
}
