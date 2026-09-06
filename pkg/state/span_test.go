package state

import (
	gocontext "context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricsv1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/telemetry"
	"github.com/helmfile/helmfile/pkg/telemetry/otlptest"
)

// TestReleaseSpanExecNesting drives ReleaseStatuses through the real execer
// (with a shim binary that answers the version probe and otherwise exits 0)
// and telemetry enabled, then asserts the release span's subprocess span
// nests under it — the contract established by the
// HelmContext.Ctx/execWithContext funnel.
func TestReleaseSpanExecNesting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix shell shim")
	}

	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile test")

	// The shim satisfies helmexec.New's `helm version` probe and makes every
	// other invocation succeed without side effects.
	shim := filepath.Join(t.TempDir(), "helm-shim")
	require.NoError(t, os.WriteFile(shim, []byte("#!/bin/sh\ncase \"$1\" in version) echo 'v3.14.0' ;; esac\nexit 0\n"), 0o755))

	shell := &helmexec.ShellRunner{
		Logger: zap.NewNop().Sugar(),
		Ctx:    telemetry.CommandContext(),
	}
	helm, err := helmexec.New(shim, helmexec.HelmExecOptions{}, zap.NewNop().Sugar(), "", "", shell)
	require.NoError(t, err)

	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			Releases: []ReleaseSpec{
				{Name: "demo", Namespace: "apps", Chart: "./charts/demo"},
			},
		},
	}

	errs := st.ReleaseStatuses(helm, 1)
	// `true` ignores all arguments and exits 0, so no errors are expected.
	require.Empty(t, errs)

	otlptest.ShutdownTelemetry(t)

	spans := rec.Spans(t)
	release := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool { return s.Name == "helmfile.release.status" }, "release status span")

	name, ok := otlptest.AttrString(release, "helmfile.release")
	require.True(t, ok)
	assert.Equal(t, "demo", name)

	// The shim binary's name starts with "helm", so its spans are helm.exec.
	// helmexec.New's version probe also produced one (parented at the command
	// span); the release's own subprocess must be the "status" one, nested
	// under the release span.
	exec := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool {
		sub, ok := otlptest.AttrString(s, "helm.subcommand")
		return s.Name == "helm.exec" && ok && sub == "status"
	}, "release status subprocess span")
	assert.Equal(t, release.TraceId, exec.TraceId, "subprocess must join the release span's trace")
	assert.Equal(t, release.SpanId, exec.ParentSpanId, "subprocess must nest under the release span")

	assertMetrics(t, rec)
}

// assertMetrics pins the two helmfile metrics recorded on this path: one
// helm.exec duration datapoint for the status subcommand, and one successful
// release.count increment for verb=status.
func assertMetrics(t *testing.T, rec *otlptest.Recorder) {
	t.Helper()

	metrics := rec.Metrics(t)

	duration := otlptest.FindMetric(t, metrics, "helmfile.helm.exec.duration")
	dp := findHistogramPoint(t, duration, "subcommand", "status")
	require.NotNil(t, dp, "duration histogram must have a subcommand=status datapoint")
	assert.Positive(t, dp.GetCount(), "histogram must record at least one observation")

	count := otlptest.FindMetric(t, metrics, "helmfile.release.count")
	sum := sumCounter(t, count, map[string]string{"verb": "status", "result": "success"})
	assert.EqualValues(t, 1, sum, "release.count must count one successful status")
}

func findHistogramPoint(t *testing.T, m *metricsv1.Metric, key, value string) *metricsv1.HistogramDataPoint {
	t.Helper()
	for _, dp := range m.GetHistogram().GetDataPoints() {
		for _, attr := range dp.GetAttributes() {
			if attr.GetKey() == key && attr.GetValue().GetStringValue() == value {
				return dp
			}
		}
	}
	return nil
}

func sumCounter(t *testing.T, m *metricsv1.Metric, want map[string]string) int64 {
	t.Helper()
	for _, dp := range m.GetSum().GetDataPoints() {
		matched := true
		seen := map[string]string{}
		for _, attr := range dp.GetAttributes() {
			seen[attr.GetKey()] = attr.GetValue().GetStringValue()
		}
		for k, v := range want {
			if seen[k] != v {
				matched = false
				break
			}
		}
		if matched {
			return dp.GetAsInt()
		}
	}
	t.Fatalf("no %s datapoint with attributes %v", m.GetName(), want)
	return 0
}

// TestSetTraceContext pins the parent wiring: per-release spans root at the
// trace context handed over by pkg/app (the load span), falling back to
// Background when unset.
func TestSetTraceContext(t *testing.T) {
	st := &HelmState{}
	assert.Equal(t, gocontext.Background(), st.releaseSpanParent(), "unset trace context must fall back to Background")

	type ctxKey struct{}
	ctx := gocontext.WithValue(gocontext.Background(), ctxKey{}, "x")
	st.SetTraceContext(ctx)
	assert.Equal(t, ctx, st.releaseSpanParent())
}

func TestSkipUndesired(t *testing.T) {
	assert.False(t, skipUndesired(&ReleaseSpec{}), "installed unset means desired")

	installed := false
	assert.True(t, skipUndesired(&ReleaseSpec{Installed: &installed}), "installed=false must be skipped")

	installed = true
	assert.False(t, skipUndesired(&ReleaseSpec{Installed: &installed}), "installed=true must run")
}
