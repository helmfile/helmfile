package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

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

	st.preBuildTransitiveSubchartDeps(filepath.Join(rootDir, "envelope"))

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
// function is a no-op (no panic) when the chart dir has no Chart.yaml.
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
	st.preBuildTransitiveSubchartDeps(rootDir)
}

// TestPreBuildTransitiveSubchartDeps_HandlesCircularDeps verifies the function
// terminates when the file:// dependency graph has a cycle.
func TestPreBuildTransitiveSubchartDeps_HandlesCircularDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode (requires helm)")
	}

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
		st.preBuildTransitiveSubchartDeps(aDir)
	}()
	select {
	case <-done:
		// success — terminated
	case <-time.After(30 * time.Second):
		t.Fatal("preBuildTransitiveSubchartDeps did not terminate on circular deps")
	}
}

// TestPreBuildTransitiveSubchartDeps_NoOpOnRemoteDeps verifies the function
// skips dependencies declared with non-file:// repositories (https://, oci://).
func TestPreBuildTransitiveSubchartDeps_NoOpOnRemoteDeps(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "helmfile-prebuild-remote-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// Chart with only a remote dep. preBuildTransitiveSubchartDeps should not
	// recurse into anything (no file:// deps to follow) and should not panic
	// when helm dep build can't resolve the remote dep in the test environment.
	writeChart(t, filepath.Join(rootDir, "chart"), "chart",
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

	// Should not panic; helm dep build failure is logged and swallowed.
	st.preBuildTransitiveSubchartDeps(filepath.Join(rootDir, "chart"))
}
