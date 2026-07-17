package state

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

// skipIfNoHelm skips the calling test when the helm binary is not available on
// PATH. Tests that exercise runHelmDepBuild through a real helmexec need helm
// installed to produce meaningful results.
func skipIfNoHelm(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skipf("skipping: helm binary not found in PATH (%v)", err)
	}
}

// newRealHelmExec builds a helmexec.Interface backed by the real helm binary on
// PATH, for tests that assert actual `helm dependency build` side effects.
func newRealHelmExec(t *testing.T) helmexec.Interface {
	t.Helper()
	logger := zap.NewNop().Sugar()
	runner := &helmexec.ShellRunner{Ctx: context.Background(), Logger: logger}
	helm, err := helmexec.New("helm", helmexec.HelmExecOptions{}, logger, "", "", runner)
	if err != nil {
		t.Fatalf("helmexec.New: %v", err)
	}
	return helm
}

// stubHelm is a minimal helmexec.Interface for prebuild tests that should NOT
// invoke a real helm binary (avoids network I/O and the helm dependency). It
// records BuildDeps/UpdateDeps calls so tests can assert which charts were
// visited. Only BuildDeps/UpdateDeps are exercised by the prebuild path, so the
// embedded nil Interface satisfies the rest of the contract without being
// dereferenced.
type stubHelm struct {
	helmexec.Interface
	buildErr error
	builds   []string
	updates  []string
}

func (s *stubHelm) BuildDeps(name, chart string, flags ...string) error {
	s.builds = append(s.builds, chart)
	return s.buildErr
}

func (s *stubHelm) UpdateDeps(chart string) error {
	s.updates = append(s.updates, chart)
	return s.buildErr
}

// writeChart creates a chart directory with Chart.yaml and the given templates.
// deps maps dependency name → repository URL (skipped if empty).
func writeChart(t *testing.T, dir, name string, deps map[string]string, templates map[string]string) {
	t.Helper()
	chartYaml := "apiVersion: v2\nname: " + name + "\nversion: 0.1.0\n"
	if len(deps) > 0 {
		chartYaml += "dependencies:\n"
		for depName, repo := range deps {
			chartYaml += "  - name: " + depName + "\n    repository: " + repo + "\n    version: 0.1.0\n"
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYaml), 0644); err != nil {
		t.Fatalf("write Chart.yaml in %s: %v", dir, err)
	}
	if len(templates) > 0 {
		tmplDir := filepath.Join(dir, "templates")
		if err := os.MkdirAll(tmplDir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", tmplDir, err)
		}
		for filename, content := range templates {
			if err := os.WriteFile(filepath.Join(tmplDir, filename), []byte(content), 0644); err != nil {
				t.Fatalf("write %s: %v", filename, err)
			}
		}
	}
}

// TestPreBuildTransitiveSubchartDeps_VisitsAllTransitiveDeps verifies that
// preBuildTransitiveSubchartDeps walks the full transitive file:// dependency
// tree by checking that helm dep build runs on every chart dir.
//
// We assert this by checking that sub2/charts/nested-* exists after the call —
// that's the exact artifact that helm's non-recursive dep build fails to
// produce for envelope charts (issue #851).
func TestPreBuildTransitiveSubchartDeps_VisitsAllTransitiveDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode (requires helm)")
	}
	skipIfNoHelm(t)

	rootDir, err := os.MkdirTemp("", "helmfile-prebuild-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// envelope → sub1, sub2
	// sub2 → nested
	writeChart(t, filepath.Join(rootDir, "envelope"), "envelope",
		map[string]string{"sub1": "file://../sub1", "sub2": "file://../sub2"},
		map[string]string{"cm.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: parent-cm\n"},
	)
	writeChart(t, filepath.Join(rootDir, "sub1"), "sub1", nil,
		map[string]string{"sm.yaml": "apiVersion: monitoring.coreos.com/v1\nkind: ServiceMonitor\nmetadata:\n  name: sub1-sm\n"},
	)
	writeChart(t, filepath.Join(rootDir, "sub2"), "sub2",
		map[string]string{"nested": "file://../nested"},
		map[string]string{"sm.yaml": "apiVersion: monitoring.coreos.com/v1\nkind: ServiceMonitor\nmetadata:\n  name: sub2-sm\n"},
	)
	writeChart(t, filepath.Join(rootDir, "nested"), "nested", nil,
		map[string]string{"sm.yaml": "apiVersion: monitoring.coreos.com/v1\nkind: ServiceMonitor\nmetadata:\n  name: nested-sm\n"},
	)

	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			DefaultHelmBinary: "helm",
		},
	}

	st.preBuildTransitiveSubchartDeps(filepath.Join(rootDir, "envelope"), newRealHelmExec(t))

	// The critical assertion: sub2/charts/ should now contain nested (as
	// .tgz or unpacked). Without the fix, helm dep build on the parent chart
	// produces sub2.tgz WITHOUT nested, and the nested ServiceMonitor is
	// silently dropped from the chartify output.
	sub2Charts := filepath.Join(rootDir, "sub2", "charts")
	entries, err := os.ReadDir(sub2Charts)
	if err != nil {
		t.Fatalf("sub2/charts should exist after preBuildTransitiveSubchartDeps ran helm dep build on sub2: %v", err)
	}
	foundNested := false
	for _, e := range entries {
		name := e.Name()
		if name == "nested" || hasNestedTgzPrefix(name) {
			foundNested = true
			break
		}
	}
	if !foundNested {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("sub2/charts should contain nested after pre-build; got entries: %v", names)
	}

	// envelope/charts should contain sub1 and sub2.
	envCharts := filepath.Join(rootDir, "envelope", "charts")
	if entries, err := os.ReadDir(envCharts); err != nil {
		t.Fatalf("envelope/charts should exist after pre-build: %v", err)
	} else if len(entries) == 0 {
		t.Fatalf("envelope/charts should contain sub1 and sub2 after pre-build")
	}
}

