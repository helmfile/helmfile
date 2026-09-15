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

// TestKubeContext tests the kube-context flag and HELMFILE_KUBE_CONTEXT env var fallback
func TestKubeContext(t *testing.T) {
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
			opts:     GlobalOptions{KubeContext: "flagset"},
			env:      "",
			expected: "flagset",
		},
		{
			opts:     GlobalOptions{KubeContext: "flagset"},
			env:      "envset",
			expected: "flagset",
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.KubeContext, test.env)
		received := NewGlobalImpl(&test.opts).KubeContext()
		require.Equalf(t, test.expected, received, "KubeContext expected %s, received %s", test.expected, received)
	}
	os.Unsetenv(envvar.KubeContext)
}

// TestNamespace tests the namespace flag and HELMFILE_NAMESPACE env var fallback
func TestNamespace(t *testing.T) {
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
			opts:     GlobalOptions{Namespace: "flagset"},
			env:      "",
			expected: "flagset",
		},
		{
			opts:     GlobalOptions{Namespace: "flagset"},
			env:      "envset",
			expected: "flagset",
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.Namespace, test.env)
		received := NewGlobalImpl(&test.opts).Namespace()
		require.Equalf(t, test.expected, received, "Namespace expected %s, received %s", test.expected, received)
	}
	os.Unsetenv(envvar.Namespace)
}

// TestHelmBinary tests the helm-binary flag and HELMFILE_HELM_BINARY env var fallback
func TestHelmBinary(t *testing.T) {
	tests := []struct {
		opts     GlobalOptions
		env      string
		expected string
	}{
		{
			opts:     GlobalOptions{},
			env:      "",
			expected: "helm",
		},
		{
			opts:     GlobalOptions{},
			env:      "envset",
			expected: "envset",
		},
		{
			opts:     GlobalOptions{HelmBinary: "flagset"},
			env:      "",
			expected: "flagset",
		},
		{
			opts:     GlobalOptions{HelmBinary: "flagset"},
			env:      "envset",
			expected: "flagset",
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.HelmBinary, test.env)
		received := NewGlobalImpl(&test.opts).HelmBinary()
		require.Equalf(t, test.expected, received, "HelmBinary expected %s, received %s", test.expected, received)
	}
	os.Unsetenv(envvar.HelmBinary)
}

// TestKustomizeBinary tests the kustomize-binary flag and HELMFILE_KUSTOMIZE_BINARY env var fallback
func TestKustomizeBinary(t *testing.T) {
	tests := []struct {
		opts     GlobalOptions
		env      string
		expected string
	}{
		{
			opts:     GlobalOptions{},
			env:      "",
			expected: "kustomize",
		},
		{
			opts:     GlobalOptions{},
			env:      "envset",
			expected: "envset",
		},
		{
			opts:     GlobalOptions{KustomizeBinary: "flagset"},
			env:      "",
			expected: "flagset",
		},
		{
			opts:     GlobalOptions{KustomizeBinary: "flagset"},
			env:      "envset",
			expected: "flagset",
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.KustomizeBinary, test.env)
		received := NewGlobalImpl(&test.opts).KustomizeBinary()
		require.Equalf(t, test.expected, received, "KustomizeBinary expected %s, received %s", test.expected, received)
	}
	os.Unsetenv(envvar.KustomizeBinary)
}

// TestLogLevel tests the log-level flag and HELMFILE_LOG_LEVEL env var fallback
func TestLogLevel(t *testing.T) {
	tests := []struct {
		opts     GlobalOptions
		env      string
		expected string
	}{
		{
			opts:     GlobalOptions{},
			env:      "",
			expected: "info",
		},
		{
			opts:     GlobalOptions{},
			env:      "envset",
			expected: "envset",
		},
		{
			opts:     GlobalOptions{LogLevel: "flagset"},
			env:      "",
			expected: "flagset",
		},
		{
			opts:     GlobalOptions{LogLevel: "flagset"},
			env:      "envset",
			expected: "flagset",
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.LogLevel, test.env)
		received := NewGlobalImpl(&test.opts).LogLevel()
		require.Equalf(t, test.expected, received, "LogLevel expected %s, received %s", test.expected, received)
	}
	os.Unsetenv(envvar.LogLevel)
}

// TestDebug tests the debug flag and HELMFILE_DEBUG env var fallback
func TestDebug(t *testing.T) {
	tests := []struct {
		opts     GlobalOptions
		env      string
		expected bool
	}{
		{
			opts:     GlobalOptions{},
			env:      "",
			expected: false,
		},
		{
			opts:     GlobalOptions{},
			env:      "true",
			expected: true,
		},
		{
			opts:     GlobalOptions{},
			env:      "anything",
			expected: false,
		},
		{
			opts:     GlobalOptions{Debug: true},
			env:      "",
			expected: true,
		},
		{
			opts:     GlobalOptions{Debug: true},
			env:      "true",
			expected: true,
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.Debug, test.env)
		received := NewGlobalImpl(&test.opts).Debug()
		require.Equalf(t, test.expected, received, "Debug expected %t, received %t", test.expected, received)
	}
	os.Unsetenv(envvar.Debug)
}

