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

// loopEntry represents one (item, paths) pair produced by expanding a [loops] entry.
type loopEntry struct {
	SourcePrefix   string // literal FS path prefix, e.g. "internal/{{.item}}/"
	Item           string // the list item value, e.g. "auth"
	ExpandedPrefix string // rendered output prefix, e.g. "internal/auth/"
}

// NewGenerator returns a Generator pre-loaded with the built-in function map.
func NewGenerator() *Generator {
	return &Generator{funcs: DefaultFuncMap()}
}

// WithFuncs returns a new Generator with additional functions merged in.
// Caller-supplied functions override built-ins with the same name.
func (g *Generator) WithFuncs(fm template.FuncMap) *Generator {
	merged := maps.Clone(g.funcs)
	maps.Copy(merged, fm)

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

	loopEntries, err := gen.expandLoops()
	if err != nil {
		return fmt.Errorf("expand loops: %w", err)
	}
	gen.loopEntries = loopEntries

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
	manifest    *Manifest
	fsys        fs.FS
	outputDir   string
	vars        map[string]any
	funcs       template.FuncMap
	loopEntries []loopEntry
}

// varsWithItem returns a clone of g.vars with "item" set to the given value.
func (g *internalGenerator) varsWithItem(item string) map[string]any {
	m := make(map[string]any, len(g.vars)+1)
	maps.Copy(m, g.vars)
	m["item"] = item

	return m
}

// expandLoops builds the full list of loopEntry values for this generation run.
func (g *internalGenerator) expandLoops() ([]loopEntry, error) {
	var entries []loopEntry
	for pattern, varNames := range g.manifest.Loops {
		varName := varNames[0] // validated by LoadManifest to have exactly one element
		items, _ := g.vars[varName].([]string)
		for _, item := range items {
			data := g.varsWithItem(item)
			expanded, err := g.renderString(pattern, data)
			if err != nil {
				return nil, fmt.Errorf("expand loop pattern %q with item %q: %w", pattern, item, err)
			}
			entries = append(entries, loopEntry{
				SourcePrefix:   pattern,
				Item:           item,
				ExpandedPrefix: expanded,
			})
		}
	}

	return entries, nil
}

func (g *internalGenerator) walk(path string, d fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}

	if path == "." || path == "template.toml" || path == "go.mod" || path == "go.sum" {
		return nil
	}

	// Directories: check conditions first (gates entire subtrees including loop source dirs).
	if d.IsDir() {
		skipped, err := g.isConditionedOut(path, nil)
		if err != nil {
			return err
		}
		if skipped {
			return fs.SkipDir
		}

		return nil
	}

	// Files: check loop membership first.
	// A file belongs to a loop if its path starts with any loop source prefix (pattern).
	// Collect matching expanded entries; if the file is a loop source but has no entries
	// (empty items list), skip it entirely.

	// isLoopSource marks files whose FS path falls under a loop-pattern directory.
	// It is derived from g.manifest.Loops (not g.loopEntries) so that loop source
	// files are silently skipped even when the items list is empty (matching is nil).
	isLoopSource := g.isLoopSource(path)
	matching := g.loopEntriesByPath(path)

	if isLoopSource {
		return g.renderLoopEntries(path, matching)
	}

	// Normal file.
	skipped, err := g.isConditionedOut(path, nil)
	if err != nil {
		return err
	}
	if skipped {
		return nil
	}

	return g.renderFile(path)
}

func (g *internalGenerator) loopEntriesByPath(path string) []loopEntry {
	var matching []loopEntry
	for _, e := range g.loopEntries {
		if strings.HasPrefix(path, e.SourcePrefix) {
			matching = append(matching, e)
		}
	}

	return matching
}

func (g *internalGenerator) isLoopSource(path string) bool {
	isLoopSource := false
	for pattern := range g.manifest.Loops {
		if strings.HasPrefix(path, pattern) {
			isLoopSource = true
			break
		}
	}

	return isLoopSource
}

// isConditionedOut reports whether path should be excluded by a condition.
// extraVars is merged into g.vars for the check; pass nil for directory-level checks.
//
// Condition keys that contain {{.item}} render to a non-matching string when extraVars
// is nil (no "item" in scope), so per-item conditions are silently skipped at directory
// walk time. They are evaluated in renderLoopFile with item in scope.
func (g *internalGenerator) isConditionedOut(path string, extraVars map[string]any) (bool, error) {
	var data any = g.vars
	if len(extraVars) > 0 {
		merged := maps.Clone(g.vars)
		maps.Copy(merged, extraVars)
		data = merged
	}
	for condKeyPattern, expr := range g.manifest.Conditions {
		condKey, err := g.renderString(condKeyPattern, data)
		if err != nil {
			return false, fmt.Errorf("render condition key %q: %w", condKeyPattern, err)
		}
		if !strings.HasPrefix(path, condKey) {
			continue
		}
		result, err := g.renderString(expr, data)
		if err != nil {
			return false, fmt.Errorf("evaluate condition for prefix %q: %w", condKeyPattern, err)
		}
		if result == "" {
			return true, nil
		}
	}

	return false, nil
}

// renderFile renders a non-loop template file into the output directory.
func (g *internalGenerator) renderFile(path string) error {
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

	err = os.WriteFile(absPath, rendered, 0600)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// renderLoopFile renders one loop iteration: src path + one item → one output file.
func (g *internalGenerator) renderLoopFile(srcPath string, e loopEntry) error {
	data := g.varsWithItem(e.Item)

	// Compute the expanded output path by replacing the source prefix with the expanded prefix.
	relPath := strings.TrimPrefix(srcPath, e.SourcePrefix)
	expandedPath := e.ExpandedPrefix + relPath

	// Per-item condition check on the expanded path.
	skipped, err := g.isConditionedOut(expandedPath, data)
	if err != nil {
		return err
	}
	if skipped {
		return nil
	}

	outPath, err := g.renderString(strings.TrimSuffix(expandedPath, ".tmpl"), data)
	if err != nil {
		return fmt.Errorf("render loop path %q (item=%q): %w", expandedPath, e.Item, err)
	}
	content, err := fs.ReadFile(g.fsys, srcPath)
	if err != nil {
		return fmt.Errorf("read loop file %q: %w", srcPath, err)
	}
	rendered, err := g.renderBytes(srcPath, content, data)
	if err != nil {
		return fmt.Errorf("render loop content %q (item=%q): %w", srcPath, e.Item, err)
	}
	absPath := filepath.Join(g.outputDir, outPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("mkdir for %q: %w", absPath, err)
	}

	err = os.WriteFile(absPath, rendered, 0600)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// renderLoopEntries renders loop entries.
func (g *internalGenerator) renderLoopEntries(path string, matching []loopEntry) error {
	for _, e := range matching {
		if err := g.renderLoopFile(path, e); err != nil {
			return err
		}
	}

	return nil
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
	t, err := template.
		New(name).
		Funcs(g.funcs).
		Delims(g.manifest.Delimiters[0], g.manifest.Delimiters[1]).
		Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %q: %w", name, err)
	}

	return buf.Bytes(), nil
}
