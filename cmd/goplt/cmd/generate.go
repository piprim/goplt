// cmd/goplt/cmd/generate.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
	"github.com/piprim/goplt"
	"github.com/spf13/cobra"
)

var successC = color.New(color.FgGreen)

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
	if err := pathGuard(templateDir, outputDir); err != nil {
		return err
	}

	fsys := os.DirFS(templateDir)

	m, err := goplt.LoadManifest(fsys)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	vars, err := collectVars(m)
	if err != nil {
		return fmt.Errorf("collect vars: %w", err)
	}

	if err := goplt.Generate(fsys, outputDir, vars); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	if err := goplt.RunHooks(m, outputDir); err != nil {
		return fmt.Errorf("post-generate hooks: %w", err)
	}

	_, _ = successC.Println("✓ Generation complete")

	return nil
}

// binding pairs a variable name with a function that writes the collected value into vars.
type binding struct {
	name  string
	apply func()
}

// collectVars builds and runs a huh form from the manifest variables,
// returning a PascalCase-keyed map of collected values.
func collectVars(m *goplt.Manifest) (map[string]any, error) {
	vars := make(map[string]any, len(m.Variables))

	for _, v := range m.Variables {
		vars[v.Name] = v.Default
	}

	var bindings []binding

	var fields []huh.Field

	for i := range m.Variables {
		f, b := buildField(m.Variables[i], vars)
		if f != nil {
			fields = append(fields, f)
			bindings = append(bindings, b)
		}
	}

	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return nil, fmt.Errorf("form cancelled: %w", err)
	}

	for _, b := range bindings {
		b.apply()
	}

	return vars, nil
}

// buildField constructs a huh.Field and its binding for a single manifest variable.
func buildField(v goplt.Variable, vars map[string]any) (huh.Field, binding) {
	name := v.Name

	switch v.Kind {
	case goplt.KindText:
		val := ""
		if s, ok := v.Default.(string); ok {
			val = s
		}

		ptr := &val
		field := huh.NewInput().
			Title(name).
			Value(ptr).
			Validate(func(s string) error {
				if def, _ := v.Default.(string); def == "" && s == "" {
					return fmt.Errorf("%s is required", name)
				}

				return nil
			})

		return field, binding{name: name, apply: func() { vars[name] = *ptr }}

	case goplt.KindBool:
		val := false
		if b, ok := v.Default.(bool); ok {
			val = b
		}

		ptr := &val
		field := huh.NewConfirm().Title(name).Value(ptr)

		return field, binding{name: name, apply: func() { vars[name] = *ptr }}

	case goplt.KindChoiceString:
		choices, _ := v.Default.([]string)
		opts := make([]huh.Option[string], len(choices))

		for j, c := range choices {
			opts[j] = huh.NewOption(c, c)
		}

		val := ""
		if len(choices) > 0 {
			val = choices[0]
		}

		ptr := &val
		field := huh.NewSelect[string]().Title(name).Options(opts...).Value(ptr)

		return field, binding{name: name, apply: func() { vars[name] = *ptr }}

	default:
		return nil, binding{}
	}
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
