// cmd/goplt/cmd/test.go
package cmd

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/piprim/goplt"
	"github.com/piprim/goplt/tui"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	var templateDir, image string
	var ask bool

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test a template by generating it and running go build + go test in a Docker sandbox",
		RunE: func(c *cobra.Command, _ []string) error {
			return runTest(c.Context(), templateDir, image, ask)
		},
	}

	wd, _ := os.Getwd()
	cmd.Flags().StringVarP(&templateDir, "template", "t", wd,
		"Template directory containing template.toml (default: current directory)")
	cmd.Flags().StringVar(&image, "image", "golang:latest",
		"Docker image to use for the sandbox")
	cmd.Flags().BoolVar(&ask, "ask", false,
		"Collect variable values interactively instead of using defaults")

	return cmd
}

func runTest(ctx context.Context, templateDir, image string, ask bool) error {
	// 1. Verify docker is available before doing any work.
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker not found — install Docker to use goplt test")
	}

	// 2. Resolve remote ref if needed.
	realTemplateDir := templateDir
	if isRemoteRef(templateDir) {
		resolved, err := resolveRemote(ctx, templateDir)
		if err != nil {
			return fmt.Errorf("resolve remote template %q: %w", templateDir, err)
		}
		realTemplateDir = resolved
	}

	// 3. Load manifest.
	fsys := os.DirFS(realTemplateDir)

	m, err := goplt.LoadManifest(fsys)
	if err != nil {
		return fmt.Errorf("load manifest in %q: %w", realTemplateDir, err)
	}

	// 4. Build vars.
	var vars map[string]any
	if ask {
		vars, err = tui.CollectVars(m)
		if err != nil {
			return fmt.Errorf("collect vars: %w", err)
		}
	} else {
		vars = buildDefaultVars(m.Variables)
	}

	// 5. Generate into a temp dir.
	tmpDir, err := os.MkdirTemp("", "goplt-test-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := goplt.Generate(fsys, m, tmpDir, vars); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	// 6. Build tar archive of generated output.
	tarData, err := buildTar(tmpDir)
	if err != nil {
		return fmt.Errorf("build tar: %w", err)
	}

	// 7. Run docker.
	script := buildScript([]string(m.Hooks.PostGenHooks))
	dockerCmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", image, "sh", "-c", script)
	dockerCmd.Stdin = bytes.NewReader(tarData)
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	if err := dockerCmd.Run(); err != nil {
		_, _ = color.New(color.FgRed).Println("✗ template failed")
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return &ExitCodeError{Code: exitErr.ExitCode()}
		}

		return fmt.Errorf("docker run: %w", err)
	}

	_, _ = successC.Println("✓ template ok")

	return nil
}

// buildDefaultVars builds a vars map from manifest variables using defaults.
// KindText variables with an empty default use the variable name as a placeholder.
func buildDefaultVars(variables []goplt.Variable) map[string]any {
	result := make(map[string]any, len(variables))

	for _, v := range variables {
		switch v.Kind {
		case goplt.KindText:
			if s, ok := v.Value.(string); ok && s != "" {
				result[v.Name] = s
			} else {
				result[v.Name] = v.Name
			}

		case goplt.KindBool:
			if b, ok := v.Value.(bool); ok {
				result[v.Name] = b
			} else {
				result[v.Name] = false
			}

		case goplt.KindStringChoice:
			choices, _ := v.Value.([]string)
			if len(choices) > 0 {
				result[v.Name] = choices[0]
			} else {
				result[v.Name] = ""
			}

		case goplt.KindStringList:
			if items, ok := v.Value.([]string); ok && len(items) > 0 {
				result[v.Name] = items
			} else {
				result[v.Name] = []string{v.Name}
			}
		}
	}

	return result
}

// buildScript assembles the sh -c script run inside the Docker container.
// It unpacks the tar archive from stdin, runs each hook, then builds and tests.
func buildScript(hooks []string) string {
	var sb strings.Builder
	//nolint:revive // strings.Builder.WriteString never returns an error
	sb.WriteString("set -e\nmkdir /work\ncd /work\ntar xf -\n")

	for _, h := range hooks {
		//nolint:revive // strings.Builder.WriteString never returns an error
		sb.WriteString(h)
		//nolint:revive // strings.Builder.WriteString never returns an error
		sb.WriteByte('\n')
	}

	//nolint:revive // strings.Builder.WriteString never returns an error
	sb.WriteString("go build ./...\ngo test ./...\n")

	return sb.String()
}

// buildTar creates an uncompressed tar archive of all files under dir.
func buildTar(dir string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// os.OpenRoot prevents symlink TOCTOU: all opens are scoped to dir and
	// will refuse to follow symlinks that escape it.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open root %q: %w", dir, err)
	}
	defer root.Close()

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relative path for %q: %w", path, err)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", path, err)
		}

		hdr := &tar.Header{
			Name:    filepath.ToSlash(rel),
			Mode:    int64(info.Mode()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write tar header for %q: %w", rel, err)
		}

		f, err := root.Open(filepath.ToSlash(rel))
		if err != nil {
			return fmt.Errorf("open %q: %w", rel, err)
		}
		defer f.Close()

		_, err = io.Copy(tw, f)

		if err != nil {
			return fmt.Errorf("failed to copy to tar writer: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk %q: %w", dir, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}

	return buf.Bytes(), nil
}
