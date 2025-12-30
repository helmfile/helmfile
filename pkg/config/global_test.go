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

// TestSkipSecrets tests SkipSecrets option with CLI flag and environment variable
func TestSkipSecrets(t *testing.T) {
	tests := []struct {
		name     string
		opts     GlobalOptions
		env      string
		expected bool
	}{
		{
			name:     "default is false",
			opts:     GlobalOptions{},
			env:      "",
			expected: false,
		},
		{
			name:     "CLI flag true",
			opts:     GlobalOptions{SkipSecrets: true},
			env:      "",
			expected: true,
		},
		{
			name:     "env variable true",
			opts:     GlobalOptions{},
			env:      "true",
			expected: true,
		},
		{
			name:     "CLI flag true takes precedence over env false",
			opts:     GlobalOptions{SkipSecrets: true},
			env:      "false",
			expected: true,
		},
		{
			name:     "CLI flag false with env true uses env",
			opts:     GlobalOptions{SkipSecrets: false},
			env:      "true",
			expected: true,
		},
		{
			name:     "env variable non-true value is false",
			opts:     GlobalOptions{},
			env:      "yes",
			expected: false,
		},
		{
			name:     "env variable TRUE (uppercase) is false",
			opts:     GlobalOptions{},
			env:      "TRUE",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv(envvar.SkipSecrets, test.env)
			defer os.Unsetenv(envvar.SkipSecrets)

			received := NewGlobalImpl(&test.opts).SkipSecrets()
			require.Equalf(t, test.expected, received, "SkipSecrets expected %t, received %t", test.expected, received)
		})
	}
}
