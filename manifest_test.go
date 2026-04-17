package goplt_test

import (
	"testing"

	"github.com/piprim/goplt"
	"github.com/stretchr/testify/assert"
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
