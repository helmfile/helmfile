package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/testutil"
)

func TestPrintEnv_SingleHelmfile_YAML(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  production:
    kubeContext: prod-cluster
    values:
      - region: us-east-1
        environment: production
        debug: false
---
releases: []
`,
	}

	app := createTestApp(t, files, "production")
	cfg := configImpl{output: "yaml"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	// Parse YAML output
	var result map[string]any
	err = yaml.Unmarshal([]byte(out), &result)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, "production", result["name"])
	assert.Equal(t, "prod-cluster", result["kubeContext"])
	assert.Contains(t, result["filePath"], "helmfile.yaml")

	// Verify values
	values, ok := result["values"].(map[string]any)
	require.True(t, ok, "values should be a map")
	assert.Equal(t, "us-east-1", values["region"])
	assert.Equal(t, "production", values["environment"])
	assert.Equal(t, false, values["debug"])
}

func TestPrintEnv_SingleHelmfile_JSON(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  staging:
    kubeContext: staging-cluster
    values:
      - database:
          host: db.staging.local
          port: 5432
---
releases: []
`,
	}

	app := createTestApp(t, files, "staging")
	cfg := configImpl{output: "json"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	// Parse JSON output (should be an array)
	var results []map[string]any
	err = json.Unmarshal([]byte(out), &results)
	require.NoError(t, err)
	require.Len(t, results, 1, "should have one environment")

	result := results[0]
	assert.Equal(t, "staging", result["name"])
	assert.Equal(t, "staging-cluster", result["kubeContext"])
	assert.Contains(t, result["filePath"], "helmfile.yaml")

	// Verify nested values
	values, ok := result["values"].(map[string]any)
	require.True(t, ok)
	database, ok := values["database"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "db.staging.local", database["host"])
	assert.Equal(t, float64(5432), database["port"]) // JSON numbers are float64
}

func TestPrintEnv_MultipleHelmfiles_YAML(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  dev:
    kubeContext: main-context
    values:
      - source: main
        sharedValue: from-main
---
helmfiles:
  - path: sub/helmfile.yaml
releases: []
`,
		"/path/to/sub/helmfile.yaml": `
environments:
  dev:
    kubeContext: sub-context
    values:
      - source: sub
        subValue: from-sub
---
releases: []
`,
	}

	app := createTestApp(t, files, "dev")
	cfg := configImpl{output: "yaml"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	// Split by --- to get individual documents
	docs := strings.Split(out, "---\n")
	// Filter out empty documents
	var nonEmptyDocs []string
	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed != "" {
			nonEmptyDocs = append(nonEmptyDocs, trimmed)
		}
	}

	assert.GreaterOrEqual(t, len(nonEmptyDocs), 2, "should have at least 2 environment documents")

	// Verify each document is valid YAML
	for i, doc := range nonEmptyDocs {
		var result map[string]any
		err := yaml.Unmarshal([]byte(doc), &result)
		require.NoError(t, err, "document %d should be valid YAML", i)
		assert.Equal(t, "dev", result["name"], "document %d should have correct name", i)
		assert.Contains(t, result, "kubeContext", "document %d should have kubeContext", i)
		assert.Contains(t, result, "values", "document %d should have values", i)
		assert.Contains(t, result, "filePath", "document %d should have filePath", i)
	}
}

func TestPrintEnv_MultipleHelmfiles_JSON(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  test:
    kubeContext: main-kube
    values:
      - mainKey: mainValue
---
helmfiles:
  - path: child1/helmfile.yaml
  - path: child2/helmfile.yaml
releases: []
`,
		"/path/to/child1/helmfile.yaml": `
environments:
  test:
    kubeContext: child1-kube
    values:
      - child1Key: child1Value
---
releases: []
`,
		"/path/to/child2/helmfile.yaml": `
environments:
  test:
    kubeContext: child2-kube
    values:
      - child2Key: child2Value
---
releases: []
`,
	}

	app := createTestApp(t, files, "test")
	cfg := configImpl{output: "json"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	// Parse JSON array
	var results []map[string]any
	err = json.Unmarshal([]byte(out), &results)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 3, "should have at least 3 environments")

	// Verify all have correct structure
	for i, result := range results {
		assert.Equal(t, "test", result["name"], "result %d should have name 'test'", i)
		assert.Contains(t, result, "kubeContext", "result %d should have kubeContext", i)
		assert.Contains(t, result, "values", "result %d should have values", i)
		assert.Contains(t, result, "filePath", "result %d should have filePath", i)
	}
}

