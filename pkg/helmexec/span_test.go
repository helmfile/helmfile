package helmexec

import (
	"context"
	"io"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/helmfile/helmfile/pkg/telemetry"
	"github.com/helmfile/helmfile/pkg/telemetry/otlptest"
)

// TestShellRunnerExecSpansExported drives ShellRunner.Execute through a real
// OTLP export and asserts names, attributes, error status, and parent linkage
// to the command span (i.e. the runner spans nest, never orphan).
func TestShellRunnerExecSpansExported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the unix true/false binaries")
	}

	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile test")

	shell := ShellRunner{
		Logger: NewLogger(io.Discard, "warn"),
		Ctx:    telemetry.CommandContext(),
	}

	out, err := shell.Execute("true", nil, nil, false)
	require.NoError(t, err)
	assert.Empty(t, out)

	// A failing command must surface its exit code and error status.
	_, err = shell.Execute("false", nil, nil, false)
	var exitErr ExitError
	require.ErrorAs(t, err, &exitErr)

	// Secret-bearing arguments must be redacted on the span.
	_, err = shell.Execute("true", []string{"--set", "secret=1", "--set=also=2"}, nil, false)
	require.NoError(t, err)

	otlptest.ShutdownTelemetry(t)

	spans := rec.Spans(t)
	root := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool { return s.Name == "helmfile test" }, "command span")

	success := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool {
		cmd, ok := otlptest.AttrString(s, "exec.command")
		return s.Name == "os.exec" && ok && cmd == "true" && !otlptest.HasAttr(s, "exec.redacted")
	}, "success os.exec span")
	if success.Status != nil {
		assert.Equal(t, v1.Status_STATUS_CODE_UNSET, success.Status.Code, "success span must not be marked error")
	}

	failure := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool {
		cmd, ok := otlptest.AttrString(s, "exec.command")
		return ok && cmd == "false"
	}, "failure os.exec span")
	require.NotNil(t, failure.Status)
	assert.Equal(t, v1.Status_STATUS_CODE_ERROR, failure.Status.Code)
	assert.EqualValues(t, 1, spanAttr(t, failure, "exec.exit_code").Value.GetIntValue())

	redactedSpan := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool { return otlptest.HasAttr(s, "exec.redacted") }, "redacted os.exec span")
	assert.Equal(t, []string{"--set", redactedArg, "--set=" + redactedArg}, spanAttrStrings(t, redactedSpan, "exec.args"))

	// Every os.exec span must be a child of the command span, proving the
	// runner-span nesting (no orphan traces).
	for _, s := range spans {
		if s.Name == "os.exec" {
			assert.Equal(t, root.TraceId, s.TraceId, "os.exec span must join the command trace")
			assert.Equal(t, root.SpanId, s.ParentSpanId, "os.exec span must nest under the command span")
		}
	}
}

func spanAttr(t *testing.T, span *v1.Span, key string) *commonpb.KeyValue {
	t.Helper()
	for _, kv := range span.Attributes {
		if kv.Key == key {
			return kv
		}
	}
	t.Fatalf("attribute %q not found on span %q", key, span.Name)
	return nil
}

func spanAttrStrings(t *testing.T, span *v1.Span, key string) []string {
	t.Helper()
	values := spanAttr(t, span, key).Value.GetArrayValue().GetValues()
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.GetStringValue())
	}
	return out
}

// TestRunnerWithCtx pins the per-call context swap that lets helm subprocess
// spans nest under per-release spans: the shared runner is never mutated, a
// clone carries the ctx, and non-ShellRunner runners pass through unchanged.
func TestRunnerWithCtx(t *testing.T) {
	baseCtx := context.Background()
	releaseCtx := context.WithValue(baseCtx, ctxKey{}, "release")

	shell := &ShellRunner{Ctx: baseCtx}
	helm := &execer{runner: shell}

	clone := helm.runnerWithCtx(releaseCtx)
	got, ok := clone.(*ShellRunner)
	require.True(t, ok)
	assert.Equal(t, releaseCtx, got.Ctx, "clone must carry the per-call ctx")
	assert.Equal(t, context.Background(), shell.Ctx, "shared runner must not be mutated")

	// A typed nil context (distinct from a nil literal, per staticcheck) must
	// still return the original runner.
	var nilCtx context.Context
	assert.Same(t, shell, helm.runnerWithCtx(nilCtx), "nil ctx must return the original runner")
}

type ctxKey struct{}
