// cmd/goplt/cmd/version_test.go
package cmd

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_output(t *testing.T) {
	cv := Version
	cn := Name
	Version = "v1.2.3"
	Name = "plop"
	t.Cleanup(func() { Version = cv; Name = cn })

	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.Execute())
	out := buf.String()
	fmt.Println(out)
	assert.Contains(t, out, Name)
	assert.Contains(t, out, Version)
}
