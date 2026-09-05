package state

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

// Test-local constants shared by the issue #2766 integration tests.
const (
	issue2766RepoName   = "myrepo"
	issue2766Chart      = "myrepo/mychart"
	issue2766Release    = "app"
	issue2766RepoURL    = "registry.example.com/charts"
	issue2766VersionArg = "--version"
)

// mockOCIPullHelm wraps exectest.Helm so an integration test can observe the
// exact chart reference, --version flag, and destination path helmfile hands
// to `helm chart pull` after OCI constraint resolution. It also implements
// ShowChartWithFlags (the helmexec.ChartInspector capability) so
// resolveOCIConstraintVersion has a resolver to talk to.
type mockOCIPullHelm struct {
	*exectest.Helm
	// ResolveFn is invoked in place of a real registry round-trip; it
	// receives the constraint version and returns the concrete resolved
	// version. Tests can vary its behavior across successive invocations to
	// simulate a newly published tag.
	ResolveFn func(constraint string) (chart.Metadata, error)

	pullCount       atomic.Int32
	pulledChart     atomic.Value // string: last ref passed to ChartPull
	pulledPath      atomic.Value // string: last destination dir passed to ChartPull
	pulledVersion   atomic.Value // string: value of the --version flag on the last ChartPull
	inspectorCalled atomic.Int32
}

func (m *mockOCIPullHelm) ShowChartWithFlags(chartPath string, flags ...string) (chart.Metadata, error) {
	m.inspectorCalled.Add(1)
	// Extract --version to prove the resolver passes the raw constraint down.
	var constraint string
	for i, f := range flags {
		if f == issue2766VersionArg && i+1 < len(flags) {
			constraint = flags[i+1]
			break
		}
	}
	if m.ResolveFn == nil {
		return chart.Metadata{}, nil
	}
	return m.ResolveFn(constraint)
}

func (m *mockOCIPullHelm) ChartPull(chartRef string, path string, flags ...string) error {
	m.pullCount.Add(1)
	m.pulledChart.Store(chartRef)
	m.pulledPath.Store(path)
	var version string
	for i, f := range flags {
		if f == issue2766VersionArg && i+1 < len(flags) {
			version = flags[i+1]
			break
		}
	}
	m.pulledVersion.Store(version)
	// Materialize a Chart.yaml so getOCIChart's findChartDirectory call
	// downstream succeeds. exectest.Helm.ChartPull would do this too, but we
	// hard-code the resolved version into the Chart.yaml here so that any
	// future assertion on Chart.yaml contents can distinguish the versions.
	chartDir := filepath.Join(path, "test-chart")
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return err
	}
	chartYaml := "apiVersion: v2\nname: test-chart\ndescription: A test chart\ntype: application\nversion: \"" + version + "\"\nappVersion: \"" + version + "\"\n"
	return os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYaml), 0644)
}

// TestGetOCIChart_ResolvesConstraintIntoCachePathAndPullFlag verifies the end
// to end wiring for issue #2766: given an OCI release with a constraint
// version like `~1`, getOCIChart must (a) call the ChartInspector to resolve
// the constraint, (b) use the resolved concrete version as the on-disk cache
// path segment (so the cache is content-addressable), and (c) pass the same
// resolved version to `helm chart pull --version`. The isolated resolver
// unit test only proves the helper works; this test proves the caller wires
// its output into every downstream consumer.
func TestGetOCIChart_ResolvesConstraintIntoCachePathAndPullFlag(t *testing.T) {
	resetChartCacheForTest()

	// Sandbox the shared helmfile cache dir so this test can safely exercise
	// the `opts.OutputDirTemplate == ""` branch that writes into remote.CacheDir()
	// without touching the user's real ~/.cache/helmfile.
	t.Setenv(envvar.CacheHome, t.TempDir())

	logger := zap.NewExample().Sugar()
	st := &HelmState{
		ReleaseSetSpec: ReleaseSetSpec{
			Repositories: []RepositorySpec{
				{Name: issue2766RepoName, URL: issue2766RepoURL, OCI: true},
			},
		},
		logger:      logger,
		valsRuntime: valsRuntime,
		fs:          filesystem.DefaultFileSystem(),
	}

	release := &ReleaseSpec{
		Name:    issue2766Release,
		Chart:   issue2766Chart,
		Version: "~1",
	}

	const resolved = "1.0.1"
	helm := &mockOCIPullHelm{
		Helm: &exectest.Helm{Helm3: true},
		ResolveFn: func(constraint string) (chart.Metadata, error) {
			require.Equal(t, "~1", constraint, "resolver must receive the raw constraint from the release spec")
			return chart.Metadata{Version: resolved}, nil
		},
	}

	_, err := st.getOCIChart(release, "", helm, ChartPrepareOptions{
		SkipRefresh: true,
		SkipDeps:    true,
	})
	require.NoError(t, err)

	// 1. Resolver was invoked exactly once for the constraint.
	require.Equal(t, int32(1), helm.inspectorCalled.Load(),
		"ChartInspector.ShowChartWithFlags must be called once for a constraint version")

	// 2. helm chart pull was invoked once (no double-download regression).
	require.Equal(t, int32(1), helm.pullCount.Load())

	// 3. The --version flag on `helm chart pull` carries the RESOLVED value,
	//    not the raw constraint. Without the resolution wiring, this would
	//    still be "~1".
	require.Equal(t, resolved, helm.pulledVersion.Load(),
		"helm chart pull --version must receive the resolved version, not the raw constraint")

	// 4. The chart ref keeps its `:<version>` suffix aligned with the resolved
	//    value (helmfile builds it as `<repo>/<chart>:<version>`), so helm has
	//    no way to observe the raw constraint at any downstream layer.
	pulledChart, _ := helm.pulledChart.Load().(string)
	require.Contains(t, pulledChart, ":"+resolved,
		"the OCI ref passed to helm chart pull must include the resolved version tag")
	require.NotContains(t, pulledChart, ":~1",
		"the OCI ref passed to helm chart pull must not include the raw constraint")

	// 5. The on-disk destination path is derived from the RESOLVED version.
	//    Prior to the fix this would be `.../mychart/_1/`; after the fix it
	//    should be `.../mychart/1.0.1/`. That path shape is what makes the
	//    shared cache content-addressable and self-invalidating.
	pulledPath, _ := helm.pulledPath.Load().(string)
	require.Contains(t, pulledPath, string(filepath.Separator)+resolved,
		"cache destination path must contain the resolved version segment")
	require.NotContains(t, pulledPath, string(filepath.Separator)+"_1",
		"cache destination path must NOT contain the raw-constraint segment (_1)")

	// 6. The chart directory helmfile created for this release actually lives
	//    under the resolved-version cache path, and its Chart.yaml holds the
	//    resolved version — belt-and-suspenders against a path/flag mismatch.
	writtenChartYaml := filepath.Join(pulledPath, "test-chart", "Chart.yaml")
	body, err := os.ReadFile(writtenChartYaml)
	require.NoError(t, err)
	require.Contains(t, string(body), "version: \""+resolved+"\"")
}

