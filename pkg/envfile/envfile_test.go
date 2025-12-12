package envfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		existingEnv map[string]string
		expectedEnv map[string]string
	}{
		{
			name: "basic key=value",
			content: `FOO=bar
BAZ=qux`,
			expectedEnv: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name: "comments and empty lines",
			content: `# This is a comment
FOO=bar

# Another comment
BAZ=qux`,
			expectedEnv: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name: "double quoted values",
			content: `DOUBLE="hello world"
UNQUOTED=hello`,
			expectedEnv: map[string]string{
				"DOUBLE":   "hello world",
				"UNQUOTED": "hello",
			},
		},
		{
			name:    "single quoted values",
			content: `SINGLE='hello world'`,
			expectedEnv: map[string]string{
				"SINGLE": "hello world",
			},
		},
		{
			name: "does not override existing",
			content: `EXISTING=fromfile
NEW=fromfile`,
			existingEnv: map[string]string{
				"EXISTING": "original",
			},
			expectedEnv: map[string]string{
				"EXISTING": "original",
				"NEW":      "fromfile",
			},
		},
		{
			name:    "handles values with equals sign",
			content: `CONNECTION=postgres://user:pass@host/db?foo=bar`,
			expectedEnv: map[string]string{
				"CONNECTION": "postgres://user:pass@host/db?foo=bar",
			},
		},
		{
			name:    "empty value",
			content: `EMPTY=`,
			expectedEnv: map[string]string{
				"EMPTY": "",
			},
		},
		{
			name:    "whitespace handling",
			content: `  SPACED  =  value  `,
			expectedEnv: map[string]string{
				"SPACED": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up env vars after test
			defer func() {
				for k := range tt.expectedEnv {
					os.Unsetenv(k)
				}
				for k := range tt.existingEnv {
					os.Unsetenv(k)
				}
			}()

			// Set up existing env vars
			for k, v := range tt.existingEnv {
				os.Setenv(k, v)
			}

			// Create temp file
			tmpDir := t.TempDir()
			envPath := filepath.Join(tmpDir, ".env")
			err := os.WriteFile(envPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			// Load env file
			err = Load(envPath)
			require.NoError(t, err)

			// Check expected values
			for k, expected := range tt.expectedEnv {
				actual := os.Getenv(k)
				require.Equal(t, expected, actual, "env var %s", k)
			}
		})
	}
}

func TestLoad_FileNotExists(t *testing.T) {
	// Should not error when file doesn't exist
	err := Load("/nonexistent/path/.env")
	require.NoError(t, err)
}

func TestLoad_EmptyPath(t *testing.T) {
	// Should not error when path is empty
	err := Load("")
	require.NoError(t, err)
}

func TestParseEnvLine(t *testing.T) {
	tests := []struct {
		line    string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{"FOO=bar", "FOO", "bar", true},
		{"FOO=", "FOO", "", true},
		{"FOO", "", "", false},
		{"=bar", "", "", false},
		{"FOO=bar=baz", "FOO", "bar=baz", true},
		{"  FOO  =  bar  ", "FOO", "bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			key, val, ok := parseEnvLine(tt.line)
			require.Equal(t, tt.wantOK, ok)
			if ok {
				require.Equal(t, tt.wantKey, key)
				require.Equal(t, tt.wantVal, val)
			}
		})
	}
}

func TestLoad_PermissionDenied(t *testing.T) {
	// Create a file with no read permissions
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, []byte("FOO=bar"), 0o000)
	require.NoError(t, err)

	// Should return an error (not silently ignore like file-not-found)
	err = Load(envPath)
	require.Error(t, err)
}

func TestLoad_CRLFLineEndings(t *testing.T) {
	// Clean up env vars after test
	defer func() {
		os.Unsetenv("CRLF_FOO")
		os.Unsetenv("CRLF_BAR")
	}()

	// Create temp file with Windows-style line endings
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	content := "CRLF_FOO=bar\r\nCRLF_BAR=baz\r\n"
	err := os.WriteFile(envPath, []byte(content), 0o644)
	require.NoError(t, err)

	// Load env file
	err = Load(envPath)
	require.NoError(t, err)

	// Check values don't have \r in them
	require.Equal(t, "bar", os.Getenv("CRLF_FOO"))
	require.Equal(t, "baz", os.Getenv("CRLF_BAR"))
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{"hello", "hello"},
		{`"hello`, `"hello`},
		{`hello"`, `hello"`},
		{`""`, ""},
		{`''`, ""},
		{`"hello'`, `"hello'`},
		{`  "hello"  `, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := trimQuotes(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
