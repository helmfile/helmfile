// Package telemetry sets up OpenTelemetry tracing for helmfile.
//
// Tracing is strictly opt-in: when not enabled (the default), every function in
// this package is a no-op, Tracer returns the OTel no-op tracer, and
// CommandContext returns context.Background — callers never need
// "is tracing enabled?" branches.
//
// All exporter, sampler, and propagator configuration comes from the standard
// OTEL_* environment variables; helmfile defines no telemetry-specific
// environment variables of its own beyond HELMFILE_OTEL_TRACING (the on/off
// switch paired with the --otel-tracing flag).
//
// The only sanctioned instrumentation scopes are ScopeHelmfile (app/state
// layer) and ScopeHelm (helmexec layer).
package telemetry

import (
	gocontext "context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

const (
	// ScopeHelmfile is the instrumentation scope for spans in the app/state layers.
	ScopeHelmfile = "helmfile"

	// ScopeHelm is the instrumentation scope for spans in the helmexec layer.
	ScopeHelm = "helm"

	// DefaultServiceName is the OTel service.name used when OTEL_SERVICE_NAME
	// is unset.
	DefaultServiceName = "helmfile"

	// ShutdownTimeout bounds the final flush of buffered spans on exit. It is
	// applied by the caller that constructs the Shutdown context.
	ShutdownTimeout = 5 * time.Second
)

// Options controls telemetry initialization.
type Options struct {
	// Enabled turns tracing on. Everything else is ignored when false.
	Enabled bool
	// Version is the helmfile version, recorded as the service.version
	// resource attribute.
	Version string
	// Logger receives one-line diagnostics (never span data). May be nil.
	Logger *zap.SugaredLogger
}

// tracingState is the immutable snapshot read by the hot-path accessors
// (CommandContext, Tracer). Setup/StartCommandSpan/Shutdown replace it under
// stateMu; readers load it atomically.
type tracingState struct {
	enabled  bool
	provider *sdktrace.TracerProvider
	// noop is the tracer provider returned while disabled, kept here so Tracer
	// does not allocate on every call.
	noop    trace.TracerProvider
	cmdSpan trace.Span
	cmdCtx  gocontext.Context
}

var (
	// stateMu serializes Setup/StartCommandSpan/Shutdown/reset transitions.
	stateMu sync.Mutex

	// current holds the active tracingState; never nil after package init.
	current atomic.Pointer[tracingState]
)

func init() {
	current.Store(disabledState())
}

func disabledState() *tracingState {
	return &tracingState{
		enabled: false,
		noop:    noop.NewTracerProvider(),
		cmdCtx:  gocontext.Background(),
	}
}

// Setup initializes the global tracer provider from the standard OTEL_*
// environment variables (see exporter.go for what is honored). It is safe to
// call unconditionally: when opts.Enabled is false, or OTEL_SDK_DISABLED=true,
// or the exporter cannot be constructed, Setup logs a diagnostic and leaves
// telemetry disabled — telemetry problems never fail a helmfile run.
func Setup(ctx gocontext.Context, opts Options) {
	if !opts.Enabled {
		return
	}
	if ctx == nil {
		ctx = gocontext.Background()
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	if current.Load().enabled {
		warnf(opts.Logger, "OpenTelemetry tracing is already initialized; ignoring duplicate Setup")
		return
	}
	if sdkDisabledFromEnv() {
		infof(opts.Logger, "OpenTelemetry tracing disabled by OTEL_SDK_DISABLED")
		return
	}

	provider, err := newTracerProvider(ctx, opts)
	if err != nil {
		warnf(opts.Logger, "OpenTelemetry tracing unavailable: %v", err)
		return
	}

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagatorsFromEnv(opts.Logger))

	current.Store(&tracingState{
		enabled:  true,
		provider: provider,
		noop:     noop.NewTracerProvider(),
		cmdCtx:   gocontext.Background(),
	})
	infof(opts.Logger, "OpenTelemetry tracing enabled")
}

// StartCommandSpan starts the root span for one helmfile command invocation and
// makes its context the one returned by CommandContext (consumed by app.New).
// A remote parent is extracted from the TRACEPARENT/TRACESTATE/BAGGAGE
// environment variables so CI-injected trace contexts are honored. It is a
// no-op when tracing is disabled.
func StartCommandSpan(command string, attrs ...attribute.KeyValue) {
	stateMu.Lock()
	defer stateMu.Unlock()

	s := current.Load()
	if s == nil || !s.enabled || s.provider == nil {
		return
	}

	parent := otel.GetTextMapPropagator().Extract(gocontext.Background(), envCarrier())
	ctx, span := s.provider.Tracer(ScopeHelmfile).Start(parent, command, trace.WithAttributes(attrs...))
	current.Store(&tracingState{
		enabled:  true,
		provider: s.provider,
		noop:     s.noop,
		cmdSpan:  span,
		cmdCtx:   ctx,
	})
}

// CommandContext returns the context of the current command's root span, or
// context.Background when tracing is disabled. It is the single source of
// truth for deriving App.ctx.
func CommandContext() gocontext.Context {
	if s := current.Load(); s != nil && s.cmdCtx != nil {
		return s.cmdCtx
	}
	return gocontext.Background()
}

// Tracer returns a tracer for the given instrumentation scope. It never
// returns nil: the OTel no-op tracer provider is used while telemetry is
// disabled, so callers need no enabled-checks.
func Tracer(name string) trace.Tracer {
	s := current.Load()
	if s == nil {
		return noop.NewTracerProvider().Tracer(name)
	}
	if !s.enabled || s.provider == nil {
		return s.noop.Tracer(name)
	}
	return s.provider.Tracer(name)
}

// Shutdown ends the command span — recording runErr and exitCode on it when
// set — and flushes buffered spans to the exporter, bounded by ctx. It is
// idempotent and nil-safe: calling it before Setup, or twice, is fine. After
// Shutdown, telemetry behaves as disabled.
func Shutdown(ctx gocontext.Context, runErr error, exitCode int) error {
	if ctx == nil {
		ctx = gocontext.Background()
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	s := current.Load()
	current.Store(disabledState())
	if s == nil || !s.enabled || s.provider == nil {
		return nil
	}

	if s.cmdSpan != nil {
		s.cmdSpan.SetAttributes(attribute.Int("helmfile.exit_code", exitCode))
		if runErr != nil {
			s.cmdSpan.RecordError(runErr)
			s.cmdSpan.SetStatus(codes.Error, runErr.Error())
		}
		s.cmdSpan.End()
	}
	return s.provider.Shutdown(ctx)
}

// reset restores the pristine disabled state; used by tests (exported as Reset
// via export_test.go).
func reset() {
	stateMu.Lock()
	defer stateMu.Unlock()
	current.Store(disabledState())
}

func infof(logger *zap.SugaredLogger, format string, args ...any) {
	if logger != nil {
		logger.Infof(format, args...)
	}
}

func warnf(logger *zap.SugaredLogger, format string, args ...any) {
	if logger != nil {
		logger.Warnf(format, args...)
	}
}
