package telemetry

import (
	gocontext "context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/helmfile/helmfile/pkg/envvar"
)

// The instruments are created from the otel global meter: before Setup they
// are delegating no-op instruments, and once Setup installs the real meter
// provider they forward to it — so recording at call sites is branch-free
// when telemetry is disabled. Setup re-creates them under the installed
// provider with the instrumentation scope version set (unavailable at
// package init).

var (
	execDurationHistogram    metric.Float64Histogram
	releaseResultCounter     metric.Int64Counter
	releaseDurationHistogram metric.Float64Histogram
)

// execDurationBuckets are tuned for seconds-scale helm subprocesses (roughly
// 10ms probe calls to multi-minute waits). The SDK's default explicit-bucket
// boundaries ([0, 5, 10, …, 10000]) are millisecond-oriented and would lump
// every sub-5s invocation — the common case — into the first bucket.
var execDurationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600,
}

func init() {
	execDurationHistogram = newExecDurationHistogram(otel.Meter(ScopeHelmfile))
	releaseResultCounter = newReleaseResultCounter(otel.Meter(ScopeHelmfile))
	releaseDurationHistogram = newReleaseDurationHistogram(otel.Meter(ScopeHelmfile))
}

func newExecDurationHistogram(m metric.Meter) metric.Float64Histogram {
	// Instrument creation through the global meter cannot fail (errors are
	// only returned for duplicate or invalid names, and errors would be
	// represented as no-op instruments anyway).
	h, _ := m.Float64Histogram(
		"helmfile.helm.exec.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of helm subprocess invocations started by helmfile, by subcommand and success."),
		metric.WithExplicitBucketBoundaries(execDurationBuckets...),
	)
	return h
}

func newReleaseResultCounter(m metric.Meter) metric.Int64Counter {
	c, _ := m.Int64Counter(
		"helmfile.release.count",
		metric.WithUnit("{release}"),
		metric.WithDescription("Completed helmfile release operations, by verb and result."),
	)
	return c
}

func newReleaseDurationHistogram(m metric.Meter) metric.Float64Histogram {
	h, _ := m.Float64Histogram(
		"helmfile.release.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Wall-clock duration of release operations (prepare..tracking), by verb and result; per-release name and namespace when HELMFILE_OTEL_METRICS_PER_RELEASE=true."),
		metric.WithExplicitBucketBoundaries(execDurationBuckets...),
	)
	return h
}

// reinitMetrics re-creates the instruments under the installed provider with
// the instrumentation scope version stamped (Setup happens before any
// recording, so the swap is race-free in production use).
func reinitMetrics(version string) {
	m := otel.Meter(ScopeHelmfile, metric.WithInstrumentationVersion(version))
	execDurationHistogram = newExecDurationHistogram(m)
	releaseResultCounter = newReleaseResultCounter(m)
	releaseDurationHistogram = newReleaseDurationHistogram(m)
}

// RecordHelmExecDuration records the duration of one helm subprocess
// invocation. No-op when telemetry is disabled.
func RecordHelmExecDuration(seconds float64, subcommand string, success bool) {
	execDurationHistogram.Record(gocontext.Background(), seconds,
		metric.WithAttributes(
			attribute.String("subcommand", subcommand),
			attribute.Bool("success", success),
		),
	)
}

// perReleaseMetrics reports whether metric attributes carrying the release
// identity are enabled (HELMFILE_OTEL_METRICS_PER_RELEASE=true). Read per
// call: release operations are low-frequency and tests toggle the variable.
func perReleaseMetrics() bool {
	return os.Getenv(envvar.OtelMetricsPerRelease) == "true"
}

// RecordReleaseDuration records the wall-clock duration of one release
// operation. Release name and namespace are attached only when
// HELMFILE_OTEL_METRICS_PER_RELEASE is set: they make time-series count
// proportional to the release fleet, which is fine for bounded CI runs but
// needs a capacity/TTL story for long-lived centralized collection.
func RecordReleaseDuration(seconds float64, verb string, err error, releaseName, namespace string) {
	result := "success"
	if err != nil {
		result = "error"
	}
	attrs := []attribute.KeyValue{
		attribute.String("verb", verb),
		attribute.String("result", result),
	}
	if perReleaseMetrics() {
		attrs = append(attrs,
			attribute.String("helmfile.release", releaseName),
			attribute.String("helmfile.namespace", namespace),
		)
	}
	releaseDurationHistogram.Record(gocontext.Background(), seconds, metric.WithAttributes(attrs...))
}

// RecordReleaseResult counts one completed release operation. No-op when
// telemetry is disabled.
func RecordReleaseResult(verb string, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	releaseResultCounter.Add(gocontext.Background(), 1,
		metric.WithAttributes(
			attribute.String("verb", verb),
			attribute.String("result", result),
		),
	)
}
