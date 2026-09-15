package helmexec

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/helmfile/helmfile/pkg/telemetry"
)

// startExecSpan starts one span per external process started by helmfile
// (helm invocations, hooks, helmfile plugins). It is effectively free when
// telemetry is disabled: telemetry.Tracer then returns the OTel no-op tracer.
// The returned context derives from ctx (or Background when nil) and may be
// used for the subprocess itself without changing cancellation semantics.
// markHelmExec returns ctx (or Background when nil) stamped with the
// helm-invocation marker used for span classification.
func markHelmExec(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, helmExecMarker{}, true)
}

// withRunnerCtx returns a runner (value or pointer ShellRunner form) whose
// context is transformed by f; non-ShellRunner runners pass through
// unchanged. ShellRunner has value receivers, so both forms satisfy Runner.
func withRunnerCtx(runner Runner, f func(context.Context) context.Context) Runner {
	switch shell := runner.(type) {
	case *ShellRunner:
		clone := *shell
		clone.Ctx = f(clone.Ctx)
		return &clone
	case ShellRunner:
		clone := shell
		clone.Ctx = f(clone.Ctx)
		return clone
	default:
		return runner
	}
}

// markHelmRunner returns a runner whose context carries the helm-invocation
// marker, so span classification and the helm duration metric do not rely on
// the executable basename.
func markHelmRunner(runner Runner) Runner {
	return withRunnerCtx(runner, markHelmExec)
}

// spanAttachedContext returns a context that keeps runnerCtx's cancellation
// chain but carries the span from spanCtx, so the subprocess span nests under
// the caller's span while the subprocess itself stays governed by the
// runner's own context (e.g. the kubedog safety valve). A nil runnerCtx falls
// back to spanCtx.
func spanAttachedContext(runnerCtx, spanCtx context.Context) context.Context {
	if runnerCtx == nil {
		return spanCtx
	}
	return trace.ContextWithSpan(runnerCtx, trace.SpanFromContext(spanCtx))
}

// helmExecMarker marks contexts of invocations made through the execer
// funnel, so span classification is authoritative even for wrapper binaries
// whose name does not start with "helm".
type helmExecMarker struct{}

func startExecSpan(ctx context.Context, cmd string, args []string) (context.Context, trace.Span, bool) {
	if ctx == nil {
		ctx = context.Background()
	}

	name, attrs := classifyExec(ctx, cmd, args)
	ctx, span := telemetry.Tracer(telemetry.ScopeHelm).Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, span, name == helmExecSpanName
}

// classifyExec builds the span name and attributes for one subprocess.
// Secret-bearing arguments are always redacted with the strict profile: span
// visibility must be at least as redacted as error messages (see redact.go).
// helmExecSpanName is the single source of truth for what counts as a helm
// invocation: the span name drives both classification and the helm duration
// metric gate.
const helmExecSpanName = "helm.exec"

func classifyExec(ctx context.Context, cmd string, args []string) (string, []attribute.KeyValue) {
	base := filepath.Base(cmd)
	isHelm := isHelmBinary(base)
	if marked, ok := ctx.Value(helmExecMarker{}).(bool); ok {
		isHelm = marked
	}
	attrs := []attribute.KeyValue{
		attribute.String("exec.command", base),
	}

	redacted := RedactArgs(args, RedactionStrict)
	for i, a := range redacted {
		// Positional arguments can be chart/repository URLs with embedded
		// credentials (AddRepo, RegistryLogin, OCI and go-getter refs).
		redacted[i] = RedactedRef(a)
	}
	attrs = append(attrs, attribute.StringSlice("exec.args", redacted))
	if !equalArgs(args, redacted) {
		attrs = append(attrs, attribute.Bool("exec.redacted", true))
	}

	if isHelm {
		if sub := helmSubcommand(args); sub != "" {
			attrs = append(attrs, attribute.String("helm.subcommand", sub))
		}
		return helmExecSpanName, attrs
	}
	return "os.exec", attrs
}

// isHelmBinary is the FALLBACK classifier: a base name starting with "helm"
// ("helm", "helm3", "helm-dev") for processes started outside the execer
// funnels (e.g. the version probe in helmexec.New). Invocations through the
// funnels are classified authoritatively by the helmExecMarker, so wrapper
// --helm-binary names classify correctly too.
func isHelmBinary(base string) bool {
	return strings.HasPrefix(base, "helm")
}

// helmSubcommand returns the best-effort helm subcommand ("upgrade",
// "repo", ...) from raw args. Bare flags are assumed to consume the next
// argument as their value (true for the global --kube-context/--kubeconfig
// helmfile prepends); a rare bare boolean flag before the subcommand can
// misattribute the value — cosmetic only.
func helmSubcommand(args []string) string {
	skipValue := false
	for _, a := range args {
		if skipValue {
			skipValue = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			skipValue = !strings.Contains(a, "=")
			continue
		}
		return a
	}
	return ""
}

// finishExecSpan records a finished process's outcome on its span and, for
// helm invocations (as classified by startExecSpan, wrapper binaries
// included), the helmfile.helm.exec.duration metric.
func finishExecSpan(span trace.Span, isHelm bool, args []string, start time.Time, err error) {
	if isHelm {
		telemetry.RecordHelmExecDuration(time.Since(start).Seconds(), helmSubcommand(args), err == nil)
	}
	if err == nil {
		return
	}
	var exitErr ExitError
	if errors.As(err, &exitErr) {
		span.SetAttributes(attribute.Int("exec.exit_code", exitErr.ExitStatus()))
	}
	// The raw error may embed command arguments and subprocess output; keep
	// the span description generic (the exit code is an attribute).
	span.SetStatus(codes.Error, "command failed")
}
