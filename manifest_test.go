package goplt_test

import (
	"testing"
	"testing/fstest"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"with-connect", "WithConnect"},
		{"with_connect", "WithConnect"},
		{"withConnect", "WithConnect"},
		{"name", "Name"},
		{"org-prefix", "OrgPrefix"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, goplt.NormalizeKey(tc.input))
		})
	}
}

func TestLoadManifest_targetDir_parsed(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
target-dir = "{{.Name}}"

[variables]
name = ""
`)},
	}

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Equal(t, "{{.Name}}", m.TargetDir)
}

func TestLoadManifest_targetDir_absent(t *testing.T) {
	fsys := fstest.MapFS{
		"template.toml": &fstest.MapFile{Data: []byte(`
[variables]
name = ""
`)},
	}

	m, err := goplt.LoadManifest(fsys)
	require.NoError(t, err)
	assert.Empty(t, m.TargetDir)
}
