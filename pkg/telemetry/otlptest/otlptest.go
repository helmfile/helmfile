// Package otlptest provides a minimal in-process OTLP/HTTP receiver for
// asserting on exported spans in tests, without an external collector.
package otlptest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/helmfile/helmfile/pkg/telemetry"
)

// Recorder is a minimal OTLP/HTTP+protobuf receiver capturing raw export
// requests.
type Recorder struct {
	Server *httptest.Server

	mu       sync.Mutex
	requests [][]byte
}

// NewRecorder starts a receiver bound to the test's lifetime.
func NewRecorder(t *testing.T) *Recorder {
	t.Helper()
	rec := &Recorder{}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	t.Cleanup(rec.Server.Close)
	return rec
}

// Spans decodes every captured request into a flat span list.
func (r *Recorder) Spans(t *testing.T) []*v1.Span {
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

// SetupTelemetry enables telemetry against the recorder, mirroring the
// cmd/root wiring (Setup + StartCommandSpan with the given command name), and
// shuts telemetry down on cleanup.
func SetupTelemetry(t *testing.T, rec *Recorder, command string) {
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
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", rec.Server.URL)

	telemetry.Setup(context.Background(), telemetry.Options{Enabled: true, Version: "test"})
	telemetry.StartCommandSpan(command)
	t.Cleanup(func() {
		_ = telemetry.Shutdown(context.Background(), nil, 0)
	})
}

// ShutdownTelemetry flushes and disables telemetry before Spans assertions;
// tests that keep running afterward must not rely on telemetry being active.
func ShutdownTelemetry(t *testing.T) {
	t.Helper()
	_ = telemetry.Shutdown(context.Background(), nil, 0)
}

// FindSpanWhere returns the first span matching pred, failing the test with a
// descriptive message otherwise.
func FindSpanWhere(t *testing.T, spans []*v1.Span, pred func(*v1.Span) bool, desc string) *v1.Span {
	t.Helper()
	for _, s := range spans {
		if pred(s) {
			return s
		}
	}
	t.Fatalf("no span matching %q (spans: %v)", desc, SpanNames(spans))
	return nil
}

func SpanNames(spans []*v1.Span) []string {
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}
	return names
}

// AttrString is the nil-safe attribute getter usable inside FindSpanWhere
// predicates, which run against spans that may lack the attribute entirely.
func AttrString(span *v1.Span, key string) (string, bool) {
	for _, kv := range span.Attributes {
		if kv.Key == key {
			return kv.Value.GetStringValue(), true
		}
	}
	return "", false
}

// HasAttr reports whether the span carries the attribute at all.
func HasAttr(span *v1.Span, key string) bool {
	_, ok := AttrString(span, key)
	return ok
}
