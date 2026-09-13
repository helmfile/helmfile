package state

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
)

// initGitRepo creates a throwaway git repository at dir containing a chart at
// chartRel (e.g. "charts/mychart") and returns the go-getter URL that targets
// it on the given ref. It is used to exercise the go-getter fetch path without
// hitting the network.
func initGitRepo(t *testing.T, dir, chartRel, ref string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", ref)
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	chartDir := filepath.Join(dir, filepath.FromSlash(chartRel))
	require.NoError(t, os.MkdirAll(chartDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: mychart\nversion: 0.1.0\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "init")
	return "git::file://" + dir + "@" + chartRel + "?ref=" + ref
}

// TestAdhocDependencyGoGetterDetection verifies that the go-getter URL shapes
// used in ad-hoc dependencies (release.dependencies[].chart) are detected as
// remote sources by remote.IsRemote, which is the predicate gating the fetch
// branch added for issue #821.
func TestAdhocDependencyGoGetterDetection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		chart  string
		remote bool
	}{
		{name: "forced git getter", chart: "git::https://github.com/example/repo.git@charts/app?ref=v1.0.0", remote: true},
		{name: "forced git getter file", chart: "git::file:///tmp/repo@charts/app?ref=main", remote: true},
		{name: "https url", chart: "https://example.com/charts/app-0.1.0.tgz", remote: true},
		{name: "named repo passthrough", chart: "prometheus-community/kube-prometheus-stack", remote: false},
		{name: "oci url passthrough", chart: "oci://registry.example.com/charts/app", remote: false},
		{name: "local relative path", chart: "../charts/app", remote: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.remote, remote.IsRemote(tc.chart))
		})
	}
}

// TestAdhocDependencyGoGetterFetch is the regression test for issue #821: an
// ad-hoc dependency using a go-getter URL must be fetched to a local directory
// via downloadAdhocDepChartWithGoGetter instead of being passed through to
// chartify (which would then fail with "no helm list entry found for
// repository"). It uses a local file:// git repo so no network access is
// required.
func TestAdhocDependencyGoGetterFetch(t *testing.T) {
	repo := t.TempDir()
	url := initGitRepo(t, repo, "charts/mychart", "main")

	logger := helmexec.NewLogger(io.Discard, "warn")
	st := &HelmState{
		logger:   logger,
		fs:       filesystem.DefaultFileSystem(),
		basePath: repo,
	}
	release := &ReleaseSpec{
		Name:      "test-release",
		Namespace: "test-ns",
	}

	got, err := st.downloadAdhocDepChartWithGoGetter(release, url)
	require.NoError(t, err)

	info, err := os.Stat(got)
	require.NoError(t, err, "fetched dependency path must exist on disk")
	assert.True(t, info.IsDir(), "fetched dependency must be a directory")
	_, err = os.Stat(filepath.Join(got, "Chart.yaml"))
	assert.NoError(t, err, "fetched dependency must contain the chart contents")
}
