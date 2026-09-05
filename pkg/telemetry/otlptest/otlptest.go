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
	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/helmfile/helmfile/pkg/telemetry"
)

// Recorder is a minimal OTLP/HTTP+protobuf receiver capturing raw export
// requests. Traces and metrics arrive on different paths (/v1/traces,
// /v1/metrics) and are decoded separately.
type Recorder struct {
	Server *httptest.Server

	mu       sync.Mutex
	requests []capturedRequest
}

type capturedRequest struct {
	path string
	body []byte
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
		rec.requests = append(rec.requests, capturedRequest{path: r.URL.Path, body: body})
		rec.mu.Unlock()
		// An empty 200 body unmarshals to a valid (empty) protobuf response.
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(rec.Server.Close)
	return rec
}

// Spans decodes every captured /v1/traces request into a flat span list.
func (r *Recorder) Spans(t *testing.T) []*v1.Span {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var spans []*v1.Span
	for _, req := range r.requests {
		if req.path != "/v1/traces" {
			continue
		}
		var msg tracepb.ExportTraceServiceRequest
		require.NoError(t, proto.Unmarshal(req.body, &msg))
		for _, rs := range msg.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				spans = append(spans, ss.Spans...)
			}
		}
	}
	return spans
}

// Metrics decodes every captured /v1/metrics request into a flat metric list.
func (r *Recorder) Metrics(t *testing.T) []*metricsv1.Metric {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var metrics []*metricsv1.Metric
	for _, req := range r.requests {
		if req.path != "/v1/metrics" {
			continue
		}
		var msg metricspb.ExportMetricsServiceRequest
		require.NoError(t, proto.Unmarshal(req.body, &msg))
		for _, rm := range msg.ResourceMetrics {
			for _, sm := range rm.ScopeMetrics {
				metrics = append(metrics, sm.Metrics...)
			}
		}
	}
	return metrics
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

// FindMetric returns the metric with the given name, failing the test
// otherwise.
func FindMetric(t *testing.T, metrics []*metricsv1.Metric, name string) *metricsv1.Metric {
	t.Helper()
	for _, m := range metrics {
		if m.GetName() == name {
			return m
		}
	}
	t.Fatalf("metric %q not found", name)
	return nil
}

// HasAttr reports whether the span carries the attribute at all.
func HasAttr(span *v1.Span, key string) bool {
	_, ok := AttrString(span, key)
	return ok
}
