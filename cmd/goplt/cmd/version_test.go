// cmd/goplt/cmd/version_test.go
package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_containsVersionAndName(t *testing.T) {
	orig := Version
	Version = "v1.2.3"
	t.Cleanup(func() { Version = orig })

	origName := Name
	Name = "mytool"
	t.Cleanup(func() { Name = origName })

	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.Execute())
	out := buf.String()

	assert.Contains(t, out, "v1.2.3")
	assert.Contains(t, out, "mytool")
}

func TestVersionCmd_defaultVersion_isNone(t *testing.T) {
	orig := Version
	Version = defaultVersion
	t.Cleanup(func() { Version = orig })

	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), defaultVersion)
}
