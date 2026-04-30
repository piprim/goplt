package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/piprim/goplt"
	"github.com/spf13/cobra"
)

const (
	tmplDirPerm  = 0o755
	tmplFilePerm = 0o600
)

func newTemplatizeCmd() *cobra.Command {
	var outputDir, sourceDir, name, orgPrefix, description string
	var skip []string
	var yes bool

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	cmd := &cobra.Command{
		Use:   "templatize",
		Short: "Convert an existing Go project into a goplt template",
		RunE: func(c *cobra.Command, _ []string) error {
			return runTemplatize(c, sourceDir, outputDir, name, orgPrefix, description, skip, yes)
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for the template (required)")
	cmd.Flags().StringVarP(&sourceDir, "dir", "d", wd, "Source directory (default: current directory)")
	cmd.Flags().StringVar(&name, "name", "", "Override project name")
	cmd.Flags().StringVar(&orgPrefix, "org-prefix", "", "Override org prefix")
	cmd.Flags().StringArrayVar(&skip, "skip", nil, "Protect a string from substitution; repeatable")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, `Pre-answer all substitution yes/no questions as "yes"`)
	cmd.Flags().StringVar(&description, "description", "", "Template description (required with --yes)")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

func runTemplatize(
	cmd *cobra.Command,
	sourceDir, outputDir, name, orgPrefix, description string,
	skip []string,
	yes bool,
) error {
	if entries, err := os.ReadDir(outputDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("templatize: output dir %q already exists and is not empty", outputDir)
	}

	if yes && description == "" {
		return fmt.Errorf("templatize: --description is required when --yes is set")
	}

	name, orgPrefix, err := resolveNameAndPrefix(sourceDir, name, orgPrefix)
	if err != nil {
		return err
	}

	for _, v := range []string{name, orgPrefix} {
		if strings.Contains(v, "{{") {
			return fmt.Errorf("templatize: value %q looks like a template placeholder; run goplt generate first", v)
		}
	}

	description, name, orgPrefix, confirmedSubs, err := collectTemplatizeVars(name, orgPrefix, description, skip, yes)
	if err != nil {
		return fmt.Errorf("templatize: collect vars: %w", err)
	}

	subs := goplt.BuildSubstitutions(name, orgPrefix, skip)

	filtered := make([]goplt.Substitution, 0, len(subs))
	for _, s := range subs {
		if s.Value == s.Placeholder {
			filtered = append(filtered, s)

			continue
		}
		if confirmedSubs[s.Value] {
			filtered = append(filtered, s)
		}
	}

	fsys := os.DirFS(sourceDir)
	report, err := goplt.Templatize(fsys, outputDir, filtered)
	if err != nil {
		return fmt.Errorf("templatize: %w", err)
	}

	if err := writeTemplateTOML(outputDir, description, name, orgPrefix); err != nil {
		return err
	}

	printTemplatizeReport(cmd, report, outputDir)

	return nil
}

// resolveNameAndPrefix fills in name and orgPrefix from the module path in
// go.mod when either is missing. Returns an error only when go.mod cannot be
// read and at least one value is still empty after the attempt.
func resolveNameAndPrefix(sourceDir, name, orgPrefix string) (string, string, error) {
	if name != "" && orgPrefix != "" {
		return name, orgPrefix, nil
	}

	modulePath, err := goplt.ReadModulePath(sourceDir)
	if err != nil {
		// go.mod unreadable — only fatal when a value is still missing.
		if name == "" || orgPrefix == "" {
			return "", "", fmt.Errorf(
				"templatize: no go.mod found in %q; point --dir at a specific module or provide --name and --org-prefix",
				sourceDir,
			)
		}

		return name, orgPrefix, nil
	}

	if name == "" {
		name = modulePath[strings.LastIndex(modulePath, "/")+1:]
	}

	if orgPrefix == "" {
		if idx := strings.LastIndex(modulePath, "/"); idx >= 0 {
			orgPrefix = modulePath[:idx]
		}
	}

	return name, orgPrefix, nil
}

//nolint:funlen // TUI form builds fields dynamically; splitting would hide the single-form assembly logic
func collectTemplatizeVars(
	name, orgPrefix, description string,
	skip []string,
	yes bool,
) (string, string, string, map[string]bool, error) {
	skipSet := make(map[string]struct{}, len(skip))
	for _, s := range skip {
		skipSet[s] = struct{}{}
	}

	type formEntry struct {
		value       string
		placeholder string
		confirmed   bool
	}

	// Derive case forms in the same order as BuildSubstitutions for display.
	entries := []formEntry{
		{value: goplt.KebabCase(name), placeholder: "{{.Name}}"},
		{value: goplt.PascalCase(name), placeholder: "{{.Name | pascal}}"},
		{value: goplt.CamelCase(name), placeholder: "{{.Name | camel}}"},
		{value: goplt.SnakeCase(name), placeholder: "{{.Name | snake}}"},
		{value: orgPrefix, placeholder: "{{.OrgPrefix}}"},
	}

	// Deduplicate by value; blank values dropped.
	seen := map[string]struct{}{}
	deduped := make([]formEntry, 0, len(entries))

	for _, e := range entries {
		if e.value == "" {
			continue
		}
		if _, ok := seen[e.value]; ok {
			continue
		}

		seen[e.value] = struct{}{}
		_, inSkip := skipSet[e.value]
		e.confirmed = !inSkip
		deduped = append(deduped, e)
	}

	if !yes {
		fields := []huh.Field{
			huh.NewInput().Title("Description").Value(&description).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("templatize: description is required")
					}

					return nil
				}),
			huh.NewInput().Title("Name").Value(&name),
			huh.NewInput().Title("Org Prefix").Value(&orgPrefix),
		}

		for i := range deduped {
			e := &deduped[i]
			label := fmt.Sprintf("Substitute %q → %s?", e.value, e.placeholder)
			fields = append(fields, huh.NewConfirm().Title(label).Value(&e.confirmed))
		}

		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#33DD33")).Padding(1).Align(lipgloss.Center)
		group := huh.NewGroup(fields...).
			WithTheme(huh.ThemeDracula()).
			Description("\n" + style.Render("Convert project to goplt template"))

		if err := huh.NewForm(group).Run(); err != nil {
			return "", "", "", nil, fmt.Errorf("templatize: tui form: %w", err)
		}
	}

	confirmed := make(map[string]bool, len(deduped))
	for _, e := range deduped {
		confirmed[e.value] = e.confirmed
	}

	return description, name, orgPrefix, confirmed, nil
}

