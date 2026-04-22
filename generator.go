// generator.go
package goplt

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Generator renders a template tree into an output directory.
// Construct one with NewGenerator; customise with WithFuncs.
type Generator struct {
	funcs template.FuncMap
}

// NewGenerator returns a Generator pre-loaded with the built-in function map.
func NewGenerator() *Generator {
	return &Generator{funcs: DefaultFuncMap()}
}

// WithFuncs returns a new Generator with additional functions merged in.
// Caller-supplied functions override built-ins with the same name.
func (g *Generator) WithFuncs(fm template.FuncMap) *Generator {
	merged := maps.Clone(g.funcs)
	for k, v := range fm {
		merged[k] = v
	}
	return &Generator{funcs: merged}
}

// Generate walks fsys, renders each file with vars using Go text/template,
// and writes the output tree under outputDir. Paths conditioned out by the
// manifest are skipped. Template functions registered via WithFuncs are
// available in every template.
func (g *Generator) Generate(fsys fs.FS, m *Manifest, outputDir string, vars map[string]any) error {
	gen := &internalGenerator{
		manifest:  m,
		fsys:      fsys,
		outputDir: outputDir,
		vars:      vars,
		funcs:     g.funcs,
	}

	if err := fs.WalkDir(fsys, ".", gen.walk); err != nil {
		return fmt.Errorf("failed to walk the file tree: %w", err)
	}

	return nil
}

// Generate walks fsys, renders each file with vars using Go text/template,
// and writes the output tree under outputDir.
// It is equivalent to NewGenerator().Generate(...) and is kept for backwards compatibility.
func Generate(fsys fs.FS, m *Manifest, outputDir string, vars map[string]any) error {
	return NewGenerator().Generate(fsys, m, outputDir, vars)
}

// internalGenerator holds the per-call state for a generation run.
type internalGenerator struct {
	manifest  *Manifest
	fsys      fs.FS
	outputDir string
	vars      map[string]any
	funcs     template.FuncMap
}

func (g *internalGenerator) walk(path string, d fs.DirEntry, walkErr error) error {
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
			return fs.SkipDir
		}

		return nil
	}

	if d.IsDir() {
		return nil
	}

	outPath, err := g.renderString(strings.TrimSuffix(path, ".tmpl"), g.vars)
	if err != nil {
		return fmt.Errorf("render path %q: %w", path, err)
	}

	content, err := fs.ReadFile(g.fsys, path)
	if err != nil {
		return fmt.Errorf("read template %q: %w", path, err)
	}

	rendered, err := g.renderBytes(path, content, g.vars)
	if err != nil {
		return fmt.Errorf("render content of %q: %w", path, err)
	}

	absPath := filepath.Join(g.outputDir, outPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("mkdir for %q: %w", absPath, err)
	}

	if err := os.WriteFile(absPath, rendered, 0600); err != nil {
		return fmt.Errorf("write %q: %w", absPath, err)
	}

	return nil
}

func (g *internalGenerator) isConditionedOut(path string) (bool, error) {
	for prefix, expr := range g.manifest.Conditions {
		if !strings.HasPrefix(path, prefix) {
			continue
		}

		result, err := g.renderString(expr, g.vars)
		if err != nil {
			return false, fmt.Errorf("evaluate condition for prefix %q: %w", prefix, err)
		}

		if result == "" {
			return true, nil
		}
	}

	return false, nil
}

func (g *internalGenerator) renderString(tmplStr string, data any) (string, error) {
	t, err := template.New("").Funcs(g.funcs).Delims(g.manifest.Delimiters[0], g.manifest.Delimiters[1]).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", tmplStr, err)
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", tmplStr, err)
	}

	return buf.String(), nil
}

func (g *internalGenerator) renderBytes(name string, content []byte, data any) ([]byte, error) {
	t, err := template.New(name).Funcs(g.funcs).Delims(g.manifest.Delimiters[0], g.manifest.Delimiters[1]).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %q: %w", name, err)
	}

	return buf.Bytes(), nil
}
