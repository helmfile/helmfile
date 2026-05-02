package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockCreateConfigProvider is a test double for CreateConfigProvider.
type mockCreateConfigProvider struct {
	name      string
	outputDir string
	force     bool
	logger    *zap.SugaredLogger
}

func (m *mockCreateConfigProvider) Name() string      { return m.name }
func (m *mockCreateConfigProvider) OutputDir() string { return m.outputDir }
func (m *mockCreateConfigProvider) Force() bool       { return m.force }
func (m *mockCreateConfigProvider) Logger() *zap.SugaredLogger {
	if m.logger != nil {
		return m.logger
	}
	return newTestLogger()
}

func newMockCreateConfig(outputDir string, force bool) *mockCreateConfigProvider {
	return &mockCreateConfigProvider{outputDir: outputDir, force: force}
}

func TestCreate_NewDirectory(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "myproject")

	a := &App{}
	cfg := newMockCreateConfig(outDir, false)

	require.NoError(t, a.Create(cfg))

	// Verify all scaffold files were created.
	assertFileContent(t, filepath.Join(outDir, "helmfile.yaml"), helmfileYAMLTemplate)
	assertFileContent(t, filepath.Join(outDir, "environments", "default.yaml"), envDefaultYAMLTemplate)
	assertFileExists(t, filepath.Join(outDir, "values", ".gitkeep"))
}

func TestCreate_CurrentDirectory(t *testing.T) {
	dir := t.TempDir()

	a := &App{}
	cfg := newMockCreateConfig(dir, false)

	require.NoError(t, a.Create(cfg))

	assertFileContent(t, filepath.Join(dir, "helmfile.yaml"), helmfileYAMLTemplate)
	assertFileContent(t, filepath.Join(dir, "environments", "default.yaml"), envDefaultYAMLTemplate)
	assertFileExists(t, filepath.Join(dir, "values", ".gitkeep"))
}

func TestCreate_ExistingHelmfileYAMLNoForce(t *testing.T) {
	dir := t.TempDir()
	// Pre-create helmfile.yaml
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helmfile.yaml"), []byte("existing"), 0o644))

	a := &App{}
	cfg := newMockCreateConfig(dir, false)

	err := a.Create(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exist")
	assert.Contains(t, err.Error(), "--force")

	// Verify the existing file was not overwritten.
	content, readErr := os.ReadFile(filepath.Join(dir, "helmfile.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "existing", string(content))
}

func TestCreate_ExistingEnvDefaultYAMLNoForce(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	require.NoError(t, os.MkdirAll(envDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "default.yaml"), []byte("existing"), 0o644))

	a := &App{}
	cfg := newMockCreateConfig(dir, false)

	err := a.Create(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exist")
	assert.Contains(t, err.Error(), "--force")

	// Verify the existing file was not overwritten.
	content, readErr := os.ReadFile(filepath.Join(envDir, "default.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "existing", string(content))
}

func TestCreate_ExistingGitkeepNoForce(t *testing.T) {
	dir := t.TempDir()
	valuesDir := filepath.Join(dir, "values")
	require.NoError(t, os.MkdirAll(valuesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(valuesDir, ".gitkeep"), []byte("existing"), 0o644))

	a := &App{}
	cfg := newMockCreateConfig(dir, false)

	err := a.Create(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exist")
	assert.Contains(t, err.Error(), "--force")

	// Verify the existing file was not overwritten.
	content, readErr := os.ReadFile(filepath.Join(valuesDir, ".gitkeep"))
	require.NoError(t, readErr)
	assert.Equal(t, "existing", string(content))
}

// TestCreate_PreflightAtomicOnLaterConflict verifies that when only a later
// scaffold file exists (e.g. environments/default.yaml but not helmfile.yaml),
// the preflight check catches it and no files are written at all.
func TestCreate_PreflightAtomicOnLaterConflict(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	require.NoError(t, os.MkdirAll(envDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "default.yaml"), []byte("existing"), 0o644))

	a := &App{}
	cfg := newMockCreateConfig(dir, false)

	err := a.Create(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exist")

	// helmfile.yaml must NOT have been created (preflight aborted before any write).
	_, statErr := os.Stat(filepath.Join(dir, "helmfile.yaml"))
	assert.True(t, os.IsNotExist(statErr), "helmfile.yaml should not have been created")
}

func TestCreate_ExistingFilesWithForce(t *testing.T) {
	dir := t.TempDir()

	// Pre-create all scaffold files with different content.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helmfile.yaml"), []byte("old"), 0o644))
	envDir := filepath.Join(dir, "environments")
	require.NoError(t, os.MkdirAll(envDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "default.yaml"), []byte("old"), 0o644))
	valuesDir := filepath.Join(dir, "values")
	require.NoError(t, os.MkdirAll(valuesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(valuesDir, ".gitkeep"), []byte("old"), 0o644))

	a := &App{}
	cfg := newMockCreateConfig(dir, true)

	require.NoError(t, a.Create(cfg))

	// Verify scaffold files were overwritten with the template content.
	assertFileContent(t, filepath.Join(dir, "helmfile.yaml"), helmfileYAMLTemplate)
	assertFileContent(t, filepath.Join(dir, "environments", "default.yaml"), envDefaultYAMLTemplate)
	assertFileExists(t, filepath.Join(dir, "values", ".gitkeep"))
}

// assertFileContent asserts that the file at path exists and contains wantContent.
func assertFileContent(t *testing.T, path, wantContent string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err, "file %s should exist", path)
	assert.Equal(t, wantContent, string(content))
}

// assertFileExists asserts that the file at path exists.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.NoError(t, err, "file %s should exist", path)
}
