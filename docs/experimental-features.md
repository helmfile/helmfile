# Experimental Features

This document describes the experimental features that are available in Helmfile.

Any experimental feature may be removed or changed in a future release without notice.

Enable experimental features with the environment variable:

```bash
# Enable all experimental features
export HELMFILE_EXPERIMENTAL=true

# Enable a specific feature
export HELMFILE_EXPERIMENTAL=explicit-selector-inheritance
```

## explicit-selector-inheritance

By default, CLI selectors (e.g., `helmfile -l name=myapp sync`) are inherited by sub-helmfiles. This experimental feature changes the behavior so that sub-helmfiles without explicit `selectors` do **not** inherit selectors from their parent or the CLI.

When enabled:
* Sub-helmfiles without `selectors` do not inherit parent/CLI selectors
* Use `selectorsInherited: true` on a sub-helmfile to explicitly opt into inheriting selectors
* `selectors: []` selects all releases (same as current behavior)

See [Selectors and needs](releases.md#selectors) for detailed examples.

## HCL helmfile-values-file support

HCL language is supported for environment values files (`.hcl` suffix). This was introduced as experimental in PR #1423 and is now a stable feature. See [Environments](environments.md#hcl-specifications) for details.

## otel-tracing

OpenTelemetry tracing support: export a trace of a helmfile run to any OTLP-compatible backend. The current increments emit the command-level root span, state-loading spans (discover/load/render/parse), one span per external process (helm invocations, hooks, plugin execs), per-hook spans, and per-release spans for the sync/diff/delete/status/test/prepare paths (the remaining loops are being added incrementally) — see [the design proposal](https://github.com/helmfile/helmfile/blob/main/docs/proposals/otel-tracing.md).

Unlike the features above, `otel-tracing` is **not** gated by `HELMFILE_EXPERIMENTAL`. It is off by default and enabled explicitly per run:

```bash
helmfile --otel-tracing -e production apply
# or
export HELMFILE_OTEL_TRACING=true
```

All exporter, sampler, and propagator settings come from the standard `OTEL_*` environment variables (e.g. `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_TRACES_EXPORTER`). See [OpenTelemetry Tracing](otel.md) for details.
