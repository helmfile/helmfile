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
