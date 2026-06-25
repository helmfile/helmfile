package state

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
)

// TestIssue923_OCIChartPreparedForNeededRelease verifies that OCI charts are
// prepared (pulled) for releases that are included via --include-needs.
// See https://github.com/helmfile/helmfile/issues/923
func TestIssue923_OCIChartPreparedForNeededRelease(t *testing.T) {
	resetChartCacheForTest()
	helmfileContent := []byte(`
repositories:
  - name: huma
    url: ghcr.io/huma-engineering/helm-charts
    oci: true
  - name: argo
    url: https://argoproj.github.io/argo-helm

releases:
  - name: argocd
    namespace: argocd
    chart: argo/argo-cd
    labels:
      app: argocd
    needs:
      - argocd/argocd-secrets
    version: ~5.37.1

  - name: argocd-secrets
    namespace: argocd
    chart: huma/external-secret-resources
    labels:
      app: secrets
    version: ~0.1.0
`)

	logger := zap.NewExample().Sugar()

	st, err := createFromYaml(helmfileContent, "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	// Simulate: helmfile -l app=argocd --include-needs
	st.Selectors = []string{"app=argocd"}

	tempDir := t.TempDir()

	helm := &exectest.Helm{
		Helm3:       true,
		ChartsMutex: &sync.Mutex{},
	}

	// PrepareCharts uses opts.IncludeTransitiveNeeds to determine which releases
	// to prepare charts for. When --include-needs is set, this should be true,
	// matching c.IncludeNeeds() in the app layer.
	// OutputDirTemplate is set to force charts into tempDir (not the global cache).
	opts := ChartPrepareOptions{
		SkipResolve:            true,
		IncludeTransitiveNeeds: true,
		Concurrency:            1,
		OutputDirTemplate:      "{{ .Release.Name }}",
	}

	releaseToChart, errs := st.PrepareCharts(helm, tempDir, 1, "apply", opts)
	require.Empty(t, errs, "PrepareCharts should not return errors")

	// Verify both releases have prepared charts
	assert.Contains(t, releaseToChart, PrepareChartKey{Name: "argocd", Namespace: "argocd"},
		"argocd chart should be prepared")
	assert.Contains(t, releaseToChart, PrepareChartKey{Name: "argocd-secrets", Namespace: "argocd"},
		"argocd-secrets (needed OCI chart) should be prepared")

	// The OCI chart path should be a local path (pulled by ChartPull), not the
	// remote chart name. This confirms the OCI chart was actually prepared.
	chartPath := releaseToChart[PrepareChartKey{Name: "argocd-secrets", Namespace: "argocd"}]
	assert.NotEqual(t, "huma/external-secret-resources", chartPath,
		"argocd-secrets chart path should be a local path (pulled), not the remote chart name")
}

// TestIssue923_OCIChartNotPreparedWithoutIncludeNeeds verifies that without
// --include-needs, the OCI chart for the needed release is NOT prepared.
func TestIssue923_OCIChartNotPreparedWithoutIncludeNeeds(t *testing.T) {
	resetChartCacheForTest()
	helmfileContent := []byte(`
repositories:
  - name: huma
    url: ghcr.io/huma-engineering/helm-charts
    oci: true

releases:
  - name: argocd
    namespace: argocd
    chart: argo/argo-cd
    labels:
      app: argocd
    needs:
      - argocd/argocd-secrets

  - name: argocd-secrets
    namespace: argocd
    chart: huma/external-secret-resources
    labels:
      app: secrets
    version: ~0.1.0
`)

	logger := zap.NewExample().Sugar()

	st, err := createFromYaml(helmfileContent, "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	// Simulate: helmfile -l app=argocd (without --include-needs)
	st.Selectors = []string{"app=argocd"}

	tempDir := t.TempDir()

	helm := &exectest.Helm{
		Helm3:       true,
		ChartsMutex: &sync.Mutex{},
	}

	opts := ChartPrepareOptions{
		SkipResolve:            true,
		IncludeTransitiveNeeds: false,
		Concurrency:            1,
		OutputDirTemplate:      "{{ .Release.Name }}",
	}

	releaseToChart, errs := st.PrepareCharts(helm, tempDir, 1, "apply", opts)
	require.Empty(t, errs)

	// argocd-secrets should NOT have a prepared chart
	assert.NotContains(t, releaseToChart, PrepareChartKey{Name: "argocd-secrets", Namespace: "argocd"},
		"argocd-secrets chart should NOT be prepared without --include-needs")
	assert.Empty(t, helm.PulledCharts,
		"ChartPull should NOT have been called without --include-needs")
}

// TestIssue923_GetSelectedReleasesWithNeeds verifies the selection logic
// that PrepareCharts depends on.
func TestIssue923_GetSelectedReleasesWithNeeds(t *testing.T) {
	helmfileContent := []byte(`
repositories:
  - name: huma
    url: ghcr.io/huma-engineering/helm-charts
    oci: true

releases:
  - name: argocd
    namespace: argocd
    chart: argo/argo-cd
    labels:
      app: argocd
    needs:
      - argocd/argocd-secrets

  - name: argocd-secrets
    namespace: argocd
    chart: huma/external-secret-resources
    labels:
      app: secrets
`)

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml(helmfileContent, filepath.Join("example", "path", "to", "helmfile.yaml"), DefaultEnv, logger)
	require.NoError(t, err)

	st.Selectors = []string{"app=argocd"}

	// With includeTransitiveNeeds=true (as PrepareCharts does when --include-needs is set)
	selected, err := st.GetSelectedReleases(true)
	require.NoError(t, err)

	names := make([]string, len(selected))
	for i, r := range selected {
		names[i] = r.Name
	}
	assert.Contains(t, names, "argocd", "selected releases should include argocd")
	assert.Contains(t, names, "argocd-secrets", "selected releases should include the needed release argocd-secrets")

	// With includeTransitiveNeeds=false (without --include-needs)
	selectedFalse, err := st.GetSelectedReleases(false)
	require.NoError(t, err)

	namesFalse := make([]string, len(selectedFalse))
	for i, r := range selectedFalse {
		namesFalse[i] = r.Name
	}
	assert.Contains(t, namesFalse, "argocd")
	assert.NotContains(t, namesFalse, "argocd-secrets",
		"needed release should NOT be selected without --include-needs")
}
