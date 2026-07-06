package state

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
)

// TestAllowFailedReleasesFlag_Enabled verifies that the AllowFailedReleases flag
// works as intended with one release that should work and another release, that
// will fail.
// The test makes one release fail by having an oci with latest version, which will always error
func TestAllowFailedReleasesFlag_Enabled(t *testing.T) {
	resetChartCacheForTest()
	helmfileContent := []byte(`
repositories:
- name: stable
  url: kubernetes-charts.storage.googleapis.com
  oci: true

releases:
  - name: grafana
    namespace: grafana
    chart: stable/grafana
  - name: error
    namespace: failed
    chart: oci://example.com/chart/error
    version: latest
`)

	logger := zap.NewExample().Sugar()

	st, err := createFromYaml(helmfileContent, "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	helm := &exectest.Helm{
		ChartsMutex: &sync.Mutex{},
	}

	// OutputDirTemplate includes .OutputDir so charts go into tempDir (auto-cleaned
	// by t.TempDir), not the global cache or CWD.
	opts := ChartPrepareOptions{
		Concurrency:         1,
		OutputDirTemplate:   "{{ .OutputDir }}/{{ .Release.Name }}",
		AllowFailedReleases: true,
	}

	releaseToChart, errs := st.PrepareCharts(helm, tempDir, 1, "apply", opts)
	require.NotEmpty(t, errs, "PrepareCharts should return an error")

	// Verify only the error chart is not prepared
	assert.Contains(t, releaseToChart, PrepareChartKey{Name: "grafana", Namespace: "grafana"},
		"grafana chart should be prepared")
	assert.NotContains(t, releaseToChart, PrepareChartKey{Name: "error", Namespace: "failed"},
		"error chart should not be prepared")
}

// TestAllowFailedReleasesFlag_Disabled verifies that the AllowFailedReleases flag
// has no unintended side effects and the default abort on error still works.
// The test makes one release fail by having an oci with latest version, which will always error
func TestAllowFailedReleasesFlag_Disabled(t *testing.T) {
	resetChartCacheForTest()
	cleanupLeakedChartDirs(t, "grafana", "error")
	helmfileContent := []byte(`
repositories:
- name: stable
  url: kubernetes-charts.storage.googleapis.com
  oci: true

releases:
  - name: grafana
    namespace: grafana
    chart: stable/grafana
  - name: error
    namespace: failed
    chart: oci://example.com/chart/error
    version: latest
`)

	logger := zap.NewExample().Sugar()

	st, err := createFromYaml(helmfileContent, "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	helm := &exectest.Helm{
		ChartsMutex: &sync.Mutex{},
	}

	// OutputDirTemplate includes .OutputDir so charts go into tempDir (auto-cleaned
	// by t.TempDir), not the global cache or CWD.
	opts := ChartPrepareOptions{
		Concurrency:       1,
		OutputDirTemplate: "{{ .OutputDir }}/{{ .Release.Name }}",
		// PrefetchSharedRemoteCharts: true,
	}

	releaseToChart, errs := st.PrepareCharts(helm, tempDir, 1, "apply", opts)
	require.NotEmpty(t, errs, "PrepareCharts should return an error")

	// Verify both releases are not prepared
	assert.NotContains(t, releaseToChart, PrepareChartKey{Name: "grafana", Namespace: "grafana"},
		"grafana chart should not be prepared")
	assert.NotContains(t, releaseToChart, PrepareChartKey{Name: "error", Namespace: "failed"},
		"error chart should not be prepared")
}
