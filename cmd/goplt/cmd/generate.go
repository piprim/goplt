// cmd/goplt/cmd/generate.go
package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/fatih/color"
	"github.com/piprim/goplt"
	"github.com/piprim/goplt/tui"
	"github.com/spf13/cobra"
)

var (
	successC = color.New(color.FgGreen)
	warnC    = color.New(color.FgYellow, color.Bold)
)

func newGenerateCmd() *cobra.Command {
	var templateDir, outputDir string
	var yes bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate files from a template directory",
		RunE: func(c *cobra.Command, _ []string) error {
			return runGenerate(templateDir, outputDir, yes, c.Flags().Changed("output"))
		},
	}

	wd, _ := os.Getwd()
	cmd.Flags().StringVarP(
		&templateDir, "template", "t", wd, "Template directory containing template.toml (default: current directory)")
	cmd.Flags().StringVarP(
		&outputDir, "output", "o", wd, "Output directory for generated files (default: current directory)")
	cmd.Flags().BoolVarP(
		&yes, "yes", "y", false, "Skip hook confirmation prompt (for CI / trusted templates)")

	return cmd
}

func runGenerate(templateDir, outputDir string, yes, outputExplicit bool) error {
	realTemplateDir := templateDir

	if isRemoteRef(templateDir) {
		resolved, err := resolveRemote(templateDir)
		if err != nil {
			return fmt.Errorf("resolve remote template %q: %w", templateDir, err)
		}
		realTemplateDir = resolved
	}

	if err := pathGuard(realTemplateDir, outputDir); err != nil {
		return err
	}

	fsys := os.DirFS(realTemplateDir)

	debugf("loading manifest from %s", realTemplateDir)
	m, err := goplt.LoadManifest(fsys)
	if err != nil {
		return fmt.Errorf(`load manifest in "%s": %w`, realTemplateDir, err)
	}

	vars, err := tui.CollectVars(m)
	if err != nil {
		return fmt.Errorf("collect vars: %w", err)
	}

	realOutputDir, err := applyTargetDir(m.TargetDir, outputDir, vars, outputExplicit)
	if err != nil {
		return fmt.Errorf("apply target-dir: %w", err)
	}

	debugf("generating project to %s", realOutputDir)
	if err := goplt.Generate(fsys, m, realOutputDir, vars); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	if err := confirmAndRunHooks(m, realOutputDir, yes); err != nil {
		return err
	}

	_, _ = successC.Println("✓ Generation completed in " + realOutputDir)

	return nil
}

// applyTargetDir renders targetDirTmpl against vars and appends the result to
// outputDir. It is a no-op when outputExplicit is true (--output was set by the
// caller) or when targetDirTmpl is empty.
func applyTargetDir(targetDirTmpl, outputDir string, vars map[string]any, outputExplicit bool) (string, error) {
	if outputExplicit || targetDirTmpl == "" {
		return outputDir, nil
	}

	t, err := template.New("target-dir").Parse(targetDirTmpl)
	if err != nil {
		return "", fmt.Errorf("parse target-dir template %q: %w", targetDirTmpl, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute target-dir template %q: %w", targetDirTmpl, err)
	}

	rendered := buf.String()
	if rendered == "" {
		return outputDir, nil
	}

	joined := filepath.Join(outputDir, rendered)
	if !strings.HasPrefix(joined+string(os.PathSeparator), outputDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("target-dir %q escapes the output directory", rendered)
	}

	return joined, nil
}

// confirmAndRunHooks shows a security warning and asks for explicit consent before
// running post-generate hooks. If --yes is set the prompt is skipped entirely.
// When no hooks are defined the function is a no-op.
func confirmAndRunHooks(m *goplt.Manifest, outputDir string, yes bool) error {
	if len(m.Hooks.PostGenHooks) == 0 {
		return nil
	}

	if !yes {
		msg := "This template defines post-generate hooks that will run shell commands on your machine."
		_, _ = warnC.Println("\n⚠  WARNING ⚠\n" + msg)
		fmt.Println("   Only proceed if you trust the template source.")
		fmt.Println()
		fmt.Println("   Commands that will be executed:")
		for _, h := range m.Hooks.PostGenHooks {
			fmt.Printf("     • %s\n", h)
		}
		fmt.Println()
		fmt.Println("   Tip: to skip this prompt in CI or for trusted templates, re-run with:")
		fmt.Println("     goplt generate --yes ...")
		fmt.Println()

		_, _ = fmt.Print("Run these hooks? [y/N]: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading hook confirmation: %w", err)
			}
			debugf("hooks skipped")

			return nil
		}
		if !strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
			debugf("hooks skipped")
			return nil
		}
		fmt.Println()
	}

	if err := goplt.RunHooks(m, outputDir); err != nil {
		return fmt.Errorf("post-generate hooks: %w", err)
	}

	return nil
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
		return "", fmt.Errorf(`failed to retrieve absolute representation of path "%s": %w`, p, err)
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}

	return resolved, nil
}
