package telemetry

import (
	gocontext "context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func attrValue(t *testing.T, res *resource.Resource, key string) (string, bool) {
	t.Helper()
	for _, kv := range res.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.String(), true
		}
	}
	return "", false
}

func TestBuildResourceDefaults(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")

	res, err := buildResource("v1.2.3")
	require.NoError(t, err)

	name, ok := attrValue(t, res, "service.name")
	require.True(t, ok, "service.name should be set")
	assert.Equal(t, DefaultServiceName, name)

	version, ok := attrValue(t, res, "service.version")
	require.True(t, ok, "service.version should be set")
	assert.Equal(t, "v1.2.3", version)

	sdkName, ok := attrValue(t, res, "telemetry.sdk.name")
	require.True(t, ok, "telemetry.sdk attributes should be set")
	assert.Equal(t, "opentelemetry", sdkName)
}

func TestBuildResourceEnvOverridesDefaults(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "helmfile-ci")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "cicd.pipeline=deploy")

	res, err := buildResource("v1.2.3")
	require.NoError(t, err)

	name, _ := attrValue(t, res, "service.name")
	assert.Equal(t, "helmfile-ci", name, "OTEL_SERVICE_NAME must override the default")

	pipeline, ok := attrValue(t, res, "cicd.pipeline")
	require.True(t, ok, "OTEL_RESOURCE_ATTRIBUTES should be merged")
	assert.Equal(t, "deploy", pipeline)
}

func TestSamplerFromEnv(t *testing.T) {
	sampledTraceID, err := trace.TraceIDFromHex("0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f")
	require.NoError(t, err)
	sampledSpanID, err := trace.SpanIDFromHex("1e1e1e1e1e1e1e1e")
	require.NoError(t, err)

	sampledParent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    sampledTraceID,
		SpanID:     sampledSpanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	// samplerDecision evaluates the sampler the way the SDK does, avoiding
	// assertions on the sampler's concrete (unexported) type.
	samplerDecision := func(sampler sdktrace.Sampler, parent *trace.SpanContext) sdktrace.SamplingDecision {
		params := sdktrace.SamplingParameters{
			TraceID: sampledTraceID,
			Name:    "test",
		}
		if parent != nil {
			params.ParentContext = trace.ContextWithSpanContext(gocontext.Background(), *parent)
		}
		return sampler.ShouldSample(params).Decision
	}

	tests := []struct {
		name    string
		sampler string
		arg     string
		parent  *trace.SpanContext
		want    sdktrace.SamplingDecision
	}{
		{name: "default", want: sdktrace.RecordAndSample},
		{name: "always_on", sampler: "always_on", want: sdktrace.RecordAndSample},
		{name: "always_off", sampler: "always_off", want: sdktrace.Drop},
		{name: "parentbased_always_on root", sampler: "parentbased_always_on", want: sdktrace.RecordAndSample},
		{name: "parentbased_always_off root", sampler: "parentbased_always_off", want: sdktrace.Drop},
		{name: "parentbased_always_off respects sampled parent", sampler: "parentbased_always_off", parent: &sampledParent, want: sdktrace.RecordAndSample},
		{name: "parentbased_traceidratio zero", sampler: "parentbased_traceidratio", arg: "0", want: sdktrace.Drop},
		{name: "traceidratio zero", sampler: "traceidratio", arg: "0", want: sdktrace.Drop},
		{name: "traceidratio invalid arg falls back to 1.0", sampler: "traceidratio", arg: "not-a-number", want: sdktrace.RecordAndSample},
		{name: "unknown falls back to default", sampler: "bogus", want: sdktrace.RecordAndSample},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", tt.sampler)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", tt.arg)

			got := samplerDecision(samplerFromEnv(nil), tt.parent)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPropagatorsFromEnv(t *testing.T) {
	const traceparent = "00-0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f-1e1e1e1e1e1e1e1e-01"

	tests := []struct {
		name       string
		env        string
		wantParent bool
	}{
		{name: "default", env: "", wantParent: true},
		{name: "tracecontext", env: "tracecontext", wantParent: true},
		{name: "baggage and tracecontext", env: "baggage,tracecontext", wantParent: true},
		{name: "stray commas tolerated", env: "tracecontext,,baggage", wantParent: true},
		{name: "none", env: "none", wantParent: false},
		{name: "unsupported ignored", env: "bogus", wantParent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_PROPAGATORS", tt.env)

			prop := propagatorsFromEnv(nil)
			ctx := prop.Extract(gocontext.Background(), propagation.MapCarrier{"traceparent": traceparent})

			sc := trace.SpanFromContext(ctx).SpanContext()
			assert.Equal(t, tt.wantParent, sc.IsValid())
		})
	}
}

func TestPropagatorsFromEnvWarnsOnUnsupported(t *testing.T) {
	t.Setenv("OTEL_PROPAGATORS", "tracecontext,bogus")
	// Only smoke-checks that parsing succeeds; the warning is untestable
	// without a logger, which propagatorsFromEnv tolerates being nil.
	prop := propagatorsFromEnv(nil)
	assert.NotNil(t, prop)
}

func TestSDKDisabledFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "unset", env: "", want: false},
		{name: "true", env: "true", want: true},
		{name: "TRUE case-insensitive", env: "TRUE", want: true},
		{name: "false", env: "false", want: false},
		{name: "surrounding whitespace", env: " true ", want: true},
		{name: "other values", env: "1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_SDK_DISABLED", tt.env)
			assert.Equal(t, tt.want, sdkDisabledFromEnv())
		})
	}
}

func TestEnvCarrier(t *testing.T) {
	t.Setenv("TRACEPARENT", "00-abc-def-01")
	t.Setenv("TRACESTATE", "")
	t.Setenv("BAGGAGE", "k=v")

	carrier := envCarrier()
	assert.Equal(t, "00-abc-def-01", carrier["traceparent"])
	assert.Equal(t, "k=v", carrier["baggage"])
	_, ok := carrier["tracestate"]
	assert.False(t, ok, "empty env vars should be omitted")
}

func TestBuildResourceKeyTypes(t *testing.T) {
	// Guard against accidentally renaming resource attribute keys.
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")

	res, err := buildResource("v")
	require.NoError(t, err)

	keys := map[string]bool{}
	for _, kv := range res.Attributes() {
		keys[string(kv.Key)] = true
	}
	assert.True(t, keys["service.name"])
	assert.True(t, keys["service.version"])
}
