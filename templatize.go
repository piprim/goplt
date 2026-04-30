package goplt

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/huandu/xstrings" //nolint:depguard // xstrings is an approved dependency for this package
)

type Substitution struct {
	Value       string
	Placeholder string
}

type SubstitutionResult struct {
	From  string
	To    string
	Count int
	Files []string
}

type TemplatizeReport struct {
	Results     []SubstitutionResult
	Skipped     map[string]int
	BinaryFiles []string
}

func ReadModulePath(dir string) (string, error) {
	f, err := os.Open(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("templatize: open go.mod in %s: %w", dir, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("templatize: scan go.mod: %w", err)
	}

	return "", fmt.Errorf("templatize: no module directive found in %s/go.mod", dir)
}

// PascalCase, CamelCase, SnakeCase, KebabCase are exported so CLI commands
// can derive case forms without taking a direct dependency on xstrings.
func PascalCase(s string) string { return xstrings.ToPascalCase(s) }
func CamelCase(s string) string  { return xstrings.ToCamelCase(s) }
func SnakeCase(s string) string  { return xstrings.ToSnakeCase(s) }
func KebabCase(s string) string  { return xstrings.ToKebabCase(s) }

// BuildSubstitutions derives all case forms of name and adds orgPrefix as-is.
// Skip strings become identity pairs (value == placeholder) so the replacer
// leaves them untouched. All pairs are sorted by value length descending so
// longer strings always win over shorter prefix matches.
func BuildSubstitutions(name, orgPrefix string, skip []string) []Substitution {
	forms := []Substitution{
		{Value: xstrings.ToKebabCase(name), Placeholder: "{{.Name}}"},
		{Value: xstrings.ToPascalCase(name), Placeholder: "{{.Name | pascal}}"},
		{Value: xstrings.ToCamelCase(name), Placeholder: "{{.Name | camel}}"},
		{Value: xstrings.ToSnakeCase(name), Placeholder: "{{.Name | snake}}"},
		{Value: orgPrefix, Placeholder: "{{.OrgPrefix}}"},
	}

	seen := map[string]struct{}{}
	deduped := make([]Substitution, 0, len(forms))

	for _, s := range forms {
		if s.Value == "" {
			continue
		}
		if _, ok := seen[s.Value]; ok {
			continue
		}

		seen[s.Value] = struct{}{}
		deduped = append(deduped, s)
	}

	for _, sk := range skip {
		if sk == "" {
			continue
		}
		if _, ok := seen[sk]; ok {
			continue
		}

		seen[sk] = struct{}{}
		deduped = append(deduped, Substitution{Value: sk, Placeholder: sk})
	}

	slices.SortStableFunc(deduped, func(a, b Substitution) int {
		return len(b.Value) - len(a.Value)
	})

	return deduped
}

const (
	binarySniffBytes = 512
	dirPerm          = 0o755
	pairsPerSub      = 2
)

// Templatize copies fsys to outputDir, applying subs to every text file's path
// and content. Binary files (detected by null byte in first 512 bytes) are
// copied verbatim. The .git directory is always skipped.
// Returns a TemplatizeReport summarising all substitutions and protected strings.
//
//nolint:gocognit,funlen // flat walk function; helper structs would obscure the logic
func Templatize(fsys fs.FS, outputDir string, subs []Substitution) (*TemplatizeReport, error) {
	pairs := make([]string, 0, len(subs)*pairsPerSub)
	for _, s := range subs {
		pairs = append(pairs, s.Value, s.Placeholder)
	}

	replacer := strings.NewReplacer(pairs...)
	counts := make(map[string]int, len(subs))
	fileSet := make(map[string]map[string]struct{}, len(subs))
	skipped := make(map[string]int)
	binaryFiles := make([]string, 0)

	for _, s := range subs {
		fileSet[s.Value] = make(map[string]struct{})
	}

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		if d.IsDir() && path == ".git" {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if path == "go.sum" {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("templatize: read %s: %w", path, err)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("templatize: stat %s: %w", path, err)
		}

		srcPerm := info.Mode().Perm()
		if srcPerm == 0 {
			srcPerm = 0o644
		}

		outRelPath := replacer.Replace(path)
		if path == "go.mod" {
			outRelPath += ".tmpl"
		}
		absOut := filepath.Join(outputDir, outRelPath)

		if err := os.MkdirAll(filepath.Dir(absOut), dirPerm); err != nil {
			return fmt.Errorf("templatize: mkdir %s: %w", filepath.Dir(absOut), err)
		}

		sniff := data
		if len(sniff) > binarySniffBytes {
			sniff = sniff[:binarySniffBytes]
		}

		if isBinary(sniff) {
			if err := os.WriteFile(absOut, data, srcPerm); err != nil {
				return fmt.Errorf("templatize: write binary %s: %w", outRelPath, err)
			}

			binaryFiles = append(binaryFiles, path)

			return nil
		}

		content := string(data)

		for _, s := range subs {
			c := strings.Count(content, s.Value)
			if c == 0 {
				continue
			}

			if s.Value == s.Placeholder {
				skipped[s.Value] += c
			} else {
				counts[s.Value] += c
				fileSet[s.Value][path] = struct{}{}
			}
		}

		rendered := escapeNonPlaceholders(replacer.Replace(content), subs)

		if err := os.WriteFile(absOut, []byte(rendered), srcPerm); err != nil {
			return fmt.Errorf("templatize: write %s: %w", outRelPath, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("templatize: walk: %w", err)
	}

	results := make([]SubstitutionResult, 0, len(subs))

	for _, s := range subs {
		if s.Value == s.Placeholder || counts[s.Value] == 0 {
			continue
		}

		fileList := make([]string, 0, len(fileSet[s.Value]))
		for f := range fileSet[s.Value] {
			fileList = append(fileList, f)
		}

		slices.Sort(fileList)
		results = append(results, SubstitutionResult{
			From:  s.Value,
			To:    s.Placeholder,
			Count: counts[s.Value],
			Files: fileList,
		})
	}

	slices.Sort(binaryFiles)

	return &TemplatizeReport{Results: results, Skipped: skipped, BinaryFiles: binaryFiles}, nil
}

// isBinary reports whether data looks like a binary file by checking for a null
// byte in the first binarySniffBytes bytes. This mirrors the heuristic used by
// git and many editors to distinguish text from binary content.
func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}

// escapeNonPlaceholders replaces any {{ in s that is not the start of a known
// substitution placeholder with {{"{{"}} — the Go template expression that
// outputs a literal {{. Without this, source files that contain {{ (shell
// scripts, plain-text guards, HTML templates) would produce invalid templates
// that fail with "function not defined" when rendered by goplt generate.
func escapeNonPlaceholders(s string, subs []Substitution) string {
	if !strings.Contains(s, "{{") {
		return s
	}

	placeholders := make([]string, 0, len(subs))
	for _, sub := range subs {
		if sub.Value != sub.Placeholder && strings.Contains(sub.Placeholder, "{{") {
			placeholders = append(placeholders, sub.Placeholder)
		}
	}

	var buf strings.Builder
	buf.Grow(len(s))

	remaining := s
	for {
		idx := strings.Index(remaining, "{{")
		if idx == -1 {
			buf.WriteString(remaining)

			break
		}

		buf.WriteString(remaining[:idx])
		rest := remaining[idx:]

		matched := false
		for _, ph := range placeholders {
			if strings.HasPrefix(rest, ph) {
				buf.WriteString(ph)
				remaining = rest[len(ph):]
				matched = true

				break
			}
		}

		if !matched {
			buf.WriteString(`{{"{{"}}`)
			remaining = rest[2:]
		}
	}

	return buf.String()
}
