package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/envvar"
)

// TestFileOrDir tests if statement
func TestFileOrDir(t *testing.T) {
	tests := []struct {
		opts     GlobalOptions
		env      string
		expected string
	}{
		{
			opts:     GlobalOptions{},
			env:      "",
			expected: "",
		},
		{
			opts:     GlobalOptions{},
			env:      "envset",
			expected: "envset",
		},
		{
			opts:     GlobalOptions{File: "folderset"},
			env:      "",
			expected: "folderset",
		},
		{
			opts:     GlobalOptions{File: "folderset"},
			env:      "envset",
			expected: "folderset",
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.FilePath, test.env)
		received := NewGlobalImpl(&test.opts).FileOrDir()
		require.Equalf(t, test.expected, received, "FileOrDir expected %t, received %t", test.expected, received)
	}
	os.Unsetenv(envvar.FilePath)
}
