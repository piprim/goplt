// cmd/goplt/cmd/version_test.go
package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_output(t *testing.T) {
	Version = "v1.2.3"
	Commit = "abc1234"
	BuildDate = "2026-04-18"
	t.Cleanup(func() { Version = ""; Commit = "none"; BuildDate = "unknown" })

	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "goplt v1.2.3 (commit: abc1234, built: 2026-04-18)\n", buf.String())
}

func TestResolvedVersion_fallsBackToModuleInfo(t *testing.T) {
	Version = ""
	t.Cleanup(func() { Version = "" })

	v := resolvedVersion()
	// In a test binary, debug.ReadBuildInfo returns "(devel)" or empty,
	// so the final fallback "dev" is expected.
	assert.NotEmpty(t, v)
}
