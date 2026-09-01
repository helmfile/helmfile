package state

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
)

// issue2616HelmfileContent defines two releases: one that prepares fine
// (grafana) and one that always fails chart preparation (error), because OCI
// charts do not support the "latest" version tag.
var issue2616HelmfileContent = []byte(`
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

// issue2616PrepareCharts runs PrepareCharts for issue2616HelmfileContent.
func issue2616PrepareCharts(t *testing.T, opts ChartPrepareOptions) (map[PrepareChartKey]string, map[PrepareChartKey]error, []error) {
	t.Helper()

	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()

	st, err := createFromYaml(issue2616HelmfileContent, "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	helm := &exectest.Helm{
		ChartsMutex: &sync.Mutex{},
	}

	// OutputDirTemplate includes .OutputDir so charts go into tempDir (auto-cleaned
	// by t.TempDir), not the global cache or CWD.
	opts.OutputDirTemplate = "{{ .OutputDir }}/{{ .Release.Name }}"

	releaseToChart, failedReleases, errs := st.PrepareCharts(helm, tempDir, 1, "apply", opts)
	return releaseToChart, failedReleases, errs
}

// TestAllowFailedReleasesFlag_Enabled verifies that the AllowFailedReleases flag
// works as intended with one release that should work and another release, that
// will fail: the healthy release is prepared and reported in the partial
// results, while the failing one is recorded in failedReleases so that callers
// can skip it during execution.
func TestAllowFailedReleasesFlag_Enabled(t *testing.T) {
	releaseToChart, failedReleases, errs := issue2616PrepareCharts(t, ChartPrepareOptions{
		Concurrency:         1,
		AllowFailedReleases: true,
	})

	require.NotEmpty(t, errs, "PrepareCharts should return an error")

	// Verify only the error chart is not prepared
	assert.Contains(t, releaseToChart, PrepareChartKey{Name: "grafana", Namespace: "grafana"},
		"grafana chart should be prepared")
	assert.NotContains(t, releaseToChart, PrepareChartKey{Name: "error", Namespace: "failed"},
		"error chart should not be prepared")

	// Verify the failed release is attributed per-release, so that the caller
	// can skip it instead of executing it against the un-prepared chart
	assert.NotContains(t, failedReleases, PrepareChartKey{Name: "grafana", Namespace: "grafana"},
		"grafana should not be marked as failed")
	failedKey := PrepareChartKey{Name: "error", Namespace: "failed"}
	assert.Contains(t, failedReleases, failedKey, "the failing release should be marked as failed")
	assert.Error(t, failedReleases[failedKey])
}

// TestAllowFailedReleasesFlag_Disabled verifies that the AllowFailedReleases flag
// has no unintended side effects and the default abort on error still works.
func TestAllowFailedReleasesFlag_Disabled(t *testing.T) {
	releaseToChart, failedReleases, errs := issue2616PrepareCharts(t, ChartPrepareOptions{
		Concurrency: 1,
	})

	require.NotEmpty(t, errs, "PrepareCharts should return an error")

	// Verify no partial results are returned on the default abort-on-error path
	assert.Nil(t, releaseToChart)
	assert.Nil(t, failedReleases)
}
