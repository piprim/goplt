package cmd

import (
	"strings"
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
)

func TestBuildDefaultVars_kindText_withDefault(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "OrgPrefix", Kind: goplt.KindText, Default: "github.com/acme"},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, "github.com/acme", result["OrgPrefix"])
}

func TestBuildDefaultVars_kindText_emptyDefault_usesName(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "Name", Kind: goplt.KindText, Default: ""},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, "Name", result["Name"])
}

func TestBuildDefaultVars_kindBool(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "WithDocker", Kind: goplt.KindBool, Default: true},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, true, result["WithDocker"])
}

func TestBuildDefaultVars_kindChoiceString_firstElement(t *testing.T) {
	vars := []goplt.Variable{
		{Name: "License", Kind: goplt.KindChoiceString, Default: []string{"MIT", "Apache-2.0"}},
	}
	result := buildDefaultVars(vars)
	assert.Equal(t, "MIT", result["License"])
}

func TestBuildScript_noHooks(t *testing.T) {
	script := buildScript(nil)
	assert.Contains(t, script, "set -e")
	assert.Contains(t, script, "tar xf -")
	assert.Contains(t, script, "go build ./...")
	assert.Contains(t, script, "go test ./...")
}

func TestBuildScript_withHooks_appearsBeforeBuild(t *testing.T) {
	script := buildScript([]string{"go mod tidy", "git init"})
	assert.Contains(t, script, "go mod tidy")
	assert.Contains(t, script, "git init")
	tidyIdx := strings.Index(script, "go mod tidy")
	buildIdx := strings.Index(script, "go build ./...")
	assert.Less(t, tidyIdx, buildIdx, "hooks must appear before go build")
}
