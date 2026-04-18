// generator.go
package goplt

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Generate walks fsys, renders each file with vars using Go text/template,
// and writes the output tree under outputDir.
// Paths conditioned out by the manifest are skipped.
func Generate(fsys fs.FS, m *Manifest, outputDir string, vars map[string]any) error {
	g := &generator{manifest: m, fsys: fsys, outputDir: outputDir, vars: vars}

	err := fs.WalkDir(fsys, ".", g.walk)
	if err != nil {
		return fmt.Errorf("failed to walks the file tree: %w", err)
	}

	return nil
}

type generator struct {
	manifest  *Manifest
	fsys      fs.FS
	outputDir string
	vars      map[string]any
}

func (g *generator) walk(path string, d fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}

	if path == "." || path == "template.toml" || path == "go.mod" || path == "go.sum" {
		return nil
	}

	skipped, err := g.isConditionedOut(path)
	if err != nil {
		return err
	}

	if skipped {
		if d.IsDir() {
			log.Println("Skipped directory: " + path)
			return fs.SkipDir
		}

		log.Println("Skipped file: " + path)

		return nil
	}

	if d.IsDir() {
		return nil
	}

	log.Println("Rendering path: " + path)
	outPath, err := renderString(strings.TrimSuffix(path, ".tmpl"), g.vars)
	if err != nil {
		return fmt.Errorf("render path %q: %w", path, err)
	}

	log.Println("Reading path: " + path)
	content, err := fs.ReadFile(g.fsys, path)
	if err != nil {
		return fmt.Errorf("read template %q: %w", path, err)
	}

	log.Println("Rendering content for file: " + path)
	rendered, err := renderBytes(path, content, g.vars)
	if err != nil {
		return fmt.Errorf("render content of %q: %w", path, err)
	}

	absPath := filepath.Join(g.outputDir, outPath)
	absDir := filepath.Dir(absPath)
	log.Println("Creating directory: " + absDir)

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("mkdir for %q: %w", absPath, err)
	}

	log.Println("Wrinting content in: " + absPath)
	if err := os.WriteFile(absPath, rendered, 0600); err != nil {
		return fmt.Errorf("write %q: %w", absPath, err)
	}

	return nil
}

func (g *generator) isConditionedOut(path string) (bool, error) {
	for prefix, expr := range g.manifest.Conditions {
		if !strings.HasPrefix(path, prefix) {
			log.Println("No rendering conditions for path: " + path)

			continue
		}

		log.Printf(`Rendering conditions "%s" for path: %s`+"\n", expr, path)
		result, err := renderString(expr, g.vars)
		if err != nil {
			return false, fmt.Errorf("evaluate condition for prefix %q: %w", prefix, err)
		}

		if result == "" {
			log.Println("Rendering condition is false")
			return true, nil
		}
	}

	log.Println("Rendering condition is true")

	return false, nil
}

func renderString(tmplStr string, data any) (string, error) {
	t, err := template.New("").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", tmplStr, err)
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", tmplStr, err)
	}

	return buf.String(), nil
}

func renderBytes(name string, content []byte, data any) ([]byte, error) {
	t, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %q: %w", name, err)
	}

	return buf.Bytes(), nil
}
