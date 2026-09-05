package helmexec

import (
	gocontext "context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/helmfile/helmfile/pkg/telemetry"
)

// otlpRecorder is a minimal OTLP/HTTP receiver capturing raw export requests
// so tests can assert on exported spans without an external collector.
type otlpRecorder struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests [][]byte
}

func newOTLPRecorder(t *testing.T) *otlpRecorder {
	t.Helper()
	rec := &otlpRecorder{}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		rec.mu.Lock()
		rec.requests = append(rec.requests, body)
		rec.mu.Unlock()
		// An empty 200 body unmarshals to a valid (empty) protobuf response.
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

// spans decodes every captured request into a flat span list.
func (r *otlpRecorder) spans(t *testing.T) []*v1.Span {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var spans []*v1.Span
	for _, body := range r.requests {
		var req tracepb.ExportTraceServiceRequest
		require.NoError(t, proto.Unmarshal(body, &req))
		for _, rs := range req.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				spans = append(spans, ss.Spans...)
			}
		}
	}
	return spans
}

func findSpanWhere(t *testing.T, spans []*v1.Span, pred func(*v1.Span) bool, desc string) *v1.Span {
	t.Helper()
	for _, s := range spans {
		if pred(s) {
			return s
		}
	}
	t.Fatalf("no span matching %q (spans: %v)", desc, spanNames(spans))
	return nil
}

func spanNames(spans []*v1.Span) []string {
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}
	return names
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

// attrString is the non-fatal variant used inside findSpanWhere predicates,
// which run against spans that may lack the attribute entirely.
func attrString(span *v1.Span, key string) (string, bool) {
	for _, kv := range span.Attributes {
		if kv.Key == key {
			return kv.Value.GetStringValue(), true
		}
	}
	return "", false
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

// setupTelemetryOTLP points telemetry at the recorder and mirrors the cmd/root
// wiring (Setup + StartCommandSpan); telemetry is reset on cleanup.
func setupTelemetryOTLP(t *testing.T, rec *otlpRecorder) {
	t.Helper()
	for _, key := range []string{
		"OTEL_TRACES_SAMPLER", "OTEL_TRACES_SAMPLER_ARG", "OTEL_PROPAGATORS",
		"OTEL_SDK_DISABLED", "TRACEPARENT", "TRACESTATE", "BAGGAGE",
		"OTEL_SERVICE_NAME", "OTEL_RESOURCE_ATTRIBUTES",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", rec.server.URL)

	telemetry.Setup(gocontext.Background(), telemetry.Options{Enabled: true, Version: "test"})
	telemetry.StartCommandSpan("helmfile test")
	t.Cleanup(func() {
		_ = telemetry.Shutdown(gocontext.Background(), nil, 0)
	})
}

// TestShellRunnerExecSpansExported drives ShellRunner.Execute through a real
// OTLP export and asserts names, attributes, error status, and parent linkage
// to the command span (i.e. the runner spans nest, never orphan).
func TestShellRunnerExecSpansExported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the unix true/false binaries")
	}

	rec := newOTLPRecorder(t)
	setupTelemetryOTLP(t, rec)

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

	require.NoError(t, telemetry.Shutdown(gocontext.Background(), nil, 0))

	spans := rec.spans(t)
	root := findSpanWhere(t, spans, func(s *v1.Span) bool { return s.Name == "helmfile test" }, "command span")

	success := findSpanWhere(t, spans, func(s *v1.Span) bool {
		cmd, ok := attrString(s, "exec.command")
		return s.Name == "os.exec" && ok && cmd == "true" && !hasAttr(t, s, "exec.redacted")
	}, "success os.exec span")
	if success.Status != nil {
		assert.Equal(t, v1.Status_STATUS_CODE_UNSET, success.Status.Code, "success span must not be marked error")
	}

	failure := findSpanWhere(t, spans, func(s *v1.Span) bool {
		cmd, ok := attrString(s, "exec.command")
		return ok && cmd == "false"
	}, "failure os.exec span")
	require.NotNil(t, failure.Status)
	assert.Equal(t, v1.Status_STATUS_CODE_ERROR, failure.Status.Code)
	assert.EqualValues(t, 1, spanAttr(t, failure, "exec.exit_code").Value.GetIntValue())

	redactedSpan := findSpanWhere(t, spans, func(s *v1.Span) bool { return hasAttr(t, s, "exec.redacted") }, "redacted os.exec span")
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

func hasAttr(t *testing.T, span *v1.Span, key string) bool {
	t.Helper()
	for _, kv := range span.Attributes {
		if kv.Key == key {
			return true
		}
	}
	return false
}
