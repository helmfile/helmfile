//go:build windows

package remote

import (
	"testing"
)

func TestIsRemote_Windows(t *testing.T) {
	testcases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "windows drive letter path",
			input:    `C:\project\services\values.yaml`,
			expected: false,
		},
		{
			name:     "windows UNC path",
			input:    `\\server\share\path\values.yaml`,
			expected: false,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRemote(tt.input)
			if result != tt.expected {
				t.Errorf("IsRemote(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParse_Windows(t *testing.T) {
	testcases := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "windows drive letter path",
			input: `C:\project\services\values.yaml`,
			err:   `parse url: local absolute path is not a remote URL: C:\project\services\values.yaml`,
		},
		{
			name:  "windows UNC path",
			input: `\\server\share\path\values.yaml`,
			err:   `parse url: local absolute path is not a remote URL: \\server\share\path\values.yaml`,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", tt.input)
			}
			if _, ok := err.(InvalidURLError); !ok {
				t.Fatalf("Parse(%q) expected InvalidURLError, got %T: %v", tt.input, err, err)
			}
			if err.Error() != tt.err {
				t.Errorf("Parse(%q) error = %q, want %q", tt.input, err.Error(), tt.err)
			}
		})
	}
}
