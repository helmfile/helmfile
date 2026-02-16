package app

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestDownloadfile(t *testing.T) {
	cases := []struct {
		name        string
		handler     func(http.ResponseWriter, *http.Request)
		filepath    string
		wantContent string
		wantError   string
	}{
		{
			name: "successful download of file content",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "helmfile")
			},
			wantContent: "helmfile",
		},
		{
			name: "404 error when file not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, "not found")
			},
			wantError: "download .*? error, code: 404",
		},
		{
			name: "500 error on server failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "server error")
			},
			wantError: "download .*? error, code: 500",
		},
		{
			name: "error due to invalid file path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "helmfile")
			},
			filepath:  "abc/down.txt",
			wantError: "open .*? no such file or directory",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			downfile := filepath.Join(dir, "down.txt")
			if c.filepath != "" {
				downfile = filepath.Join(dir, c.filepath)
			}

			ts := httptest.NewServer(http.HandlerFunc(c.handler))
			defer ts.Close()

			err := downloadfile(downfile, ts.URL)

			if c.wantError != "" {
				assert.Error(t, err)
				if err != nil {
					matched, regexErr := regexp.MatchString(c.wantError, err.Error())
					assert.NoError(t, regexErr)
					assert.True(t, matched, "expected error message to match regex: %s", c.wantError)
				}
				return
			}

			content, err := os.ReadFile(downfile)
			assert.NoError(t, err)
			assert.Equal(t, c.wantContent, string(content), "unexpected content in downloaded file")
		})
	}
}

// initMockRunner implements helmexec.Runner for testing with configurable behavior.
type initMockRunner struct {
	// executeFunc is called for each Execute call. If nil, returns empty output and no error.
	executeFunc func(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error)
}

func (m *initMockRunner) Execute(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
	if m.executeFunc != nil {
		return m.executeFunc(cmd, args, env, enableLiveOutput)
	}
	return []byte{}, nil
}

func (m *initMockRunner) ExecuteStdIn(cmd string, args []string, env map[string]string, stdin io.Reader) ([]byte, error) {
	return []byte{}, nil
}

// mockInitConfigProvider implements InitConfigProvider for testing.
type mockInitConfigProvider struct {
	force bool
}

func (m *mockInitConfigProvider) Force() bool {
	return m.force
}

func newTestLogger() *zap.SugaredLogger {
	cfg := zapcore.EncoderConfig{MessageKey: "message"}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.AddSync(io.Discard),
		zapcore.DebugLevel,
	)
	return zap.New(core).Sugar()
}

// createPluginYAML creates a plugin.yaml in a temp plugins directory.
func createPluginYAML(t *testing.T, pluginsDir, pluginDirName, name, version string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, pluginDirName)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := fmt.Sprintf("name: %s\nversion: %s\n", name, version)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(content), 0o644))
}

// newHelmPluginMockRunner creates a mock runner that returns a valid helm version
// and fails all "helm plugin" subcommands with the given error.
func newHelmPluginMockRunner(pluginErr error) *initMockRunner {
	return &initMockRunner{
		executeFunc: func(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
			for _, a := range args {
				if a == "--short" {
					return []byte("v3.18.6"), nil
				}
			}
			// Fail any "helm plugin ..." subcommand (install, update, etc.)
			if len(args) > 0 && args[0] == "plugin" {
				return nil, pluginErr
			}
			return []byte{}, nil
		},
	}
}

func TestCheckHelmPlugins_InstallErrorButPluginPresent(t *testing.T) {
	pluginsDir := t.TempDir()
	t.Setenv("HELM_PLUGINS", pluginsDir)

	// Do NOT pre-populate plugins — the directory starts empty so
	// GetPluginVersion returns "not installed" and the install path is triggered.
	// The mock runner simulates the Windows scenario where "helm plugin install"
	// places the binary but the post-install script fails: it creates the
	// plugin.yaml on disk and then returns an error.
	runner := &initMockRunner{
		executeFunc: func(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
			for _, a := range args {
				if a == "--short" {
					return []byte("v3.18.6"), nil
				}
			}
			if len(args) > 0 && args[0] == "plugin" && len(args) >= 3 && args[1] == "install" {
				// Find which plugin is being installed by matching the repo URL.
				repo := args[2]
				for _, p := range helmPlugins {
					if p.repo == repo {
						createPluginYAML(t, pluginsDir, p.name, p.name, strings.TrimPrefix(p.version, "v"))
						break
					}
				}
				return nil, helmexec.ExitError{Message: "sh: not found", Code: 1}
			}
			return []byte{}, nil
		},
	}

	h := NewHelmfileInit("helm", &mockInitConfigProvider{force: true}, newTestLogger(), runner)
	err := h.CheckHelmPlugins()
	// Should succeed because plugins are present despite install errors
	assert.NoError(t, err)
}

func TestCheckHelmPlugins_InstallErrorPluginTrulyMissing(t *testing.T) {
	pluginsDir := t.TempDir()
	t.Setenv("HELM_PLUGINS", pluginsDir)

	// Don't create any plugin files — the plugins directory is empty.

	runner := newHelmPluginMockRunner(helmexec.ExitError{Message: "sh: not found", Code: 1})

	h := NewHelmfileInit("helm", &mockInitConfigProvider{force: true}, newTestLogger(), runner)
	err := h.CheckHelmPlugins()
	// Should fail because plugin is truly not installed
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sh: not found")
}
