package telemetry

import (
	gocontext "context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.uber.org/zap"
)

// defaultSamplerName is the OTel default sampler for a CLI tool: always sample
// unless a remote parent (e.g. an unsampled CI trace) says otherwise.
const defaultSamplerName = "parentbased_always_on"

// HermeticEnvVars lists every environment variable this package reads. It is
// the single source of truth for tests (which clear them all) — add new
// variables here, not in test files.
var HermeticEnvVars = []string{
	"OTEL_TRACES_EXPORTER",
	"OTEL_METRICS_EXPORTER",
	"OTEL_TRACES_SAMPLER",
	"OTEL_TRACES_SAMPLER_ARG",
	"OTEL_PROPAGATORS",
	"OTEL_SDK_DISABLED",
	"OTEL_SERVICE_NAME",
	"OTEL_RESOURCE_ATTRIBUTES",
	"HELMFILE_OTEL_METRICS_PER_RELEASE",
	"TRACEPARENT",
	"TRACESTATE",
	"BAGGAGE",
}

// newTracerProvider builds the SDK provider. Exporter selection is delegated
// to autoexport (OTEL_TRACES_EXPORTER: otlp | console | none; protocol and
// endpoint via OTEL_EXPORTER_OTLP_*), so helmfile maintains no
// exporter-construction code of its own.
func newTracerProvider(ctx gocontext.Context, opts Options, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("constructing trace exporter from OTEL_* environment: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerFromEnv(opts.Logger)),
	), nil
}

// newProviders builds the tracer and meter providers over one shared
// resource. If the metric provider fails after the tracer provider was
// constructed, the tracer provider is shut down (bounded) so its batch
// goroutine does not outlive the failed setup.
func newProviders(ctx gocontext.Context, opts Options) (*sdktrace.TracerProvider, *sdkmetric.MeterProvider, error) {
	res, err := buildResource(opts.Version)
	if err != nil {
		return nil, nil, fmt.Errorf("building OTel resource: %w", err)
	}

	provider, err := newTracerProvider(ctx, opts, res)
	if err != nil {
		return nil, nil, err
	}

	meters, err := newMeterProvider(ctx, res)
	if err != nil {
		shutdownCtx, cancel := gocontext.WithTimeout(gocontext.Background(), ShutdownTimeout)
		defer cancel()
		_ = provider.Shutdown(shutdownCtx)
		return nil, nil, err
	}

	return provider, meters, nil
}

// newMeterProvider builds the metrics provider. Reader selection is delegated
// to autoexport (OTEL_METRICS_EXPORTER: otlp | console | prometheus | none);
// the OTLP reader is a periodic reader whose interval honors
// OTEL_METRIC_EXPORT_INTERVAL (read by the SDK itself).
func newMeterProvider(ctx gocontext.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	reader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("constructing metric reader from OTEL_* environment: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	), nil
}

// buildResource merges helmfile's defaults with OTEL_SERVICE_NAME /
// OTEL_RESOURCE_ATTRIBUTES overrides. resource.Merge(a, b) lets b win, so the
// environment-derived resource is merged last.
func buildResource(serviceVersion string) (*resource.Resource, error) {
	defaults := resource.NewSchemaless(
		semconv.ServiceName(DefaultServiceName),
		semconv.ServiceVersion(serviceVersion),
	)
	fromEnv, err := resource.New(gocontext.Background(),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}
	return resource.Merge(defaults, fromEnv)
}

// samplerFromEnv parses OTEL_TRACES_SAMPLER / OTEL_TRACES_SAMPLER_ARG. Unknown
// or invalid values fall back to the default sampler with a warning — a bad
// sampler configuration must not fail the run.
func samplerFromEnv(logger *zap.SugaredLogger) sdktrace.Sampler {
	name := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER"))
	if name == "" {
		name = defaultSamplerName
	}
	arg := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))

	switch name {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(parseSamplerRatio(arg, logger))
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(parseSamplerRatio(arg, logger)))
	default:
		warnf(logger, "unsupported OTEL_TRACES_SAMPLER %q; falling back to %s", name, defaultSamplerName)
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

func parseSamplerRatio(arg string, logger *zap.SugaredLogger) float64 {
	if arg == "" {
		return 1.0
	}
	ratio, err := strconv.ParseFloat(arg, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		warnf(logger, "invalid OTEL_TRACES_SAMPLER_ARG %q; using 1.0", arg)
		return 1.0
	}
	return ratio
}

// propagatorsFromEnv parses OTEL_PROPAGATORS (default "tracecontext,baggage").
// Unsupported names are ignored with a warning rather than failing the run.
func propagatorsFromEnv(logger *zap.SugaredLogger) propagation.TextMapPropagator {
	raw := os.Getenv("OTEL_PROPAGATORS")
	if raw == "" {
		raw = "tracecontext,baggage"
	}

	var propagators []propagation.TextMapPropagator
	for _, name := range strings.Split(raw, ",") {
		switch strings.TrimSpace(name) {
		case "tracecontext":
			propagators = append(propagators, propagation.TraceContext{})
		case "baggage":
			propagators = append(propagators, propagation.Baggage{})
		case "none":
			return propagation.NewCompositeTextMapPropagator()
		case "":
			// Tolerate stray commas, e.g. "tracecontext,,baggage".
		default:
			warnf(logger, "unsupported OTEL_PROPAGATORS entry %q; ignoring", name)
		}
	}
	return propagation.NewCompositeTextMapPropagator(propagators...)
}

// sdkDisabledFromEnv reports whether OTEL_SDK_DISABLED disables the SDK. The
// OTel Go SDK core does not read this variable itself, so this wrapper honors
// the specification instead.
func sdkDisabledFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

// propagatedEnvKeys are the W3C propagation headers that can appear as process
// environment variables; TRACEPARENT is how CI systems hand a parent context
// to child processes.
var propagatedEnvKeys = []string{"TRACEPARENT", "TRACESTATE", "BAGGAGE"}

func envCarrier() propagation.MapCarrier {
	carrier := propagation.MapCarrier{}
	for _, key := range propagatedEnvKeys {
		if v := os.Getenv(key); v != "" {
			carrier[strings.ToLower(key)] = v
		}
	}
	return carrier
}
