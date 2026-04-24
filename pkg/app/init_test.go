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

func TestCheckHelmPlugins_UpdateFailsFallbackToReinstall(t *testing.T) {
	pluginsDir := t.TempDir()
	t.Setenv("HELM_PLUGINS", pluginsDir)

	// Pre-populate plugins with outdated versions so the update path is triggered.
	for _, p := range helmPlugins {
		createPluginYAML(t, pluginsDir, p.name, p.name, "0.0.1")
	}

	// Track which plugin sub-commands were executed.
	var calledOps []string

	// The mock runner simulates "helm plugin update" failing and falling back to
	// "helm plugin uninstall" + "helm plugin install" which succeeds and writes the
	// required version to disk.
	runner := &initMockRunner{
		executeFunc: func(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
			for _, a := range args {
				if a == "--short" {
					return []byte("v3.18.6"), nil
				}
			}
			if len(args) >= 2 && args[0] == "plugin" {
				switch args[1] {
				case "update":
					calledOps = append(calledOps, "update:"+args[2])
					// Simulate helm plugin update failing (as can happen with Helm 4)
					return nil, helmexec.ExitError{Message: "plugin update failed", Code: 1}
				case "uninstall":
					calledOps = append(calledOps, "uninstall:"+args[2])
					// Simulate successful uninstall
					return []byte{}, nil
				case "install":
					// Find which plugin is being installed by matching the repo URL.
					if len(args) >= 3 {
						repo := args[2]
						for _, p := range helmPlugins {
							if p.repo == repo {
								calledOps = append(calledOps, "install:"+p.name)
								createPluginYAML(t, pluginsDir, p.name, p.name, strings.TrimPrefix(p.version, "v"))
								break
							}
						}
					}
					return []byte{}, nil
				}
			}
			return []byte{}, nil
		},
	}

	h := NewHelmfileInit("helm", &mockInitConfigProvider{force: true}, newTestLogger(), runner)
	err := h.CheckHelmPlugins()
	// Should succeed: update failed but fallback reinstall updated the plugin
	assert.NoError(t, err)

	// Verify that for each plugin the fallback path was taken:
	// update was attempted, then uninstall + install were called.
	for _, p := range helmPlugins {
		assert.Contains(t, calledOps, "update:"+p.name, "expected update to be attempted for plugin %s", p.name)
		assert.Contains(t, calledOps, "uninstall:"+p.name, "expected uninstall to be called for plugin %s", p.name)
		assert.Contains(t, calledOps, "install:"+p.name, "expected install to be called for plugin %s", p.name)
	}
}

func TestCheckHelmPlugins_UpdateErrorButPluginAtRequiredVersion(t *testing.T) {
	pluginsDir := t.TempDir()
	t.Setenv("HELM_PLUGINS", pluginsDir)

	// Pre-populate plugins with outdated versions so the update path is triggered.
	for _, p := range helmPlugins {
		createPluginYAML(t, pluginsDir, p.name, p.name, "0.0.1")
	}

	// The mock runner simulates:
	// 1. "helm plugin update" failing
	// 2. "helm plugin uninstall" succeeding
	// 3. "helm plugin install" writing the correct version but returning an error
	//    (e.g., post-install script error on Windows)
	// In this case, UpdatePlugin returns the install error, but CheckHelmPlugins
	// verifies the version and warns instead of returning an error.
	runner := &initMockRunner{
		executeFunc: func(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
			for _, a := range args {
				if a == "--short" {
					return []byte("v3.18.6"), nil
				}
			}
			if len(args) >= 2 && args[0] == "plugin" {
				switch args[1] {
				case "update":
					return nil, helmexec.ExitError{Message: "plugin update failed", Code: 1}
				case "uninstall":
					return []byte{}, nil
				case "install":
					// Write the correct version to disk, then return an error
					// (simulates post-install script failure on Windows)
					if len(args) >= 3 {
						repo := args[2]
						for _, p := range helmPlugins {
							if p.repo == repo {
								createPluginYAML(t, pluginsDir, p.name, p.name, strings.TrimPrefix(p.version, "v"))
								break
							}
						}
					}
					return nil, helmexec.ExitError{Message: "post-install script failed", Code: 1}
				}
			}
			return []byte{}, nil
		},
	}

	h := NewHelmfileInit("helm", &mockInitConfigProvider{force: true}, newTestLogger(), runner)
	err := h.CheckHelmPlugins()
	// Should succeed: UpdatePlugin returned an error (from the fallback install step),
	// but the plugin is present at the required version, so CheckHelmPlugins warns and continues.
	assert.NoError(t, err)
}
