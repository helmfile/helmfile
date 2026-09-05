package state

import (
	gocontext "context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/telemetry"
	"github.com/helmfile/helmfile/pkg/telemetry/otlptest"
)

// TestReleaseSpanExecNesting drives ReleaseStatuses through the real execer
// (with a shim binary that answers the version probe and otherwise exits 0)
// and telemetry enabled, then asserts the release span's subprocess span
// nests under it — the contract established by the
// HelmContext.Ctx/execWithContext funnel.
func TestReleaseSpanExecNesting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix shell shim")
	}

	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile test")

	// The shim satisfies helmexec.New's `helm version` probe and makes every
	// other invocation succeed without side effects.
	shim := filepath.Join(t.TempDir(), "helm-shim")
	require.NoError(t, os.WriteFile(shim, []byte("#!/bin/sh\ncase \"$1\" in version) echo 'v3.14.0' ;; esac\nexit 0\n"), 0o755))

	shell := &helmexec.ShellRunner{
		Logger: zap.NewNop().Sugar(),
		Ctx:    telemetry.CommandContext(),
	}
	helm, err := helmexec.New(shim, helmexec.HelmExecOptions{}, zap.NewNop().Sugar(), "", "", shell)
	require.NoError(t, err)

	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
		ReleaseSetSpec: ReleaseSetSpec{
			Releases: []ReleaseSpec{
				{Name: "demo", Namespace: "apps", Chart: "./charts/demo"},
			},
		},
	}

	errs := st.ReleaseStatuses(helm, 1)
	// `true` ignores all arguments and exits 0, so no errors are expected.
	require.Empty(t, errs)

	otlptest.ShutdownTelemetry(t)

	spans := rec.Spans(t)
	release := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool { return s.Name == "helmfile.release.status" }, "release status span")

	name, ok := otlptest.AttrString(release, "helmfile.release")
	require.True(t, ok)
	assert.Equal(t, "demo", name)

	// The shim binary's name starts with "helm", so its spans are helm.exec.
	// helmexec.New's version probe also produced one (parented at the command
	// span); the release's own subprocess must be the "status" one, nested
	// under the release span.
	exec := otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool {
		sub, ok := otlptest.AttrString(s, "helm.subcommand")
		return s.Name == "helm.exec" && ok && sub == "status"
	}, "release status subprocess span")
	assert.Equal(t, release.TraceId, exec.TraceId, "subprocess must join the release span's trace")
	assert.Equal(t, release.SpanId, exec.ParentSpanId, "subprocess must nest under the release span")
}

// TestSetTraceContext pins the parent wiring: per-release spans root at the
// trace context handed over by pkg/app (the load span), falling back to
// Background when unset.
func TestSetTraceContext(t *testing.T) {
	st := &HelmState{}
	assert.Equal(t, gocontext.Background(), st.releaseSpanParent(), "unset trace context must fall back to Background")

	type ctxKey struct{}
	ctx := gocontext.WithValue(gocontext.Background(), ctxKey{}, "x")
	st.SetTraceContext(ctx)
	assert.Equal(t, ctx, st.releaseSpanParent())
}
