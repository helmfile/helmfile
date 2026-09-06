package state

import (
	gocontext "context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
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
	st, helm := newShimState(t)
	require.Empty(t, st.ReleaseStatuses(helm, 1))

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

// newShimState builds a one-release HelmState driven through the real
// execer with a shim binary that answers the version probe and otherwise
// exits 0 without side effects. Telemetry must already be set up.
func newShimState(t *testing.T) (*HelmState, helmexec.Interface) {
	t.Helper()
	shim := filepath.Join(t.TempDir(), "helm-shim")
	require.NoError(t, os.WriteFile(shim, []byte("#!/bin/sh\ncase \"$1\" in version) echo 'v3.14.0' ;; esac\nexit 0\n"), 0o755))

	shell := &helmexec.ShellRunner{
		Logger: zap.NewNop().Sugar(),
		Ctx:    telemetry.CommandContext(),
	}
	helm, err := helmexec.New(shim, helmexec.HelmExecOptions{}, zap.NewNop().Sugar(), "", "", shell)
	require.NoError(t, err)

	return &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			Releases: []ReleaseSpec{
				{Name: "demo", Namespace: "apps", Chart: "./charts/demo"},
			},
		},
	}, helm
}

// TestReleaseDurationMetricDefaultDims pins that helmfile.release.duration is
// exported with only the bounded verb/result dimensions unless
// HELMFILE_OTEL_METRICS_PER_RELEASE is enabled.
func TestReleaseDurationMetricDefaultDims(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix shell shim")
	}
	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile test")
	st, helm := newShimState(t)
	require.Empty(t, st.ReleaseStatuses(helm, 1))
	otlptest.ShutdownTelemetry(t)

	metrics := rec.Metrics(t)
	duration := otlptest.FindMetric(t, metrics, "helmfile.release.duration")
	dp := findHistogramPoint(t, duration, "verb", "status")
	require.NotNil(t, dp)
	assert.Positive(t, dp.GetCount())

	seen := map[string]string{}
	for _, attr := range dp.GetAttributes() {
		seen[attr.GetKey()] = attr.GetValue().GetStringValue()
	}
	assert.Equal(t, map[string]string{"verb": "status", "result": "success"}, seen,
		"release identity must NOT be attached by default (bounded cardinality)")
}

// TestReleaseDurationMetricPerRelease pins the opt-in high-cardinality mode:
// HELMFILE_OTEL_METRICS_PER_RELEASE adds the release name and namespace.
func TestReleaseDurationMetricPerRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix shell shim")
	}
	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile test")
	// Set AFTER SetupTelemetry, whose hermetic env reset clears it first.
	t.Setenv("HELMFILE_OTEL_METRICS_PER_RELEASE", "true")
	st, helm := newShimState(t)
	require.Empty(t, st.ReleaseStatuses(helm, 1))
	otlptest.ShutdownTelemetry(t)

	duration := otlptest.FindMetric(t, rec.Metrics(t), "helmfile.release.duration")
	dp := findHistogramPoint(t, duration, "helmfile.release", "demo")
	require.NotNil(t, dp, "per-release datapoint must exist when enabled")

	seen := map[string]string{}
	for _, attr := range dp.GetAttributes() {
		seen[attr.GetKey()] = attr.GetValue().GetStringValue()
	}
	assert.Equal(t, map[string]string{
		"verb":               "status",
		"result":             "success",
		"helmfile.release":   "demo",
		"helmfile.namespace": "apps",
	}, seen)
}

// assertMetrics pins the two helmfile metrics recorded on this path: one
// helm.exec duration datapoint for the status subcommand, and one successful
// release.count increment for verb=status.
func assertMetrics(t *testing.T, rec *otlptest.Recorder) {
	t.Helper()

	metrics := rec.Metrics(t)

	duration := otlptest.FindMetric(t, metrics, "helmfile.helm.exec.duration")
	assert.Equal(t, "s", duration.GetUnit())
	dp := findHistogramPoint(t, duration, "subcommand", "status")
	require.NotNil(t, dp, "duration histogram must have a subcommand=status datapoint")
	assert.Positive(t, dp.GetCount(), "histogram must record at least one observation")
	// Buckets must be the seconds-tuned set, not the SDK's millisecond-
	// oriented defaults.
	bounds := dp.GetExplicitBounds()
	require.NotEmpty(t, bounds)
	assert.Less(t, bounds[0], 0.01, "first bucket must resolve sub-second invocations")
	assert.Contains(t, bounds, 1.0)
	assert.Contains(t, bounds, 5.0)
	assert.Greater(t, bounds[len(bounds)-1], 300.0, "top bucket must cover multi-minute waits")

	releaseDuration := otlptest.FindMetric(t, metrics, "helmfile.release.duration")
	rdp := findHistogramPoint(t, releaseDuration, "verb", "status")
	require.NotNil(t, rdp, "release duration must have a verb=status datapoint")
	assert.Equal(t, "s", releaseDuration.GetUnit())

	count := otlptest.FindMetric(t, metrics, "helmfile.release.count")
	assert.Equal(t, "{release}", count.GetUnit(), "counters use curly-annotation units")
	sum := sumCounter(t, count, map[string]string{"verb": "status", "result": "success"})
	assert.EqualValues(t, 1, sum, "release.count must count one successful status")

	// The instrumentation scope carries the helmfile version.
	for _, sm := range rec.ScopeMetrics(t) {
		if sm.Scope.GetName() == "helmfile" {
			assert.Equal(t, "test", sm.Scope.GetVersion(), "scope version must be stamped")
		}
	}
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

// TestTraceOnlyContextParentsToRelease pins the trace-only bridge: with a
// parent (the release span context) hooks attach to it, stay non-cancellable,
// and fall back to the command context without one.
func TestTraceOnlyContextParentsToRelease(t *testing.T) {
	noopTracer := noop.NewTracerProvider().Tracer("test")
	parentCtx, parentSpan := noopTracer.Start(gocontext.Background(), "helmfile.release.sync")

	hookCtx := traceOnlyContext(parentCtx)

	assert.Equal(t, parentSpan, trace.SpanFromContext(hookCtx), "bridged context must carry the release span")
	assert.Nil(t, hookCtx.Done(), "bridged context must remain non-cancellable (historical behavior)")

	fallback := traceOnlyContext()
	assert.Equal(t, trace.SpanFromContext(telemetry.CommandContext()), trace.SpanFromContext(fallback), "no parent falls back to the command span")
}