func TestPrintEnv_WithDefaults(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  default:
    values:
      - color: blue
  prod:
    values:
      - color: red
        size: large
---
releases: []
`,
	}

	app := createTestApp(t, files, "prod")
	cfg := configImpl{output: "json"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	var results []map[string]any
	err = json.Unmarshal([]byte(out), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)

	values, ok := results[0]["values"].(map[string]any)
	require.True(t, ok)

	// Should have values from prod environment
	assert.Equal(t, "red", values["color"])
	assert.Equal(t, "large", values["size"])
}

func TestPrintEnv_InvalidOutputFormat(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  dev:
    values:
      - test: value
---
releases: []
`,
	}

	app := createTestApp(t, files, "dev")
	cfg := configImpl{output: "xml"} // Invalid format

	_, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output format")
	})
	require.NoError(t, err)
}

func TestPrintEnv_EmptyValues(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  minimal:
    kubeContext: minimal-cluster
---
releases: []
`,
	}

	app := createTestApp(t, files, "minimal")
	cfg := configImpl{output: "json"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	var results []map[string]any
	err = json.Unmarshal([]byte(out), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "minimal", result["name"])
	assert.Equal(t, "minimal-cluster", result["kubeContext"])

	// Values should exist but be empty
	values, ok := result["values"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, values)
}

func TestPrintEnv_NoKubeContext(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  local:
    values:
      - app: myapp
---
releases: []
`,
	}

	app := createTestApp(t, files, "local")
	cfg := configImpl{output: "yaml"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	var result map[string]any
	err = yaml.Unmarshal([]byte(out), &result)
	require.NoError(t, err)

	assert.Equal(t, "local", result["name"])
	// kubeContext should be present but empty
	kubeContext, exists := result["kubeContext"]
	assert.True(t, exists)
	assert.Equal(t, "", kubeContext)
}

func TestPrintEnv_DefaultOutput(t *testing.T) {
	// When output is empty string, should default to YAML
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  dev:
    values:
      - key: value
---
releases: []
`,
	}

	app := createTestApp(t, files, "dev")
	cfg := configImpl{output: ""} // Empty output should default to yaml

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		assert.NoError(t, err)
	})
	require.NoError(t, err)

	// Should be valid YAML
	var result map[string]any
	err = yaml.Unmarshal([]byte(out), &result)
	require.NoError(t, err, "empty output format should default to YAML")
}

func TestPrintEnv_UndefinedEnvironment(t *testing.T) {
	// Test behavior with undefined environment
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  production:
    values:
      - env: prod
---
releases: []
`,
	}

	app := createTestApp(t, files, "staging") // staging is not defined
	cfg := configImpl{output: "json"}

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintEnv(cfg)
		// The behavior depends on helmfile's environment handling
		// It may succeed with empty values or fail
		if err != nil {
			assert.Contains(t, err.Error(), "environment")
		}
	})
	require.NoError(t, err)

	// If no error, output should be valid JSON (potentially empty array)
	if out != "" {
		var results []map[string]any
		err = json.Unmarshal([]byte(out), &results)
		// Should either be valid JSON or empty
		if err != nil {
			assert.Equal(t, "", out, "if not valid JSON, output should be empty")
		}
	}
}

// Helper function to create test app with common setup
func createTestApp(t *testing.T, files map[string]string, environment string) *App {
	t.Helper()

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	require.NoError(t, err)

	var buffer bytes.Buffer
	syncWriter := testhelper.NewSyncWriter(&buffer)
	logger := helmexec.NewLogger(syncWriter, "warn")

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		fs:                  ffs.DefaultFileSystem(),
		OverrideKubeContext: "default",
		Env:                 environment,
		Logger:              logger,
		valsRuntime:         valsRuntime,
	}, files)

	expectNoCallsToHelm(app)

	return app
}
