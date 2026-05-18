package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/runtime"
	"github.com/helmfile/helmfile/pkg/yaml"
)

func TestRewriteChartDependencies(t *testing.T) {
	tests := []struct {
		name           string
		chartYaml      string
		expectModified bool
		expectError    bool
		validate       func(t *testing.T, modifiedChartYaml string)
	}{
		{
			name:           "no Chart.yaml exists",
			chartYaml:      "",
			expectModified: false,
			expectError:    false,
		},
		{
			name: "no dependencies",
			chartYaml: `apiVersion: v2
name: test-chart
version: 1.0.0
`,
			expectModified: false,
			expectError:    false,
		},
		{
			name: "absolute file:// dependency - not modified",
			chartYaml: `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: file:///absolute/path/to/chart
    version: 1.0.0
`,
			expectModified: false,
			expectError:    false,
			validate: func(t *testing.T, modifiedChartYaml string) {
				if !strings.Contains(modifiedChartYaml, "file:///absolute/path/to/chart") {
					t.Errorf("absolute path should not be modified")
				}
			},
		},
		{
			name: "relative file:// dependency - should be modified",
			chartYaml: `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: file://../relative-chart
    version: 1.0.0
`,
			expectModified: true,
			expectError:    false,
			validate: func(t *testing.T, modifiedChartYaml string) {
				if strings.Contains(modifiedChartYaml, "file://../relative-chart") {
					t.Errorf("relative path should have been converted to absolute")
				}

				if !strings.Contains(modifiedChartYaml, "file://") {
					t.Errorf("should still have file:// prefix")
				}
			},
		},
		{
			name: "mixed dependencies - only relative file:// modified",
			chartYaml: `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: https://charts.example.com
    version: 1.0.0
  - name: dep2
    repository: file://../relative-chart
    version: 2.0.0
  - name: dep3
    repository: file:///absolute/chart
    version: 3.0.0
  - name: dep4
    repository: oci://registry.example.com/charts/mychart
    version: 4.0.0
`,
			expectModified: true,
			expectError:    false,
			validate: func(t *testing.T, modifiedChartYaml string) {
				if !strings.Contains(modifiedChartYaml, "https://charts.example.com") {
					t.Errorf("https repository should not be modified")
				}

				if strings.Contains(modifiedChartYaml, "file://../relative-chart") {
					t.Errorf("relative file:// path should have been converted")
				}

				if !strings.Contains(modifiedChartYaml, "file:///absolute/chart") {
					t.Errorf("absolute file:// path should not be modified")
				}

				if !strings.Contains(modifiedChartYaml, "oci://registry.example.com") {
					t.Errorf("oci repository should not be modified")
				}
			},
		},
		{
			name: "multiple relative dependencies",
			chartYaml: `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: file://../chart1
    version: 1.0.0
  - name: dep2
    repository: file://./chart2
    version: 2.0.0
  - name: dep3
    repository: file://../../../chart3
    version: 3.0.0
`,
			expectModified: true,
			expectError:    false,
			validate: func(t *testing.T, modifiedChartYaml string) {
				if strings.Contains(modifiedChartYaml, "file://../chart1") ||
					strings.Contains(modifiedChartYaml, "file://./chart2") ||
					strings.Contains(modifiedChartYaml, "file://../../../chart3") {
					t.Errorf("all relative paths should have been converted")
				}
			},
		},
		{
			name: "extra fields",
			chartYaml: `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: https://charts.example.com
    version: 1.0.0
    condition: dep.install
    import-values:
    - child: persistence
      parent: global.persistence
    extra-field2: bbb
extra-field: aaa
`,
			expectModified: false,
			expectError:    false,
			validate: func(t *testing.T, modifiedChartYaml string) {
				requiredFields := []string{
					"condition: dep.install",
					"import-values:",
					"child: persistence",
					"parent: global.persistence",
					"extra-field2: bbb",
					"extra-field: aaa",
				}

				for _, field := range requiredFields {
					if !strings.Contains(modifiedChartYaml, field) {
						t.Errorf("field %q should be preserved", field)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "helmfile-test-")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			if tt.chartYaml != "" {
				chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
				if err := os.WriteFile(chartYamlPath, []byte(tt.chartYaml), 0644); err != nil {
					t.Fatalf("failed to write Chart.yaml: %v", err)
				}
			}

			logger := zap.NewNop().Sugar()
			st := &HelmState{
				logger: logger,
				fs:     filesystem.DefaultFileSystem(),
			}

			rewrittenPath, cleanup, err := st.rewriteChartDependencies(tempDir)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if tt.expectModified {
				if rewrittenPath == tempDir {
					t.Errorf("expected rewrittenPath != tempDir when modifications are needed, got same path")
				}
			} else {
				if rewrittenPath != tempDir {
					t.Errorf("expected rewrittenPath == tempDir when no modifications are needed, got %q", rewrittenPath)
				}
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer cleanup()

			if tt.chartYaml == "" {
				return
			}

			modifiedChartBytes, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.yaml"))
			if err != nil {
				t.Fatalf("failed to read Chart.yaml: %v", err)
			}
			modifiedChartYaml := string(modifiedChartBytes)

			if tt.validate != nil {
				tt.validate(t, modifiedChartYaml)
			}
		})
	}
}

func TestRewriteChartDependencies_OriginalNotModified(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "helmfile-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalChart := `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: file://../relative-chart
    version: 1.0.0
`

	chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
	if err := os.WriteFile(chartYamlPath, []byte(originalChart), 0644); err != nil {
		t.Fatalf("failed to write Chart.yaml: %v", err)
	}

	logger := zap.NewNop().Sugar()
	st := &HelmState{
		logger: logger,
		fs:     filesystem.DefaultFileSystem(),
	}

	rewrittenPath, cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if rewrittenPath == tempDir {
			return
		}

		cleanup()

		if _, statErr := os.Stat(rewrittenPath); !os.IsNotExist(statErr) {
			t.Errorf("expected rewritten chart path %q to be removed after cleanup", rewrittenPath)
		}
	})

	if rewrittenPath == tempDir {
		t.Errorf("expected a different path when modifications are needed, got same path")
	}

	modifiedContent, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.yaml"))
	if err != nil {
		t.Fatalf("failed to read modified Chart.yaml: %v", err)
	}

	if string(modifiedContent) == originalChart {
		t.Errorf("Chart.yaml in the copy should have been modified")
	}

	originalContent, err := os.ReadFile(chartYamlPath)
	if err != nil {
		t.Fatalf("failed to read original Chart.yaml: %v", err)
	}

	if string(originalContent) != originalChart {
		t.Errorf("original Chart.yaml should not have been modified\nexpected:\n%s\ngot:\n%s",
			originalChart, string(originalContent))
	}
}

func TestRewriteChartDependencies_PreservesOtherFields(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "helmfile-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	chartYaml := `apiVersion: v2
name: test-chart
version: 1.0.0
description: A test chart
keywords:
  - test
  - example
maintainers:
  - name: Test User
    email: test@example.com
dependencies:
  - name: dep1
    repository: file://../relative-chart
    version: 1.0.0
    condition: test
`

	chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
	if err := os.WriteFile(chartYamlPath, []byte(chartYaml), 0644); err != nil {
		t.Fatalf("failed to write Chart.yaml: %v", err)
	}

	logger := zap.NewNop().Sugar()
	st := &HelmState{
		logger: logger,
		fs:     filesystem.DefaultFileSystem(),
	}

	rewrittenPath, cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if rewrittenPath == tempDir {
			return
		}

		cleanup()

		if _, statErr := os.Stat(rewrittenPath); !os.IsNotExist(statErr) {
			t.Errorf("expected rewritten chart path %q to be removed after cleanup", rewrittenPath)
		}
	})

	modifiedContent, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.yaml"))
	if err != nil {
		t.Fatalf("failed to read modified Chart.yaml: %v", err)
	}

	content := string(modifiedContent)

	requiredFields := []string{
		"apiVersion: v2",
		"name: test-chart",
		"version: 1.0.0",
		"description: A test chart",
		"keywords:",
		"maintainers:",
		"condition:",
	}

	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("field %q should be preserved", field)
		}
	}

	if strings.Contains(content, "file://../relative-chart") {
		t.Errorf("relative path should have been converted to absolute")
	}

	originalContent, err := os.ReadFile(chartYamlPath)
	if err != nil {
		t.Fatalf("failed to read original Chart.yaml: %v", err)
	}
	if string(originalContent) != chartYaml {
		t.Errorf("original Chart.yaml should not have been modified")
	}
}

func TestRewriteChartDependencies_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(tempDir string) error
		expectError bool
	}{
		{
			name: "invalid yaml in Chart.yaml",
			setupFunc: func(tempDir string) error {
				invalidYaml := `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: file://../chart
    version: 1.0.0
    invalid yaml here!
`
				return os.WriteFile(filepath.Join(tempDir, "Chart.yaml"), []byte(invalidYaml), 0644)
			},
			expectError: true,
		},
		{
			name: "unreadable Chart.yaml",
			setupFunc: func(tempDir string) error {
				chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
				if err := os.WriteFile(chartYamlPath, []byte("test"), 0000); err != nil {
					return err
				}
				return nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "helmfile-test-")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			if err := tt.setupFunc(tempDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			logger := zap.NewNop().Sugar()
			st := &HelmState{
				logger: logger,
				fs:     filesystem.DefaultFileSystem(),
			}

			_, _, err = st.rewriteChartDependencies(tempDir)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRewriteChartDependencies_WindowsStylePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "helmfile-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	chartYaml := `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: file://./subdir/chart
    version: 1.0.0
`

	chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
	if err := os.WriteFile(chartYamlPath, []byte(chartYaml), 0644); err != nil {
		t.Fatalf("failed to write Chart.yaml: %v", err)
	}

	logger := zap.NewNop().Sugar()
	st := &HelmState{
		logger: logger,
		fs:     filesystem.DefaultFileSystem(),
	}

	rewrittenPath, cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if rewrittenPath == tempDir {
			return
		}

		cleanup()

		if _, statErr := os.Stat(rewrittenPath); !os.IsNotExist(statErr) {
			t.Errorf("expected rewritten chart path %q to be removed after cleanup", rewrittenPath)
		}
	})

	data, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.yaml"))
	if err != nil {
		t.Fatalf("failed to read Chart.yaml: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "file://./subdir/chart") {
		t.Errorf("relative path with ./ should have been converted")
	}
}

func TestRewriteChartDependencies_RaceCondition(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "helmfile-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	chartYaml := `apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    repository: file://../relative-chart
    version: 1.0.0
`

	chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
	if err := os.WriteFile(chartYamlPath, []byte(chartYaml), 0644); err != nil {
		t.Fatalf("failed to write Chart.yaml: %v", err)
	}

	numGoroutines := 10
	var wg sync.WaitGroup
	var readyWg sync.WaitGroup
	errCh := make(chan error, numGoroutines)
	ready := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		readyWg.Add(1)
		go func() {
			defer wg.Done()

			readyWg.Done()
			<-ready

			logger := zap.NewNop().Sugar()
			st := &HelmState{
				logger: logger,
				fs:     filesystem.DefaultFileSystem(),
			}

			rewrittenPath, cleanup, err := st.rewriteChartDependencies(tempDir)
			if err != nil {
				errCh <- err
				return
			}
			defer cleanup()

			data, readErr := os.ReadFile(filepath.Join(rewrittenPath, "Chart.yaml"))
			if readErr != nil {
				errCh <- readErr
				return
			}

			type ChartDependency struct {
				Name       string `yaml:"name"`
				Repository string `yaml:"repository"`
				Version    string `yaml:"version"`
			}
			type ChartMeta struct {
				APIVersion   string            `yaml:"apiVersion"`
				Name         string            `yaml:"name"`
				Version      string            `yaml:"version"`
				Dependencies []ChartDependency `yaml:"dependencies,omitempty"`
			}

			var meta ChartMeta
			if unmarshalErr := yaml.Unmarshal(data, &meta); unmarshalErr != nil {
				errCh <- unmarshalErr
				return
			}

			if meta.Name != "test-chart" {
				errCh <- fmt.Errorf("expected chart name 'test-chart', got %q", meta.Name)
				return
			}
		}()
	}

	readyWg.Wait()
	close(ready)

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("goroutine error: %v", err)
	}

	data, err := os.ReadFile(chartYamlPath)
	if err != nil {
		t.Fatalf("failed to read original Chart.yaml: %v", err)
	}

	type ChartDependency struct {
		Name       string `yaml:"name"`
		Repository string `yaml:"repository"`
		Version    string `yaml:"version"`
	}
	type ChartMeta struct {
		APIVersion   string            `yaml:"apiVersion"`
		Name         string            `yaml:"name"`
		Version      string            `yaml:"version"`
		Dependencies []ChartDependency `yaml:"dependencies,omitempty"`
	}

	var chartMeta ChartMeta
	if err := yaml.Unmarshal(data, &chartMeta); err != nil {
		t.Fatalf("original Chart.yaml is not valid YAML: %v", err)
	}

	if chartMeta.Name != "test-chart" {
		t.Errorf("expected original chart name 'test-chart', got %q", chartMeta.Name)
	}
	if len(chartMeta.Dependencies) == 0 {
		t.Fatalf("expected original Chart.yaml to contain at least one dependency, got %d", len(chartMeta.Dependencies))
	}

	const wantDependencyName = "dep1"
	if chartMeta.Dependencies[0].Name != wantDependencyName {
		t.Errorf("expected first dependency name %q, got %q", wantDependencyName, chartMeta.Dependencies[0].Name)
	}
	const wantRepository = "file://../relative-chart"
	if chartMeta.Dependencies[0].Repository != wantRepository {
		t.Errorf("expected original dependency repository %q, got %q", wantRepository, chartMeta.Dependencies[0].Repository)
	}
}

// TestRewriteChartDependencies_RefreshesChartLock verifies that when Chart.yaml has
// its file:// dependencies rewritten to absolute paths, an existing Chart.lock is
// also updated in the temp copy: the digest is recomputed (otherwise `helm dep
// build` would error with "lock out of sync") and matching file:// repository URLs
// are mirrored over from the rewritten Chart.yaml (otherwise `helm dep build` would
// resolve the lock's relative file:// path against the temp directory and fail).
// Locked versions are preserved verbatim.
func TestRewriteChartDependencies_RefreshesChartLock(t *testing.T) {
	tempDir := t.TempDir()

	chartYaml := `apiVersion: v2
name: parent-chart
version: 1.0.0
dependencies:
  - name: local-dep
    repository: file://../local-dep
    version: 1.0.0
  - name: remote-dep
    repository: https://example.com/charts
    version: "*"
`
	if err := os.WriteFile(filepath.Join(tempDir, "Chart.yaml"), []byte(chartYaml), 0644); err != nil {
		t.Fatalf("writing Chart.yaml: %v", err)
	}

	const originalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	chartLock := `dependencies:
  - name: local-dep
    repository: file://../local-dep
    version: 1.0.0
  - name: remote-dep
    repository: https://example.com/charts
    version: 1.2.3
digest: ` + originalDigest + `
generated: "2024-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(tempDir, "Chart.lock"), []byte(chartLock), 0644); err != nil {
		t.Fatalf("writing Chart.lock: %v", err)
	}

	logger := zap.NewNop().Sugar()
	st := &HelmState{
		basePath: tempDir,
		fs:       filesystem.DefaultFileSystem(),
		logger:   logger,
	}

	rewrittenPath, cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("rewriteChartDependencies failed: %v", err)
	}
	defer cleanup()

	if rewrittenPath == tempDir {
		t.Fatalf("expected a temp copy to be created, got original path %q", rewrittenPath)
	}

	lockData, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.lock"))
	if err != nil {
		t.Fatalf("reading rewritten Chart.lock: %v", err)
	}

	var lock struct {
		Dependencies []struct {
			Name       string `yaml:"name"`
			Repository string `yaml:"repository"`
			Version    string `yaml:"version"`
		} `yaml:"dependencies"`
		Digest    string `yaml:"digest"`
		Generated string `yaml:"generated"`
	}
	if err := yaml.Unmarshal(lockData, &lock); err != nil {
		t.Fatalf("parsing rewritten Chart.lock: %v", err)
	}

	if lock.Digest == originalDigest {
		t.Errorf("expected digest to be recomputed; still %q", lock.Digest)
	}
	if !strings.HasPrefix(lock.Digest, "sha256:") {
		t.Errorf("expected sha256 digest, got %q", lock.Digest)
	}

	if len(lock.Dependencies) != 2 {
		t.Fatalf("expected 2 lock dependencies, got %d", len(lock.Dependencies))
	}

	// The local file:// dependency's repository must be mirrored to the absolute
	// path so `helm dep build` can resolve it from the temp chart directory.
	localDep := lock.Dependencies[0]
	if localDep.Name != "local-dep" {
		t.Fatalf("expected first lock dep name 'local-dep', got %q", localDep.Name)
	}
	if !filepath.IsAbs(strings.TrimPrefix(localDep.Repository, "file://")) {
		t.Errorf("expected local-dep repository to be an absolute file:// path, got %q", localDep.Repository)
	}
	if localDep.Version != "1.0.0" {
		t.Errorf("expected local-dep version preserved as 1.0.0, got %q", localDep.Version)
	}

	// Remote (non-file://) deps must be untouched.
	remoteDep := lock.Dependencies[1]
	if remoteDep.Repository != "https://example.com/charts" {
		t.Errorf("expected remote dep repository unchanged, got %q", remoteDep.Repository)
	}
	if remoteDep.Version != "1.2.3" {
		t.Errorf("expected remote dep version preserved as 1.2.3, got %q", remoteDep.Version)
	}

	// The original Chart.lock on disk must be untouched.
	originalLock, err := os.ReadFile(filepath.Join(tempDir, "Chart.lock"))
	if err != nil {
		t.Fatalf("reading original Chart.lock: %v", err)
	}
	if string(originalLock) != chartLock {
		t.Errorf("original Chart.lock was modified; expected unchanged content")
	}
}

// TestRewriteChartDependencies_RefreshesChartLockWithExtraFields verifies that
// Chart.lock digest recomputation includes all dependency fields (alias, condition,
// tags, import-values, enabled) — not just name/repository/version — so the digest
// stays compatible with Helm's resolver.HashReq for charts using those fields.
// It proves field coverage by running two chart variants under a shared root
// (so file:// paths resolve to the same absolute location) and asserting the
// digests differ only due to extra fields.
func TestRewriteChartDependencies_RefreshesChartLockWithExtraFields(t *testing.T) {
	// Use a shared root so both chart variants resolve file://../local-dep to the
	// same absolute path — isolating the digest difference to field content only.
	sharedRoot := t.TempDir()
	chartDir := filepath.Join(sharedRoot, "parent")
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		t.Fatalf("creating chart dir: %v", err)
	}

	// Run rewriteChartDependencies for a given Chart.yaml and return the recomputed digest.
	getDigest := func(t *testing.T, chartYaml, chartLock string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYaml), 0644); err != nil {
			t.Fatalf("writing Chart.yaml: %v", err)
		}
		if err := os.WriteFile(filepath.Join(chartDir, "Chart.lock"), []byte(chartLock), 0644); err != nil {
			t.Fatalf("writing Chart.lock: %v", err)
		}
		logger := zap.NewNop().Sugar()
		st := &HelmState{
			basePath: chartDir,
			fs:       filesystem.DefaultFileSystem(),
			logger:   logger,
		}
		rewrittenPath, cleanup, err := st.rewriteChartDependencies(chartDir)
		if err != nil {
			t.Fatalf("rewriteChartDependencies failed: %v", err)
		}
		defer cleanup()
		lockData, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.lock"))
		if err != nil {
			t.Fatalf("reading rewritten Chart.lock: %v", err)
		}
		var lock struct {
			Digest string `yaml:"digest"`
		}
		if err := yaml.Unmarshal(lockData, &lock); err != nil {
			t.Fatalf("parsing rewritten Chart.lock: %v", err)
		}
		return lock.Digest
	}

	const originalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	baseLock := `dependencies:
  - name: local-dep
    repository: file://../local-dep
    version: 1.0.0
    alias: my-local
  - name: local-dep
    repository: file://../local-dep-alt
    version: 2.0.0
    alias: my-local-alt
digest: ` + originalDigest + `
generated: "2024-01-01T00:00:00Z"
`

	// Chart.yaml with extra fields (alias, condition, tags, import-values).
	chartYamlWithExtras := `apiVersion: v2
name: parent-chart
version: 1.0.0
dependencies:
  - name: local-dep
    repository: file://../local-dep
    version: 1.0.0
    alias: my-local
    condition: local-dep.enabled
    tags:
      - frontend
      - optional
    import-values:
      - child: config
        parent: global.config
  - name: local-dep
    repository: file://../local-dep-alt
    version: 2.0.0
    alias: my-local-alt
`

	// Same chart without condition/tags/import-values — only alias remains.
	chartYamlWithoutExtras := `apiVersion: v2
name: parent-chart
version: 1.0.0
dependencies:
  - name: local-dep
    repository: file://../local-dep
    version: 1.0.0
    alias: my-local
  - name: local-dep
    repository: file://../local-dep-alt
    version: 2.0.0
    alias: my-local-alt
`

	digestWith := getDigest(t, chartYamlWithExtras, baseLock)
	digestWithout := getDigest(t, chartYamlWithoutExtras, baseLock)

	if !strings.HasPrefix(digestWith, "sha256:") {
		t.Errorf("expected sha256 digest, got %q", digestWith)
	}
	if digestWith == originalDigest {
		t.Errorf("expected digest to be recomputed; still %q", digestWith)
	}
	if digestWith == digestWithout {
		t.Errorf("digest should differ when extra fields (condition, tags, import-values) are present, but both are %q", digestWith)
	}

	// Also verify alias-based matching: both deps have name "local-dep" but
	// different aliases; both should get their file:// paths rewritten.
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYamlWithExtras), 0644); err != nil {
		t.Fatalf("writing Chart.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.lock"), []byte(baseLock), 0644); err != nil {
		t.Fatalf("writing Chart.lock: %v", err)
	}
	logger := zap.NewNop().Sugar()
	st := &HelmState{
		basePath: chartDir,
		fs:       filesystem.DefaultFileSystem(),
		logger:   logger,
	}
	rewrittenPath, cleanup, err := st.rewriteChartDependencies(chartDir)
	if err != nil {
		t.Fatalf("rewriteChartDependencies failed: %v", err)
	}
	defer cleanup()

	lockData, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.lock"))
	if err != nil {
		t.Fatalf("reading rewritten Chart.lock: %v", err)
	}
	var lock struct {
		Dependencies []struct {
			Name       string `yaml:"name"`
			Repository string `yaml:"repository"`
			Version    string `yaml:"version"`
			Alias      string `yaml:"alias"`
		} `yaml:"dependencies"`
	}
	if err := yaml.Unmarshal(lockData, &lock); err != nil {
		t.Fatalf("parsing rewritten Chart.lock: %v", err)
	}
	if len(lock.Dependencies) != 2 {
		t.Fatalf("expected 2 lock dependencies, got %d", len(lock.Dependencies))
	}

	dep1 := lock.Dependencies[0]
	if dep1.Alias != "my-local" {
		t.Errorf("expected first lock dep alias 'my-local', got %q", dep1.Alias)
	}
	if !filepath.IsAbs(strings.TrimPrefix(dep1.Repository, "file://")) {
		t.Errorf("expected first dep repository to be an absolute file:// path, got %q", dep1.Repository)
	}

	dep2 := lock.Dependencies[1]
	if dep2.Alias != "my-local-alt" {
		t.Errorf("expected second lock dep alias 'my-local-alt', got %q", dep2.Alias)
	}
	if !filepath.IsAbs(strings.TrimPrefix(dep2.Repository, "file://")) {
		t.Errorf("expected second dep repository to be an absolute file:// path, got %q", dep2.Repository)
	}
}

// TestRewriteChartDependencies_GoYamlV2ImportValues verifies that Chart.lock
// refresh works under go-yaml v2 (HELMFILE_GO_YAML_V3=false), where nested
// maps in import-values decode as map[interface{}]interface{} which json.Marshal
// cannot handle without normalization.
func TestRewriteChartDependencies_GoYamlV2ImportValues(t *testing.T) {
	prev := runtime.GoYamlV3
	runtime.GoYamlV3 = false
	t.Cleanup(func() {
		runtime.GoYamlV3 = prev
	})

	tempDir := t.TempDir()

	chartYaml := `apiVersion: v2
name: parent-chart
version: 1.0.0
dependencies:
  - name: local-dep
    repository: file://../local-dep
    version: 1.0.0
    import-values:
      - child: config
        parent: global.config
`
	chartLock := `dependencies:
  - name: local-dep
    repository: file://../local-dep
    version: 1.0.0
    import-values:
      - child: config
        parent: global.config
digest: sha256:0000000000000000000000000000000000000000000000000000000000000000
generated: "2024-01-01T00:00:00Z"
`

	if err := os.WriteFile(filepath.Join(tempDir, "Chart.yaml"), []byte(chartYaml), 0644); err != nil {
		t.Fatalf("writing Chart.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "Chart.lock"), []byte(chartLock), 0644); err != nil {
		t.Fatalf("writing Chart.lock: %v", err)
	}

	logger := zap.NewNop().Sugar()
	st := &HelmState{
		basePath: tempDir,
		fs:       filesystem.DefaultFileSystem(),
		logger:   logger,
	}

	rewrittenPath, cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("rewriteChartDependencies failed: %v", err)
	}
	defer cleanup()

	lockData, err := os.ReadFile(filepath.Join(rewrittenPath, "Chart.lock"))
	if err != nil {
		t.Fatalf("reading rewritten Chart.lock: %v", err)
	}

	var lock struct {
		Digest string `yaml:"digest"`
	}
	if err := yaml.Unmarshal(lockData, &lock); err != nil {
		t.Fatalf("parsing rewritten Chart.lock: %v", err)
	}

	if !strings.HasPrefix(lock.Digest, "sha256:") {
		t.Errorf("expected sha256 digest, got %q", lock.Digest)
	}
	const originalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if lock.Digest == originalDigest {
		t.Errorf("expected digest to be recomputed; still %q", lock.Digest)
	}
}
