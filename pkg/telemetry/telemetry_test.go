package telemetry

import (
	gocontext "context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// hermeticEnvVars are the environment variables read by this package; clearing
// them makes each test independent of the developer's shell.
var hermeticEnvVars = []string{
	"OTEL_TRACES_EXPORTER",
	"OTEL_TRACES_SAMPLER",
	"OTEL_TRACES_SAMPLER_ARG",
	"OTEL_PROPAGATORS",
	"OTEL_SDK_DISABLED",
	"OTEL_SERVICE_NAME",
	"OTEL_RESOURCE_ATTRIBUTES",
	"TRACEPARENT",
	"TRACESTATE",
	"BAGGAGE",
}

// setupForTest enables telemetry with the "none" exporter, so tests touch
// neither the network nor stdout while spans are still recorded locally.
func setupForTest(t *testing.T) {
	t.Helper()
	for _, key := range hermeticEnvVars {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	Reset()
	Setup(gocontext.Background(), Options{Enabled: true, Version: "test-version"})
	require.True(t, current.Load().enabled, "telemetry should be enabled after Setup")
}

func TestDisabledByDefaultIsFullNoOp(t *testing.T) {
	Reset()

	assert.Equal(t, gocontext.Background(), CommandContext())

	// Tracer returns a no-op tracer: spans are non-recording.
	_, span := Tracer(ScopeHelmfile).Start(CommandContext(), "span")
	assert.False(t, span.IsRecording())
	span.End()

	// StartCommandSpan and Setup(disabled) are no-ops.
	StartCommandSpan("helmfile sync", attribute.String("k", "v"))
	Setup(gocontext.Background(), Options{Enabled: false})
	assert.Equal(t, gocontext.Background(), CommandContext())

	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0))
}

func TestAccessorsBeforeSetupAreNilSafe(t *testing.T) {
	Reset()

	assert.Equal(t, gocontext.Background(), CommandContext())
	_, span := Tracer(ScopeHelm).Start(gocontext.Background(), "span")
	assert.False(t, span.IsRecording())
	span.End()
	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0))
	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0), "Shutdown is idempotent")
}

func TestEnabledCommandSpanBecomesCommandContext(t *testing.T) {
	setupForTest(t)

	StartCommandSpan("helmfile sync", attribute.String("helmfile.environment", "test"))
	ctx := CommandContext()
	require.NotEqual(t, gocontext.Background(), ctx)

	span := trace.SpanFromContext(ctx)
	require.True(t, span.IsRecording())
	require.True(t, span.SpanContext().IsValid())

	// Tracer switches to the real provider.
	_, childSpan := Tracer(ScopeHelm).Start(ctx, "child")
	assert.True(t, childSpan.IsRecording())
	childSpan.End()

	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0))

	// After Shutdown everything behaves as disabled again.
	assert.Equal(t, gocontext.Background(), CommandContext())
	_, span = Tracer(ScopeHelmfile).Start(gocontext.Background(), "span")
	assert.False(t, span.IsRecording())
	span.End()
}

func TestTraceParentExtraction(t *testing.T) {
	setupForTest(t)

	traceID := strings.Repeat("0f", 16)
	spanID := strings.Repeat("1e", 8)
	t.Setenv("TRACEPARENT", "00-"+traceID+"-"+spanID+"-01")

	StartCommandSpan("helmfile sync")

	sc := trace.SpanFromContext(CommandContext()).SpanContext()
	require.True(t, sc.IsValid(), "command span should join the remote parent trace")
	assert.Equal(t, traceID, sc.TraceID().String())
}

func TestSetupIdempotentAndReinitializable(t *testing.T) {
	setupForTest(t)

	// A duplicate Setup is ignored rather than replacing the provider.
	Setup(gocontext.Background(), Options{Enabled: true, Version: "other"})
	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0))
	assert.False(t, current.Load().enabled)

	// After Shutdown, Setup can initialize a fresh provider.
	Setup(gocontext.Background(), Options{Enabled: true, Version: "test-version"})
	require.True(t, current.Load().enabled)
	StartCommandSpan("helmfile sync")
	span := trace.SpanFromContext(CommandContext())
	require.True(t, span.IsRecording())
	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0))
}

func TestSetupDegradesOnBadExporter(t *testing.T) {
	for _, key := range hermeticEnvVars {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_TRACES_EXPORTER", "not-a-real-exporter")
	Reset()

	// Invalid exporter configuration must disable telemetry, not fail.
	Setup(gocontext.Background(), Options{Enabled: true, Version: "test-version"})
	assert.False(t, current.Load().enabled)
	assert.Equal(t, gocontext.Background(), CommandContext())
	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0))
}

func TestSetupHonorsSDKDisabled(t *testing.T) {
	for _, key := range hermeticEnvVars {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	Reset()

	Setup(gocontext.Background(), Options{Enabled: true, Version: "test-version"})
	assert.False(t, current.Load().enabled)
}

func TestShutdownRecordableWithoutCommandSpan(t *testing.T) {
	setupForTest(t)

	// Shutdown without StartCommandSpan must not panic.
	assert.NoError(t, Shutdown(gocontext.Background(), nil, 0))
}

func TestShutdownWithNilContext(t *testing.T) {
	setupForTest(t)
	StartCommandSpan("helmfile sync")

	// A nil context is part of the defensive contract; use a typed nil to
	// satisfy staticcheck while still exercising the nil path.
	var nilCtx gocontext.Context
	assert.NoError(t, Shutdown(nilCtx, nil, 0))
}

func TestScopeConstants(t *testing.T) {
	assert.Equal(t, "helmfile", ScopeHelmfile)
	assert.Equal(t, "helm", ScopeHelm)
}
