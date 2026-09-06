package telemetry

import (
	gocontext "context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// The instruments are created from the otel global meter: before Setup they
// are delegating no-op instruments, and once Setup installs the real meter
// provider they forward to it — so recording at call sites is branch-free
// when telemetry is disabled.

var (
	execDurationHistogram metric.Float64Histogram
	releaseResultCounter  metric.Int64Counter
)

func init() {
	m := otel.Meter(ScopeHelmfile)
	// Instrument creation through the global meter cannot fail (errors are
	// only returned for duplicate or invalid names, and errors would be
	// represented as no-op instruments anyway).
	execDurationHistogram, _ = m.Float64Histogram(
		"helmfile.helm.exec.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of helm subprocess invocations started by helmfile, by subcommand and success."),
	)
	releaseResultCounter, _ = m.Int64Counter(
		"helmfile.release.count",
		metric.WithDescription("Completed helmfile release operations, by verb and result."),
	)
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
