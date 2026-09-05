package state

import (
	gocontext "context"
	"sort"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/helmfile/helmfile/pkg/telemetry"
)

// SetTraceContext sets the context used as the parent of per-release spans
// (pkg/app sets it to the helmfile.load span context right after loading a
// state file). A nil context keeps spans parented at Background — with
// tracing disabled, span starts are no-ops anyway.
func (st *HelmState) SetTraceContext(ctx gocontext.Context) {
	st.traceCtx = ctx
}

func (st *HelmState) releaseSpanParent() gocontext.Context {
	if st.traceCtx != nil {
		return st.traceCtx
	}
	return gocontext.Background()
}

// startReleaseSpan starts one helmfile.release.<verb> span for a release,
// parented from the state's trace context. The returned context is meant to
// be stamped into the per-release helmexec.HelmContext so the release's helm
// subprocesses nest under the span.
func (st *HelmState) startReleaseSpan(verb string, release *ReleaseSpec) (gocontext.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("helmfile.release", release.Name),
		attribute.String("helmfile.namespace", release.Namespace),
		attribute.String("helmfile.chart", release.Chart),
	}
	if release.Version != "" {
		attrs = append(attrs, attribute.String("helmfile.chart_version", release.Version))
	}
	if len(release.Labels) > 0 {
		attrs = append(attrs, attribute.StringSlice("helmfile.labels", sortedLabelPairs(release.Labels)))
	}

	ctx, span := telemetry.Tracer(telemetry.ScopeHelmfile).Start(st.releaseSpanParent(), "helmfile.release."+verb,
		trace.WithAttributes(attrs...),
	)
	return ctx, span
}

// endReleaseSpan ends a release span, recording err (when non-nil) as the
// span's error status. Ending an already-ended span is a no-op, so it is safe
// to call from every exit path of a worker-loop item.
func endReleaseSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func sortedLabelPairs(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+labels[k])
	}
	return pairs
}

// releaseErrAsError converts a *ReleaseError to error without the typed-nil
// trap: a nil *ReleaseError must become a nil error, or endReleaseSpan would
// call Error() on a nil pointer.
func releaseErrAsError(relErr *ReleaseError) error {
	if relErr == nil {
		return nil
	}
	return relErr
}

// doWithReleaseSpan runs do for one release under a helmfile.release.<verb>
// span, recording the returned error on the span. It is the convenience form
// used by the iterateOnReleases-based loops.
func (st *HelmState) doWithReleaseSpan(verb string, release ReleaseSpec, workerIndex int, do func(ReleaseSpec, int) error) error {
	_, span := st.startReleaseSpan(verb, &release)
	err := do(release, workerIndex)
	endReleaseSpan(span, err)
	return err
}
