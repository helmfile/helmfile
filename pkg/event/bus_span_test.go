package event

import (
	goContext "context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/telemetry"
	"github.com/helmfile/helmfile/pkg/telemetry/otlptest"
)

// TestHookSpanExported runs one hook through the default ShellRunner with
// telemetry enabled and asserts the helmfile.hook span and its child os.exec
// span (proving the hook's subprocess nests under the hook span, not the
// command span directly).
func TestHookSpanExported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the unix true binary")
	}

	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile test")

	bus := &Bus{
		Hooks: []Hook{
			{
				Name:    "migrate",
				Events:  []string{"presync"},
				Command: "true",
			},
			{
				Name:    "other-event-hook",
				Events:  []string{"postsync"},
				Command: "true",
			},
		},
		Logger: zap.NewNop().Sugar(),
		Fs:     filesystem.DefaultFileSystem(),
		Ctx:    goContext.WithoutCancel(telemetry.CommandContext()),
	}

	executed, err := bus.Trigger("presync", nil, nil)
	require.NoError(t, err)
	assert.True(t, executed, "the presync hook should have run")

	otlptest.ShutdownTelemetry(t)

	spans := rec.Spans(t)
	hook := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool { return s.Name == "helmfile.hook" }, "hook span")

	evt, ok := otlptest.AttrString(hook, "hook.event")
	require.True(t, ok)
	assert.Equal(t, "presync", evt)
	name, ok := otlptest.AttrString(hook, "hook.name")
	require.True(t, ok)
	assert.Equal(t, "migrate", name)

	// Exactly one hook span: the postsync hook must not run for presync.
	for _, s := range spans {
		if s.Name == "helmfile.hook" {
			id, _ := otlptest.AttrString(s, "hook.name")
			assert.Equal(t, "migrate", id, "only the matching hook should produce a span")
		}
	}

	// The hook's subprocess span nests under the hook span.
	exec := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool { return s.Name == "os.exec" }, "hook exec span")
	assert.Equal(t, hook.TraceId, exec.TraceId)
	assert.Equal(t, hook.SpanId, exec.ParentSpanId, "os.exec span must nest under the helmfile.hook span")
}

// TestHookSpanFailureMessageIsGeneric runs a failing hook and asserts the
// span's error status does not embed the rendered command or runner details.
func TestHookSpanFailureMessageIsGeneric(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the unix false binary")
	}

	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile test")

	bus := &Bus{
		Hooks: []Hook{
			{
				Name:    "fail",
				Events:  []string{"presync"},
				Command: "false",
				Args:    []string{"--secret", "hunter2"},
			},
		},
		Logger: zap.NewNop().Sugar(),
		Fs:     filesystem.DefaultFileSystem(),
		Ctx:    goContext.WithoutCancel(telemetry.CommandContext()),
	}

	_, err := bus.Trigger("presync", nil, nil)
	require.Error(t, err)

	otlptest.ShutdownTelemetry(t)

	hook := otlptest.FindSpanWhere(t, rec.Spans(t), func(s *v1.Span) bool { return s.Name == "helmfile.hook" }, "hook span")
	require.NotNil(t, hook.Status)
	assert.Equal(t, v1.Status_STATUS_CODE_ERROR, hook.Status.Code)
	assert.Equal(t, "hook failed", hook.Status.Message)
	assert.NotContains(t, hook.Status.Message, "false")
	assert.NotContains(t, hook.Status.Message, "hunter2")
}
