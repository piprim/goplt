// funcs_test.go
package goplt_test

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderWithDefaultFuncs is a test helper that executes a template string
// using the default goplt function map.
func renderWithDefaultFuncs(t *testing.T, tmplStr string, data any) string {
	t.Helper()
	fm := goplt.DefaultFuncMap()
	parsed, err := template.New("").Funcs(fm).Parse(tmplStr)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, parsed.Execute(&buf, data))

	return buf.String()
}

func TestDefaultFuncMap(t *testing.T) {
	t.Run("snake", func(t *testing.T) {
		assert.Equal(t, "my_app", renderWithDefaultFuncs(t, `{{"MyApp" | snake}}`, nil))
		assert.Equal(t, "my_app", renderWithDefaultFuncs(t, `{{"my-app" | snake}}`, nil))
	})

	t.Run("camel", func(t *testing.T) {
		assert.Equal(t, "myApp", renderWithDefaultFuncs(t, `{{"my_app" | camel}}`, nil))
	})

	t.Run("pascal", func(t *testing.T) {
		assert.Equal(t, "MyApp", renderWithDefaultFuncs(t, `{{"my_app" | pascal}}`, nil))
	})

	t.Run("kebab", func(t *testing.T) {
		assert.Equal(t, "my-app", renderWithDefaultFuncs(t, `{{"MyApp" | kebab}}`, nil))
	})

	t.Run("upper", func(t *testing.T) {
		assert.Equal(t, "HELLO", renderWithDefaultFuncs(t, `{{"hello" | upper}}`, nil))
	})

	t.Run("lower", func(t *testing.T) {
		assert.Equal(t, "hello", renderWithDefaultFuncs(t, `{{"HELLO" | lower}}`, nil))
	})

	t.Run("trim", func(t *testing.T) {
		assert.Equal(t, "hello", renderWithDefaultFuncs(t, `{{"  hello  " | trim}}`, nil))
	})

	t.Run("replace", func(t *testing.T) {
		assert.Equal(t, "b-b", renderWithDefaultFuncs(t, `{{replace "a" "b" "a-a"}}`, nil))
	})

	t.Run("has_prefix", func(t *testing.T) {
		assert.Equal(t, "true", renderWithDefaultFuncs(t, `{{hasPrefix "he" "hello" | printf "%v"}}`, nil))
		assert.Equal(t, "false", renderWithDefaultFuncs(t, `{{hasPrefix "wo" "hello" | printf "%v"}}`, nil))
	})

	t.Run("has_suffix", func(t *testing.T) {
		assert.Equal(t, "true", renderWithDefaultFuncs(t, `{{hasSuffix "lo" "hello" | printf "%v"}}`, nil))
	})

	t.Run("contains", func(t *testing.T) {
		assert.Equal(t, "true", renderWithDefaultFuncs(t, `{{contains "ell" "hello" | printf "%v"}}`, nil))
	})
}
