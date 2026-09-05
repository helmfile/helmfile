# OpenTelemetry Tracing

Helmfile can export [OpenTelemetry](https://opentelemetry.io/) traces of a run, which is especially useful in CI/CD: see where time is spent, which helm invocations dominate, how label selectors fan out, and what to target for improvement.

Tracing is **experimental** and **off by default**. It currently emits the command-level root span, state-loading spans (discover/load/render/parse), one span per external process (helm invocations, hooks, plugin execs), and per-hook spans; per-release spans are being added incrementally.

## Enabling

```bash
helmfile --otel-tracing -e production -l tier=backend apply
```

or via environment variable (handy for CI):

```bash
export HELMFILE_OTEL_TRACING=true
helmfile sync
```

`--otel-tracing` overrides `HELMFILE_OTEL_TRACING` when specified.

## Configuration

Everything beyond the on/off switch uses the standard [OpenTelemetry environment variables](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/); helmfile defines no telemetry-specific variables of its own.

| Variable | Default | Notes |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTLP endpoint; use `http://host:4317` for gRPC |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` | or `grpc` |
| `OTEL_EXPORTER_OTLP_HEADERS` | — | e.g. `authorization=Bearer <token>` |
| `OTEL_TRACES_EXPORTER` | `otlp` | `otlp` \| `console` \| `none` |
| `OTEL_TRACES_SAMPLER` (+ `OTEL_TRACES_SAMPLER_ARG`) | `parentbased_always_on` | |
| `OTEL_SERVICE_NAME` | `helmfile` | |
| `OTEL_RESOURCE_ATTRIBUTES` | — | e.g. `cicd.pipeline=deploy,cicd.run_id=4821` |
| `OTEL_PROPAGATORS` | `tracecontext,baggage` | |
| `OTEL_SDK_DISABLED` | `false` | standard kill switch |

HTTP proxies (`HTTPS_PROXY`/`HTTP_PROXY`) are honored automatically by the Go runtime.

## Quick start without a collector

To inspect what would be exported, print spans as JSON on stdout:

```bash
OTEL_TRACES_EXPORTER=console helmfile --otel-tracing -l name=myrelease template
```

## Quick start with Jaeger

```bash
docker run --rm -p 16686:16686 -p 4318:4318 jaegertracing/all-in-one:latest

OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
  helmfile --otel-tracing -e production apply
# Open http://localhost:16686 and pick service "helmfile"
```

## What gets traced

The root span is named after the command (`helmfile apply`, `helmfile sync`, ...) and carries `helmfile.command`, `helmfile.file`, `helmfile.environment`, and `helmfile.selectors` attributes, plus `helmfile.exit_code` and the error (if any) at the end.

Every external process helmfile starts — each helm invocation, hook command, and plugin exec — gets its own span nested under the command span: `helm.exec` (with `helm.subcommand`) for helm binaries, `os.exec` otherwise, both carrying `exec.command`, redacted `exec.args`, and `exec.exit_code` on failure.

State loading is traced too: `helmfile.discover_states`, one `helmfile.load` per state file (including nested helmfiles), with `helmfile.render` and `helmfile.parse` children — rendering is frequently the hidden time sink. Each hook execution produces a `helmfile.hook` span (with `hook.event` and `hook.name`) that its subprocess span nests under.

Per-release spans are on the roadmap; see [the design proposal](proposals/otel-tracing.md) for the planned span taxonomy.

Secret-bearing command arguments (`--set`, `--set-file`, `--username`, `--password`, ...) are never recorded in span attributes — they are masked before export.

## CI trace correlation

If your CI system injects a W3C `TRACEPARENT` environment variable into jobs, helmfile joins that trace automatically, so helmfile spans appear inside your pipeline's trace.

## Troubleshooting

- **No spans arrive**: check `OTEL_EXPORTER_OTLP_ENDPOINT`/protocol match your collector (4318 for `http/protobuf`, 4317 for `grpc`); verify with `OTEL_TRACES_EXPORTER=console`.
- **Tracing breaks nothing**: exporter/sampler misconfiguration and export failures never fail a helmfile run; at worst you get no spans.
- **Still stuck?** Set `OTEL_SDK_DISABLED=false` explicitly and rerun with `--log-level debug`.
