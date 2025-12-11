package state

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/helmfile/vals"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/stretchr/testify/require"
)

func TestIssue1757(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "issue1757")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create the chart directory structure
	chartDir := filepath.Join(tmpDir, "chart")
	err = os.MkdirAll(filepath.Join(chartDir, "templates"), 0755)
	require.NoError(t, err)

	// Write Chart.yaml
	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`apiVersion: v2
type: application
name: test
version: 0.1.0
`), 0644)
	require.NoError(t, err)

	// Write values.yaml (enabled: false to produce empty output)
	err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(`enabled: false
`), 0644)
	require.NoError(t, err)

	// Write templates/configmap.yaml
	err = os.WriteFile(filepath.Join(chartDir, "templates/configmap.yaml"), []byte(`{{- if .Values.enabled }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
{{- end }}
`), 0644)
	require.NoError(t, err)

	// Setup HelmState
	logger := helmexec.NewLogger(os.Stdout, "debug")
	fs := filesystem.DefaultFileSystem()
	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	require.NoError(t, err)
	
	st := &HelmState{
		basePath: tmpDir,
		logger:   logger,
		fs:       fs,
		ReleaseSetSpec: ReleaseSetSpec{
			DefaultHelmBinary: "helm",
		},
		RenderedValues: map[string]any{},
		valsRuntime:    valsRuntime,
	}

	// Define the release with transformers
	release := &ReleaseSpec{
		Name:      "test",
		Chart:     filepath.Join(tmpDir, "chart"),
		Namespace: "default",
		Transformers: []any{
			map[string]any{
				"apiVersion": "builtin",
				"kind":       "LabelTransformer",
				"metadata": map[string]any{
					"name": "unused",
				},
				"labels": map[string]any{
					"foo": "bar",
				},
				"fieldSpecs": []any{
					map[string]any{
						"path":   "spec/groups/rules/labels",
						"create": true,
					},
				},
			},
		},
	}

	// We need a helm executor
	runner := helmexec.ShellRunner{
		Logger: logger,
		Ctx:    context.Background(),
	}
	helm, err := helmexec.New("helm", helmexec.HelmExecOptions{}, logger, "", "", runner)
	require.NoError(t, err)

	// Call PrepareCharts which triggers the chartification process
	opts := ChartPrepareOptions{
		OutputDir: tmpDir,
	}
	
	// We need to add the release to the state
	st.Releases = []ReleaseSpec{*release}

	_, errs := st.PrepareCharts(helm, tmpDir, 1, "template", opts)
	
	require.Empty(t, errs, "PrepareCharts should not return errors")
}

func TestIssue1757_JSONPatches(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "issue1757_json")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create the chart directory structure
	chartDir := filepath.Join(tmpDir, "chart")
	err = os.MkdirAll(filepath.Join(chartDir, "templates"), 0755)
	require.NoError(t, err)

	// Write Chart.yaml
	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`apiVersion: v2
type: application
name: test
version: 0.1.0
`), 0644)
	require.NoError(t, err)

	// Write values.yaml (enabled: false to produce empty output)
	err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(`enabled: false
`), 0644)
	require.NoError(t, err)

	// Write templates/configmap.yaml
	err = os.WriteFile(filepath.Join(chartDir, "templates/configmap.yaml"), []byte(`{{- if .Values.enabled }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
{{- end }}
`), 0644)
	require.NoError(t, err)

	// Setup HelmState
	logger := helmexec.NewLogger(os.Stdout, "debug")
	fs := filesystem.DefaultFileSystem()
	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	require.NoError(t, err)
	
	st := &HelmState{
		basePath: tmpDir,
		logger:   logger,
		fs:       fs,
		ReleaseSetSpec: ReleaseSetSpec{
			DefaultHelmBinary: "helm",
		},
		RenderedValues: map[string]any{},
		valsRuntime:    valsRuntime,
	}

	// Define the release with jsonPatches
	release := &ReleaseSpec{
		Name:      "test",
		Chart:     filepath.Join(tmpDir, "chart"),
		Namespace: "default",
		JSONPatches: []any{
			map[string]any{
				"target": map[string]any{
					"kind": "ConfigMap",
					"name": "test",
				},
				"patch": []any{
					map[string]any{
						"op":    "add",
						"path":  "/metadata/labels/foo",
						"value": "bar",
					},
				},
			},
		},
	}

	// We need a helm executor
	runner := helmexec.ShellRunner{
		Logger: logger,
		Ctx:    context.Background(),
	}
	helm, err := helmexec.New("helm", helmexec.HelmExecOptions{}, logger, "", "", runner)
	require.NoError(t, err)

	// Call PrepareCharts which triggers the chartification process
	opts := ChartPrepareOptions{
		OutputDir: tmpDir,
	}
	
	// We need to add the release to the state
	st.Releases = []ReleaseSpec{*release}

	_, errs := st.PrepareCharts(helm, tmpDir, 1, "template", opts)
	
	require.Empty(t, errs, "PrepareCharts should not return errors with JSONPatches")
}
