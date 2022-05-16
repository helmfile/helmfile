package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoMatchingHelmfileError tests the NoMatchingHelmfileError error
func TestNoMatchingHelmfileError_Error(t *testing.T) {
	tests := []struct {
		selectors []string
		env       string
		expected  string
	}{
		{
			selectors: []string{"a", "b"},
			env:       "c",
			expected:  "err: no releases found that matches specified selector(a, b) and environment(c), in any helmfile",
		},
		{
			selectors: []string{"a", "b"},
			expected:  "err: no releases found that matches specified selector(a, b) and environment(), in any helmfile",
		},
		{
			env:      "c",
			expected: "err: no releases found that matches specified selector() and environment(c), in any helmfile",
		},
	}

	for _, test := range tests {
		err := &NoMatchingHelmfileError{
			selectors: test.selectors,
			env:       test.env,
		}

		require.Equal(t, test.expected, err.Error())
	}
}
