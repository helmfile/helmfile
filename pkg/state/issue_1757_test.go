package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/helmfile/chartify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

// TestProcessChartification_EmptyRender is an end-to-end regression test for
// issue #1757: a chart that renders zero resources (e.g. everything gated
// behind a falsy `{{- if .Values.enabled }}`) combined with
// transformers/jsonPatches/strategicMergePatches used to crash helmfile with
//
//	assertion failed: unexpected dir entry "" it must be the abs path to the output directory
//
// because chartify (<= v0.28.1) expects exactly one rendered output directory
// and aborts when it finds none. The initial fix in helmfile (#2724) worked
// around this by string-matching that assertion error and skipping
// chartification entirely. Chartify v0.28.2 (see
// https://github.com/helmfile/chartify/issues/206 and its fix in
// https://github.com/helmfile/chartify/pull/207) now treats an empty render as
// a no-op success internally - it removes the chart's content dirs, cleans up
// Chart.yaml `dependencies` and the lock file, and skips the kustomize step -
// so the helmfile-side workaround was removed. This test guards the resulting
// direct behavior: processChartification must succeed on an empty render and
// hand back a chartified chart that is still usable by subsequent helm
// commands (`helm dep build`-safe Chart.yaml, empty `helm template` output).
//
// It exercises the real processChartification -> chartify.Chartify wiring, so
// it needs real helm and kustomize binaries on PATH and is skipped if either
// is missing. Verified to pass on Linux; skipped on Windows because the
// file:// dependency URL this test needs (to make rewriteChartDependencies
// actually produce a temp copy) hits an unrelated, pre-existing Windows
// path-handling issue in chartify/helm's dependency resolution (a Windows
// drive letter embedded in a file:// URL gets mis-joined onto another path),
// independent of the code path under test here.
func TestProcessChartification_EmptyRender(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: unrelated file:// dependency URL / drive letter handling issue in chartify's helm dependency resolution, not the code path under test; passes on Linux (CI)")
	}
	helmBin := "helm"
	if _, err := exec.LookPath(helmBin); err != nil {
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
	// resources - the exact condition that used to trigger chartify's
	// empty-output assertion.
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
			DefaultHelmBinary:      helmBin,
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

	// The core #1757 regression: no "assertion failed: unexpected dir entry ..."
	// error, chartify (>= v0.28.2) treats the empty render as a no-op.
	require.NoError(t, err)
	assert.True(t, buildDeps, "buildDeps should be true (!skipDeps)")

	// The returned path is chartify's output dir; it must exist and contain a Chart.yaml.
	info, statErr := os.Stat(resultPath)
	require.NoError(t, statErr, "returned chart path %q must still exist after processChartification returns", resultPath)
	assert.True(t, info.IsDir())
	_, err = os.Stat(filepath.Join(resultPath, "Chart.yaml"))
	assert.NoError(t, err, "Chart.yaml should be reachable from the returned path %q", resultPath)

	// On the empty-render path chartify removes the Chart.yaml `dependencies`
	// field (together with charts/ and Chart.lock), so that a subsequent
	// `helm dependency build`/`helm template` on the chartified output does not
	// fail with "found in Chart.yaml, but missing in charts/ directory".
	chartYaml, readErr := os.ReadFile(filepath.Join(resultPath, "Chart.yaml"))
	require.NoError(t, readErr)
	assert.NotContainsf(t, string(chartYaml), "dependencies:",
		"Chart.yaml dependencies field should have been removed after an empty render; got:\n%s", chartYaml)

	// The chartified output must render to (empty) nothing without any further
	// preparation, proving it is directly usable by subsequent helm commands.
	tmplOut, tmplErr := exec.Command(helmBin, "template", release.Name, resultPath).CombinedOutput()
	require.NoErrorf(t, tmplErr, "helm template on chartified output failed: %s", tmplOut)
	assert.Empty(t, strings.TrimSpace(string(tmplOut)), "expected empty render, got:\n%s", tmplOut)

	// processChartification tracked the output dir for deferred cleanup; after
	// cleanup the dir (and its parent temp dir) must be gone.
	st.CleanupChartifyTempDirs()
	_, statErr = os.Stat(resultPath)
	assert.True(t, os.IsNotExist(statErr), "chartify temp dir %q should have been removed by CleanupChartifyTempDirs", resultPath)
}
