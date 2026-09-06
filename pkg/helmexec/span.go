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
// helmExecMarker marks contexts of invocations made through the execer
// funnel, so span classification is authoritative even for wrapper binaries
// whose name does not start with "helm".
type helmExecMarker struct{}

func startExecSpan(ctx context.Context, cmd string, args []string) (context.Context, trace.Span, bool) {
	if ctx == nil {
		ctx = context.Background()
	}

	name, attrs, isHelm := classifyExec(ctx, cmd, args)
	ctx, span := telemetry.Tracer(telemetry.ScopeHelm).Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, span, isHelm
}

// classifyExec builds the span name and attributes for one subprocess.
// Secret-bearing arguments are always redacted with the strict profile: span
// visibility must be at least as redacted as error messages (see redact.go).
func classifyExec(ctx context.Context, cmd string, args []string) (string, []attribute.KeyValue, bool) {
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
		return "helm.exec", attrs, true
	}
	return "os.exec", attrs, false
}

// isHelmBinary reports whether a base name refers to a helm binary ("helm",
// "helm3", custom builds like "helm-dev"). Non-matching binaries simply get
// os.exec spans, which is only a cosmetic distinction.
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
