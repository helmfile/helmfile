package app

import (
	gocontext "context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/telemetry"
)

func newAppForTest() *App {
	globalImpl := config.NewGlobalImpl(&config.GlobalOptions{})
	printEnvImpl := config.NewPrintEnvImpl(globalImpl, &config.PrintEnvOptions{})
	return New(printEnvImpl)
}

func requireCancelable(t *testing.T, a *App) {
	t.Helper()
	require.NotNil(t, a.ctx)
	require.NotNil(t, Cancel)
	Cancel()
	select {
	case <-a.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("app context was not canceled by app.Cancel()")
	}
}

// TestNewPreservesContextContract pins the app.New context behavior that the
// telemetry integration relies on: with tracing disabled (the default), the
// app context is rooted at context.Background and remains cancellable exactly
// as before the telemetry.CommandContext() change.
func TestNewPreservesContextContract(t *testing.T) {
	a := newAppForTest()
	requireCancelable(t, a)
}

// TestNewAdoptsTelemetryCommandContext verifies that app.New derives its
// context from the telemetry command span when tracing is enabled: the app
// context carries the command span's trace, so downstream spans nest under it,
// and cancellation still works.
func TestNewAdoptsTelemetryCommandContext(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Cleanup(func() {
		_ = telemetry.Shutdown(gocontext.Background(), nil, 0)
	})

	telemetry.Setup(gocontext.Background(), telemetry.Options{Enabled: true, Version: "test"})
	telemetry.StartCommandSpan("helmfile test")

	commandSC := trace.SpanFromContext(telemetry.CommandContext()).SpanContext()
	require.True(t, commandSC.IsValid(), "command span should be recording")

	a := newAppForTest()

	// WithCancel preserves context values, so the command span — and its trace
	// ID — must still be visible through the app context.
	appSC := trace.SpanFromContext(a.ctx).SpanContext()
	require.True(t, appSC.IsValid())
	require.Equal(t, commandSC.TraceID(), appSC.TraceID(), "app context must derive from the command span context")

	requireCancelable(t, a)
}
