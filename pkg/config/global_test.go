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

// TestEnvFile tests the EnvFile() method precedence
func TestEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		opts     GlobalOptions
		env      string
		expected string
	}{
		{
			name:     "empty when nothing set",
			opts:     GlobalOptions{},
			env:      "",
			expected: "",
		},
		{
			name:     "env var used when flag not set",
			opts:     GlobalOptions{},
			env:      ".env.production",
			expected: ".env.production",
		},
		{
			name:     "flag takes precedence over env var",
			opts:     GlobalOptions{EnvFile: ".env.local"},
			env:      ".env.production",
			expected: ".env.local",
		},
		{
			name:     "flag used when env var not set",
			opts:     GlobalOptions{EnvFile: ".env"},
			env:      "",
			expected: ".env",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.env != "" {
				os.Setenv(envvar.EnvFile, test.env)
				defer os.Unsetenv(envvar.EnvFile)
			} else {
				os.Unsetenv(envvar.EnvFile)
			}
			received := NewGlobalImpl(&test.opts).EnvFile()
			require.Equal(t, test.expected, received)
		})
	}
}
