package helmexec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test the specific scenario mentioned in issue #2124
func TestFindSemVerInfo_Issue2124(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "issue #2124 - kustomize version 5.7.1 without v prefix",
			input:    "5.7.1",
			expected: "v5.7.1",
			wantErr:  false,
		},
		{
			name:     "kustomize version with v prefix",
			input:    "v5.7.1",
			expected: "v5.7.1",
			wantErr:  false,
		},
		{
			name:     "kustomize structured output",
			input:    "{v5.7.1  2025-07-23T12:45:29Z   }",
			expected: "v5.7.1",
			wantErr:  false,
		},
		{
			name:     "helm version format",
			input:    "v3.18.4+gd80839c",
			expected: "v3.18.4+gd80839c",
			wantErr:  false,
		},
		{
			name:    "invalid version",
			input:   "not-a-version",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := findSemVerInfo(tc.input)
			
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}