// TestQuiet tests the quiet flag and HELMFILE_QUIET env var fallback
func TestQuiet(t *testing.T) {
	tests := []struct {
		opts     GlobalOptions
		env      string
		expected bool
	}{
		{
			opts:     GlobalOptions{},
			env:      "",
			expected: false,
		},
		{
			opts:     GlobalOptions{},
			env:      "true",
			expected: true,
		},
		{
			opts:     GlobalOptions{},
			env:      "anything",
			expected: false,
		},
		{
			opts:     GlobalOptions{Quiet: true},
			env:      "",
			expected: true,
		},
		{
			opts:     GlobalOptions{Quiet: true},
			env:      "true",
			expected: true,
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.Quiet, test.env)
		received := NewGlobalImpl(&test.opts).Quiet()
		require.Equalf(t, test.expected, received, "Quiet expected %t, received %t", test.expected, received)
	}
	os.Unsetenv(envvar.Quiet)
}

// TestNoColor tests the no-color flag, HELMFILE_NO_COLOR and NO_COLOR env var fallbacks
func TestNoColor(t *testing.T) {
	tests := []struct {
		opts        GlobalOptions
		helmfileEnv string
		standardEnv string
		expected    bool
	}{
		{
			opts:        GlobalOptions{},
			helmfileEnv: "",
			standardEnv: "",
			expected:    false,
		},
		{
			opts:        GlobalOptions{},
			helmfileEnv: "true",
			standardEnv: "",
			expected:    true,
		},
		{
			opts:        GlobalOptions{},
			helmfileEnv: "anything",
			standardEnv: "",
			expected:    false,
		},
		{
			opts:        GlobalOptions{},
			helmfileEnv: "",
			standardEnv: "1",
			expected:    true,
		},
		{
			opts:        GlobalOptions{},
			helmfileEnv: "",
			standardEnv: "anything",
			expected:    true,
		},
		{
			opts:        GlobalOptions{NoColor: true},
			helmfileEnv: "",
			standardEnv: "",
			expected:    true,
		},
	}

	for _, test := range tests {
		os.Setenv(envvar.NoColor, test.helmfileEnv)
		os.Setenv("NO_COLOR", test.standardEnv)
		received := NewGlobalImpl(&test.opts).NoColor()
		require.Equalf(t, test.expected, received, "NoColor expected %t, received %t", test.expected, received)
	}
	os.Unsetenv(envvar.NoColor)
	os.Unsetenv("NO_COLOR")
}

// TestColorRespectsNoColorEnv guards against ValidateConfig() firing when
// HELMFILE_NO_COLOR / NO_COLOR is set without an explicit --color/--no-color flag.
// Color() must consult NoColor() (which is env-aware) before falling back to TTY autodetect.
func TestColorRespectsNoColorEnv(t *testing.T) {
	tests := []struct {
		name        string
		helmfileEnv string
		standardEnv string
	}{
		{name: "HELMFILE_NO_COLOR=true", helmfileEnv: "true"},
		{name: "NO_COLOR set", standardEnv: "1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(envvar.NoColor, test.helmfileEnv)
			t.Setenv("NO_COLOR", test.standardEnv)
			g := NewGlobalImpl(&GlobalOptions{})
			require.True(t, g.NoColor(), "NoColor() should be true when env is set")
			require.False(t, g.Color(), "Color() should be false when NoColor() is true via env")
			require.NoError(t, g.ValidateConfig(), "ValidateConfig() should not error from env-only no-color")
		})
	}
}

// TestColorFlagOverridesNoColorEnv guards against ValidateConfig() firing when
// --color is explicitly passed but HELMFILE_NO_COLOR / NO_COLOR is set in the
// environment. The flag must win over the env var.
func TestColorFlagOverridesNoColorEnv(t *testing.T) {
	tests := []struct {
		name        string
		helmfileEnv string
		standardEnv string
	}{
		{name: "HELMFILE_NO_COLOR=true", helmfileEnv: "true"},
		{name: "NO_COLOR set", standardEnv: "1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(envvar.NoColor, test.helmfileEnv)
			t.Setenv("NO_COLOR", test.standardEnv)
			g := NewGlobalImpl(&GlobalOptions{Color: true})
			require.True(t, g.Color(), "Color() should be true when --color is set")
			require.False(t, g.NoColor(), "NoColor() should be false when --color is set, even if env says otherwise")
			require.NoError(t, g.ValidateConfig(), "ValidateConfig() should not error when --color overrides env no-color")
		})
	}
}

// TestRepoRetry tests the repo-retries flag and HELMFILE_REPO_RETRIES env var fallback
func TestRepoRetry(t *testing.T) {
	// RepoRetry < 0 means the flag was not specified (CLI default sentinel).
	unset := GlobalOptions{RepoRetry: -1}
	tests := []struct {
		name     string
		opts     GlobalOptions
		env      string
		expected int
	}{
		{name: "default (unset)", opts: unset, env: "", expected: 0},
		{name: "env set", opts: unset, env: "3", expected: 3},
		{name: "flag set", opts: GlobalOptions{RepoRetry: 5}, env: "", expected: 5},
		{name: "flag overrides env", opts: GlobalOptions{RepoRetry: 5}, env: "3", expected: 5},
		{name: "flag zero disables env", opts: GlobalOptions{RepoRetry: 0}, env: "3", expected: 0},
		{name: "invalid env ignored", opts: unset, env: "abc", expected: 0},
		{name: "negative env ignored", opts: unset, env: "-1", expected: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(envvar.RepoRetry, test.env)
			received := NewGlobalImpl(&test.opts).RepoRetry()
			require.Equalf(t, test.expected, received, "RepoRetry expected %d, received %d", test.expected, received)
		})
	}
}
