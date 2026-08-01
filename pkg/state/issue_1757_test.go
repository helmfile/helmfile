package state

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/helmfile/chartify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

// TestIsChartifyEmptyRenderOutputError verifies that isChartifyEmptyRenderOutputError
// only matches chartify's specific "empty rendered output dir" assertion failure, not
// other, unrelated chartify errors. This is a regression test for issue #1757: a chart
// that renders zero resources (e.g. everything gated behind a falsy `if`) combined with
// transformers/jsonPatches/strategicMergePatches used to crash helmfile with
//
//	assertion failed: unexpected dir entry "" it must be the abs path to the output directory
//
// because chartify expects exactly one rendered output directory and found none.
func TestIsChartifyEmptyRenderOutputError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "chartify empty render assertion error",
			err:      errors.New(`assertion failed: unexpected dir entry "" it must be the abs path to the output directory`),
			expected: true,
		},
		{
			name:     "unrelated chartify error",
			err:      errors.New("exec: \"kustomize\": executable file not found in %PATH%"),
			expected: false,
		},
		{
			name:     "unrelated multiple-dir-entries assertion",
			err:      errors.New(`assertion failed: there should be only one dir entry under the helm output dir /tmp/foo`),
			expected: false,
		},
		{
			name:     "helm template failure",
			err:      errors.New("Error: template: mychart/templates/broken.yaml:3:5: executing..."),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isChartifyEmptyRenderOutputError(tt.err))
		})
	}
}

// TestProcessChartification_EmptyRenderReturnsSurvivingPath is an end-to-end
// regression test for a bug found in review of the #1757 fix: the empty-render
// no-op path must return the chart's original path, not the temp copy that
// rewriteChartDependencies creates when the chart has relative file:// deps -
// that temp dir is removed by a deferred cleanup as soon as processChartification
// returns, so returning it would hand the caller a path to a directory that no
// longer exists on disk.
//
// This exercises the real processChartification -> chartify.Chartify wiring
// (unlike TestIsChartifyEmptyRenderOutputError above, which only tests the pure
// string-matching helper), so it needs real helm and kustomize binaries on PATH
// and is skipped if either is missing. Verified to pass on Linux; skipped on
// Windows because the file:// dependency URL this test needs (to make
// rewriteChartDependencies actually produce a temp copy) hits an unrelated,
// pre-existing Windows path-handling issue in chartify/helm's dependency
// resolution (a Windows drive letter embedded in a file:// URL gets
// mis-joined onto another path), independent of the fix under test here.
func TestProcessChartification_EmptyRenderReturnsSurvivingPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: unrelated file:// dependency URL / drive letter handling issue in chartify's helm dependency resolution, not the code path under test; passes on Linux (CI)")
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not found on PATH, skipping")
	}
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not found on PATH, skipping")
	}

	tempDir := t.TempDir()

	// A sibling chart referenced via a relative file:// dependency, so that
	// processChartification takes the rewriteChartDependencies path (line
	// `if st.fs.DirectoryExistsAt(chartPath) { ... }`) and reassigns its local
	// chartPath variable to a temp copy before ever calling chartify.
	depChartDir := filepath.Join(tempDir, "dep-chart")
	require.NoError(t, os.MkdirAll(depChartDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(depChartDir, "Chart.yaml"), []byte(`
apiVersion: v2
name: dep-chart
version: 0.1.0
`), 0644))

	chartDir := filepath.Join(tempDir, "chart")
	require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`
apiVersion: v2
name: emptychart
version: 0.1.0
dependencies:
  - name: dep-chart
    version: 0.1.0
    repository: "file://../dep-chart"
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("enabled: false\n"), 0644))
	// Every template is gated behind a falsy condition, so helm renders zero
	// resources - the exact condition that triggers chartify's empty-output
	// assertion.
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", "deployment.yaml"), []byte(`
{{- if .Values.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
{{- end }}
`), 0644))

	// A transformer is required for chartify to be invoked at all (see the
	// call site in state.go: chartification is only non-nil, and processChartification
	// only gets called, when transformers/jsonPatches/strategicMergePatches are set).
	transformerPath := filepath.Join(tempDir, "transformer.yaml")
	require.NoError(t, os.WriteFile(transformerPath, []byte(`
apiVersion: builtin
kind: AnnotationsTransformer
metadata:
  name: notImportantHere
annotations:
  area: "51"
fieldSpecs:
  - path: metadata/annotations
    create: true
`), 0644))

	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			DefaultHelmBinary:      "helm",
			DefaultKustomizeBinary: "kustomize",
		},
	}

	chartification := &Chartify{
		Opts: &chartify.ChartifyOpts{
			Transformers: []string{transformerPath},
		},
	}
	release := &ReleaseSpec{}
	release.Name = "empty-release"

	resultPath, buildDeps, err := st.processChartification(
		chartification, release, chartDir, ChartPrepareOptions{}, false, "template",
	)

	require.NoError(t, err)
	assert.True(t, buildDeps, "buildDeps should be true (!skipDeps) for the no-op path")

	// The returned path must still exist: it must be the original chart
	// directory, not the deps-rewritten temp copy that gets deleted by
	// rewriteChartDependencies' deferred cleanup on return.
	info, statErr := os.Stat(resultPath)
	require.NoError(t, statErr, "returned chart path %q must still exist after processChartification returns", resultPath)
	assert.True(t, info.IsDir())

	// It should specifically be the chart's own directory (or an
	// equally-valid, not-yet-cleaned-up path to the same chart), not some
	// other temp directory. Chart.yaml must be readable from it.
	_, err = os.Stat(filepath.Join(resultPath, "Chart.yaml"))
	assert.NoError(t, err, "Chart.yaml should be reachable from the returned path %q", resultPath)
}