func hasNestedTgzPrefix(name string) bool {
	return strings.HasPrefix(name, "nested-")
}

// TestPreBuildTransitiveSubchartDeps_HandlesMissingChartYaml verifies the
// function is a no-op (no panic) when the chart dir has no Chart.yaml. A stub
// helm is used since the missing Chart.yaml short-circuits before any helm call.
func TestPreBuildTransitiveSubchartDeps_HandlesMissingChartYaml(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "helmfile-prebuild-noyaml-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(rootDir)

	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			DefaultHelmBinary: "helm",
		},
	}

	// Should not panic.
	st.preBuildTransitiveSubchartDeps(rootDir, &stubHelm{})
}

// TestPreBuildTransitiveSubchartDeps_HandlesCircularDeps verifies the function
// terminates when the file:// dependency graph has a cycle.
func TestPreBuildTransitiveSubchartDeps_HandlesCircularDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode (requires helm)")
	}
	skipIfNoHelm(t)

	rootDir, err := os.MkdirTemp("", "helmfile-prebuild-cycle-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// a → b → a (cycle). Use absolute file:// paths so each can resolve the other.
	aDir := filepath.Join(rootDir, "a")
	bDir := filepath.Join(rootDir, "b")
	writeChart(t, aDir, "a", map[string]string{"b": "file://" + bDir}, nil)
	writeChart(t, bDir, "b", map[string]string{"a": "file://" + aDir}, nil)

	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			DefaultHelmBinary: "helm",
		},
	}

	// Should terminate, not infinite-loop.
	done := make(chan struct{})
	go func() {
		defer close(done)
		st.preBuildTransitiveSubchartDeps(aDir, newRealHelmExec(t))
	}()
	select {
	case <-done:
		// success — terminated
	case <-time.After(30 * time.Second):
		t.Fatal("preBuildTransitiveSubchartDeps did not terminate on circular deps")
	}
}

// TestPreBuildTransitiveSubchartDeps_DoesNotRecurseIntoRemoteDeps verifies the
// function does not recurse into dependencies declared with non-file://
// repositories (https://, oci://). It still runs helm dep build on the chart dir
// itself, but does not follow the remote dependency.
//
// A stub helm whose BuildDeps returns an error is used so the test fails fast
// and deterministically without performing any network I/O (a real helm would
// try to fetch https://charts.example.com).
func TestPreBuildTransitiveSubchartDeps_DoesNotRecurseIntoRemoteDeps(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "helmfile-prebuild-remote-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// Chart with only a remote dep. preBuildTransitiveSubchartDeps should not
	// recurse into anything (no file:// deps to follow).
	chartDir := filepath.Join(rootDir, "chart")
	writeChart(t, chartDir, "chart",
		map[string]string{"remote": "https://charts.example.com"},
		nil,
	)

	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			DefaultHelmBinary: "helm",
		},
	}

	helm := &stubHelm{buildErr: errors.New("stub: dependency build unavailable")}
	st.preBuildTransitiveSubchartDeps(chartDir, helm)

	// BuildDeps must be called exactly once, on the chart dir itself — proving
	// the function did not recurse into the remote dep.
	if len(helm.builds) != 1 {
		t.Fatalf("expected exactly one BuildDeps call (chart dir, no recursion into remote), got %v", helm.builds)
	}
	if got, want := filepath.Base(helm.builds[0]), "chart"; got != want {
		t.Fatalf("BuildDeps called on %q, want chart dir %q", helm.builds[0], want)
	}
	// The stub error is not a lock-out-of-sync signal, so there must be no
	// fallback to UpdateDeps.
	if len(helm.updates) != 0 {
		t.Fatalf("expected no UpdateDeps fallback, got %v", helm.updates)
	}
}
