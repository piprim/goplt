// cmd/goplt/cmd/init.go
package cmd

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/piprim/goplt"
	"github.com/piprim/goplt/cmd/goplt/inittempl"
	"github.com/piprim/goplt/tui"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var outputDir, domain, complexity string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new goplt template directory",
		RunE: func(c *cobra.Command, _ []string) error {
			return runInit(outputDir, domain, complexity, c.Flags().Changed("output"))
		},
	}

	wd, _ := os.Getwd()
	cmd.Flags().StringVarP(&outputDir, "output", "o", wd,
		"Destination directory for the scaffolded template (default: current directory)")
	cmd.Flags().StringVarP(&domain, "domain", "d", "go-library-simple",
		"Template domain to scaffold")
	cmd.Flags().StringVarP(&complexity, "complexity", "c", "",
		"Pre-set complexity: minimal, standard, or advanced")

	return cmd
}

func runInit(outputDir, domain, complexity string, outputExplicit bool) error {
	subFS, _ := fs.Sub(inittempl.FS, "templates/"+domain)

	m, err := goplt.LoadManifest(subFS)
	if err != nil {
		if _, statErr := fs.Stat(subFS, "template.toml"); statErr != nil {
			return fmt.Errorf("domain %q not found", domain)
		}
		return fmt.Errorf("load meta-manifest: %w", err)
	}

	if complexity != "" {
		for i := range m.Variables {
			if m.Variables[i].Name == "Complexity" {
				reordered := []string{complexity}
				for _, c := range []string{"minimal", "standard", "advanced"} {
					if c != complexity {
						reordered = append(reordered, c)
					}
				}
				m.Variables[i].Default = reordered
				break
			}
		}
	}

	vars, err := tui.CollectVars(m)
	if err != nil {
		return fmt.Errorf("collect vars: %w", err)
	}

	realOutputDir, err := applyTargetDir(m.TargetDir, outputDir, vars, outputExplicit, m.Delimiters)
	if err != nil {
		return fmt.Errorf("apply target-dir: %w", err)
	}

	debugf("scaffolding template to %s", realOutputDir)
	if err := goplt.Generate(subFS, m, realOutputDir, vars); err != nil {
		return fmt.Errorf("scaffold template: %w", err)
	}

	_, _ = successC.Println("✓ Template scaffolded in " + realOutputDir)
	return nil
}
