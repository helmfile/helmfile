package helmexec

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

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
func startExecSpan(ctx context.Context, cmd string, args []string) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}

	name, attrs := execSpanAttributes(cmd, args)
	return telemetry.Tracer(telemetry.ScopeHelm).Start(ctx, name, trace.WithAttributes(attrs...))
}

// execSpanAttributes builds the span name and attributes for one subprocess.
// Secret-bearing arguments are always redacted with the strict profile: span
// visibility must be at least as redacted as error messages (see redact.go).
func execSpanAttributes(cmd string, args []string) (string, []attribute.KeyValue) {
	base := filepath.Base(cmd)
	attrs := []attribute.KeyValue{
		attribute.String("exec.command", base),
	}

	redacted := RedactArgs(args, RedactionStrict)
	attrs = append(attrs, attribute.StringSlice("exec.args", redacted))
	if !equalArgs(args, redacted) {
		attrs = append(attrs, attribute.Bool("exec.redacted", true))
	}

	if isHelmBinary(base) {
		if sub := helmSubcommand(args); sub != "" {
			attrs = append(attrs, attribute.String("helm.subcommand", sub))
		}
		return "helm.exec", attrs
	}
	return "os.exec", attrs
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

// finishExecSpan records a finished process's exit status on its span.
func finishExecSpan(span trace.Span, err error) {
	if err == nil {
		return
	}
	var exitErr ExitError
	if errors.As(err, &exitErr) {
		span.SetAttributes(attribute.Int("exec.exit_code", exitErr.ExitStatus()))
	}
	span.SetStatus(codes.Error, err.Error())
}