// TestGetOCIChart_ResolvesToDifferentVersionsPicksSeparateCachePaths verifies
// that when a constraint resolves to two different concrete versions across
// runs (e.g. `~1` first resolves to 1.0.1, then a newer 1.0.2 tag is
// published and matches the same constraint), each resolution writes to its
// own cache path. This is the core promise of the fix: once the raw
// constraint is out of the path, the shared cache stops silently serving
// stale content when the registry gets a new matching tag.
func TestGetOCIChart_ResolvesToDifferentVersionsPicksSeparateCachePaths(t *testing.T) {
	resetChartCacheForTest()
	t.Setenv(envvar.CacheHome, t.TempDir())

	logger := zap.NewExample().Sugar()
	st := &HelmState{
		ReleaseSetSpec: ReleaseSetSpec{
			Repositories: []RepositorySpec{
				{Name: issue2766RepoName, URL: issue2766RepoURL, OCI: true},
			},
		},
		logger:      logger,
		valsRuntime: valsRuntime,
		fs:          filesystem.DefaultFileSystem(),
	}

	// First resolution: `~1` -> 1.0.1
	release := &ReleaseSpec{Name: issue2766Release, Chart: issue2766Chart, Version: "~1"}
	first := &mockOCIPullHelm{
		Helm: &exectest.Helm{Helm3: true},
		ResolveFn: func(_ string) (chart.Metadata, error) {
			return chart.Metadata{Version: "1.0.1"}, nil
		},
	}
	_, err := st.getOCIChart(release, "", first, ChartPrepareOptions{SkipRefresh: true, SkipDeps: true})
	require.NoError(t, err)
	firstPath, _ := first.pulledPath.Load().(string)
	require.Contains(t, firstPath, string(filepath.Separator)+"1.0.1")

	// Simulate the registry publishing 1.0.2 and start over with a fresh
	// process-local cache (resetChartCacheForTest). The constraint is
	// unchanged; only what it resolves to has moved. Second resolution:
	// `~1` -> 1.0.2 must land in a distinct cache dir, not reuse `1.0.1`.
	resetChartCacheForTest()
	release = &ReleaseSpec{Name: issue2766Release, Chart: issue2766Chart, Version: "~1"}
	second := &mockOCIPullHelm{
		Helm: &exectest.Helm{Helm3: true},
		ResolveFn: func(_ string) (chart.Metadata, error) {
			return chart.Metadata{Version: "1.0.2"}, nil
		},
	}
	_, err = st.getOCIChart(release, "", second, ChartPrepareOptions{SkipRefresh: true, SkipDeps: true})
	require.NoError(t, err)
	secondPath, _ := second.pulledPath.Load().(string)
	require.Contains(t, secondPath, string(filepath.Separator)+"1.0.2",
		"the second resolution must use its own resolved-version cache path")
	require.NotEqual(t, firstPath, secondPath,
		"newly-resolved version must NOT reuse the previously-resolved cache directory")
	// Neither run should have polluted the other with a raw-constraint segment.
	require.False(t, strings.Contains(firstPath, "_1") || strings.Contains(secondPath, "_1"),
		"no cache path should carry the raw `_1` constraint segment after resolution")
}
