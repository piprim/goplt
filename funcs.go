// funcs.go
package goplt

import (
	"strings"
	"text/template"

	"github.com/huandu/xstrings"
)

// DefaultFuncMap returns the built-in template function map available in every
// goplt template. Callers may use this to inspect or extend the default set.
func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"snake":     xstrings.ToSnakeCase,
		"camel":     xstrings.ToCamelCase,
		"pascal":    xstrings.ToPascalCase,
		"kebab":     xstrings.ToKebabCase,
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"trim":      strings.TrimSpace,
		"replace":   func(old, replacement, s string) string { return strings.ReplaceAll(s, old, replacement) },
		"hasPrefix": func(prefix, s string) bool { return strings.HasPrefix(s, prefix) },
		"hasSuffix": func(suffix, s string) bool { return strings.HasSuffix(s, suffix) },
		"contains":  func(substr, s string) bool { return strings.Contains(s, substr) },
	}
}
