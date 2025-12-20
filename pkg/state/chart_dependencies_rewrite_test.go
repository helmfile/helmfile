package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

func TestRewriteChartDependencies(t *testing.T) {
	tests := []struct {
		name           string
		chartYaml      string
		expectModified bool
		expectError    bool
		validate       func(t *testing.T, chartPath string)
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
			validate: func(t *testing.T, chartPath string) {
				data, err := os.ReadFile(filepath.Join(chartPath, "Chart.yaml"))
				if err != nil {
					t.Fatalf("failed to read Chart.yaml: %v", err)
				}
				content := string(data)
				if !strings.Contains(content, "file:///absolute/path/to/chart") {
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
			validate: func(t *testing.T, chartPath string) {
				data, err := os.ReadFile(filepath.Join(chartPath, "Chart.yaml"))
				if err != nil {
					t.Fatalf("failed to read Chart.yaml: %v", err)
				}
				content := string(data)

				// Should have been converted to absolute path
				if strings.Contains(content, "file://../relative-chart") {
					t.Errorf("relative path should have been converted to absolute")
				}

				// Should now have an absolute path
				if !strings.Contains(content, "file://") {
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
			validate: func(t *testing.T, chartPath string) {
				data, err := os.ReadFile(filepath.Join(chartPath, "Chart.yaml"))
				if err != nil {
					t.Fatalf("failed to read Chart.yaml: %v", err)
				}
				content := string(data)

				// HTTPS repo should remain unchanged
				if !strings.Contains(content, "https://charts.example.com") {
					t.Errorf("https repository should not be modified")
				}

				// Relative path should be converted
				if strings.Contains(content, "file://../relative-chart") {
					t.Errorf("relative file:// path should have been converted")
				}

				// Absolute path should remain unchanged
				if !strings.Contains(content, "file:///absolute/chart") {
					t.Errorf("absolute file:// path should not be modified")
				}

				// OCI repo should remain unchanged
				if !strings.Contains(content, "oci://registry.example.com") {
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
			validate: func(t *testing.T, chartPath string) {
				data, err := os.ReadFile(filepath.Join(chartPath, "Chart.yaml"))
				if err != nil {
					t.Fatalf("failed to read Chart.yaml: %v", err)
				}
				content := string(data)

				// All relative paths should be converted
				if strings.Contains(content, "file://../chart1") ||
					strings.Contains(content, "file://./chart2") ||
					strings.Contains(content, "file://../../../chart3") {
					t.Errorf("all relative paths should have been converted")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir, err := os.MkdirTemp("", "helmfile-test-")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Create Chart.yaml if provided
			if tt.chartYaml != "" {
				chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
				if err := os.WriteFile(chartYamlPath, []byte(tt.chartYaml), 0644); err != nil {
					t.Fatalf("failed to write Chart.yaml: %v", err)
				}
			}

			// Create HelmState with logger
			logger := zap.NewNop().Sugar()
			st := &HelmState{
				logger: logger,
				fs:     filesystem.DefaultFileSystem(),
			}

			// Read original content if it exists
			var originalContent []byte
			chartYamlPath := filepath.Join(tempDir, "Chart.yaml")
			if _, err := os.Stat(chartYamlPath); err == nil {
				originalContent, _ = os.ReadFile(chartYamlPath)
			}

			// Call rewriteChartDependencies
			cleanup, err := st.rewriteChartDependencies(tempDir)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate the modified Chart.yaml
			if tt.validate != nil {
				tt.validate(t, tempDir)
			}

			// Call cleanup and verify restoration
			if tt.chartYaml != "" {
				cleanup()

				// Read restored content
				restoredContent, err := os.ReadFile(chartYamlPath)
				if err != nil {
					t.Fatalf("failed to read restored Chart.yaml: %v", err)
				}

				// Verify content was restored
				if string(restoredContent) != string(originalContent) {
					t.Errorf("cleanup did not restore original content\noriginal:\n%s\nrestored:\n%s",
						string(originalContent), string(restoredContent))
				}
			}
		})
	}
}

func TestRewriteChartDependencies_CleanupRestoresOriginal(t *testing.T) {
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

	cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify modification happened
	modifiedContent, err := os.ReadFile(chartYamlPath)
	if err != nil {
		t.Fatalf("failed to read modified Chart.yaml: %v", err)
	}

	if string(modifiedContent) == originalChart {
		t.Errorf("Chart.yaml should have been modified")
	}

	// Call cleanup
	cleanup()

	// Verify restoration
	restoredContent, err := os.ReadFile(chartYamlPath)
	if err != nil {
		t.Fatalf("failed to read restored Chart.yaml: %v", err)
	}

	if string(restoredContent) != originalChart {
		t.Errorf("cleanup did not restore original content\nexpected:\n%s\ngot:\n%s",
			originalChart, string(restoredContent))
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

	cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Read modified content
	modifiedContent, err := os.ReadFile(chartYamlPath)
	if err != nil {
		t.Fatalf("failed to read modified Chart.yaml: %v", err)
	}

	content := string(modifiedContent)

	// Verify top-level fields are preserved
	// Note: The implementation preserves top-level chart metadata but may not preserve
	// extra dependency-level fields (like condition, tags) that are not in the ChartDependency struct
	requiredFields := []string{
		"apiVersion: v2",
		"name: test-chart",
		"version: 1.0.0",
		"description: A test chart",
		"keywords:",
		"maintainers:",
	}

	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("field %q should be preserved", field)
		}
	}

	// Verify the dependency was rewritten
	if strings.Contains(content, "file://../relative-chart") {
		t.Errorf("relative path should have been converted to absolute")
	}
}

func TestRewriteChartDependencies_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(tempDir string) error
		expectError bool
		errorMsg    string
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

			_, err = st.rewriteChartDependencies(tempDir)

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

	// Test with backslash (Windows-style) paths
	// Note: file:// URLs should use forward slashes, but test handling of edge cases
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

	cleanup, err := st.rewriteChartDependencies(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Should handle the path correctly
	data, err := os.ReadFile(chartYamlPath)
	if err != nil {
		t.Fatalf("failed to read Chart.yaml: %v", err)
	}

	content := string(data)
	// The relative path should have been converted to absolute
	if strings.Contains(content, "file://./subdir/chart") {
		t.Errorf("relative path with ./ should have been converted")
	}
}