func writeTemplateTOML(outputDir, description, name, orgPrefix string) error {
	content := fmt.Sprintf(`description = %q

[variables.name]
kind        = "input"
value       = %q
description = "Project name in kebab-case"
required    = true

[variables.org-prefix]
kind        = "input"
value       = %q
description = "Go module path prefix"
required    = true

[hooks]
post-generate = ["go mod tidy"]
`, description, name, orgPrefix)

	if err := os.MkdirAll(outputDir, tmplDirPerm); err != nil {
		return fmt.Errorf("templatize: mkdir output: %w", err)
	}

	path := filepath.Join(outputDir, "template.toml")

	if err := os.WriteFile(path, []byte(content), tmplFilePerm); err != nil {
		return fmt.Errorf("templatize: write template.toml: %w", err)
	}

	return nil
}

func printTemplatizeReport(cmd *cobra.Command, report *goplt.TemplatizeReport, outputDir string) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "\nSubstitution report:")

	for _, r := range report.Results {
		fmt.Fprintf(out, "  %-30s → %-30s %d occurrence(s) in %d file(s)\n",
			fmt.Sprintf("%q", r.From), r.To, r.Count, len(r.Files))
	}

	if len(report.Skipped) > 0 {
		fmt.Fprintln(out, "\nProtected:")

		for value, count := range report.Skipped {
			fmt.Fprintf(out, "  %-30s %d occurrence(s) preserved\n", fmt.Sprintf("%q", value), count)
		}
	}

	if len(report.BinaryFiles) > 0 {
		fmt.Fprintf(out, "\nBinary files copied verbatim: %d\n", len(report.BinaryFiles))
	}

	_, _ = successC.Fprintln(out, "\n✓ Template written to "+outputDir)
}
