package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRemoteRef(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"./my-template", false},
		{"../sibling", false},
		{"/absolute/path", false},
		{"~/templates/foo", false},
		{"relative-no-dot", false},
		{"relative/no-dot", false},
		{"", false},
		{"github.com/piprim/goplt-tmpl/cli-cobra", true},
		{"github.com/piprim/goplt-tmpl/cli-cobra@v1.0.0", true},
		{"golang.org/x/example/hello", true},
		{"plop.com/xxx/yyy/zzz/123", true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, isRemoteRef(tc.input))
		})
	}
}

func TestParseRemoteRef(t *testing.T) {
	tests := []struct {
		input   string
		module  string
		version string
	}{
		{
			"github.com/piprim/goplt-tmpl/cli-cobra",
			"github.com/piprim/goplt-tmpl/cli-cobra",
			"latest",
		},
		{
			"github.com/piprim/goplt-tmpl/cli-cobra@v1.0.0",
			"github.com/piprim/goplt-tmpl/cli-cobra",
			"v1.0.0",
		},
		{
			"github.com/piprim/goplt-tmpl/cli-cobra@main",
			"github.com/piprim/goplt-tmpl/cli-cobra",
			"main",
		},
		{
			"github.com/piprim/goplt-tmpl/cli-cobra@latest",
			"github.com/piprim/goplt-tmpl/cli-cobra",
			"latest",
		},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			mod, ver := parseRemoteRef(tc.input)
			assert.Equal(t, tc.module, mod)
			assert.Equal(t, tc.version, ver)
		})
	}
}

func TestResolveRemote_real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	dir, err := resolveRemote("github.com/piprim/goplt-tmpl/cli-cobra@latest")
	require.NoError(t, err)
	assert.NotEmpty(t, dir)

	_, err = os.Stat(filepath.Join(dir, "template.toml"))
	assert.NoError(t, err, "template.toml must exist in the resolved module directory")
}
