# Proposal: OpenTelemetry Tracing Support

- **Issue**: [#2767 — feat-request: OpenTelemetry (tracing) support](https://github.com/helmfile/helmfile/issues/2767)
- **Status**: Draft
- **Origin discussion**: [#2758](https://github.com/helmfile/helmfile/discussions/2758)

## 1. Summary

Add opt-in OpenTelemetry (OTel) distributed tracing to helmfile. When enabled, a helmfile
run produces a trace whose spans cover the full execution timeline — state-file discovery,
template rendering, per-release operations, hooks, and every helm subprocess helmfile itself
starts — exported via OTLP to any OTel-compatible backend (Jaeger, Tempo, Zipkin,
Honeycomb, Datadog, Grafana Cloud, cloud-vendor collectors, ...).

This directly serves the motivating use case from CI/CD: *see where time is spent, what runs
most often (label selectors), and what to target for improvement* — the same capability
Terragrunt ships today ([Terragrunt OTel docs](https://docs.terragrunt.com/troubleshooting/open-telemetry/)).

When disabled (the default), behavior and performance are identical to today: a no-op tracer
provider, no goroutines, no network, no exported spans (explicit guarantees in §7).

### Example resulting trace

```
helmfile sync  file=helmfile.yaml env=production          (root, ~2m)
├─ helmfile.discover_states                                (~5ms)
├─ helmfile.load file=helmfile.yaml                        (~800ms)
│  ├─ helmfile.render pass=values                          (~400ms)
│  └─ helmfile.parse                                       (~50ms)
├─ helmfile.repos.update                                   (~4s)
│  └─ helm.exec subcommand="repo update"                   (~4s)
├─ helmfile.release.prepare release=gateway chart=gateway  (~9s)
│  └─ helm.exec subcommand="dependency build"              (~8s)
├─ helmfile.release.sync release=envoy ns=ingress          (~21s)
│  ├─ helmfile.hook event=presync                          (~2s)
│  │  └─ os.exec command="./migrate.sh"                    (~2s)
│  ├─ helm.exec subcommand="upgrade --install envoy ..."   (~18s)
│  └─ helmfile.hook event=postsync                         (~1s)
├─ helmfile.release.sync release=api ns=apps               (~35s)
└─ helmfile.wait release=api (kubedog)                     (~30s)
```

## 2. Goals and non-goals

### Goals

1. **Opt-in, zero-cost-when-off**: tracing disabled by default; no measurable overhead or
   behavior change when not requested.
2. **Standard OTLP export** configured through the [OTel environment-variable
   specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
   — no helmfile-specific duplicate knobs for endpoints/protocols/headers/sampling.
3. **Meaningful hierarchy**: spans nest (command → state file → release → hook/subprocess)
   so the critical path of a sync/apply is visible at a glance — *including* the kubedog and
   hook paths, which today run on detached contexts (§4.3).
4. **Safe by default**: span attributes never contain secret-bearing values
   (`--set` values, registry credentials, secret refs).
5. **CI-friendly**: W3C `tracecontext` propagation so a CI system that injects
   `TRACEPARENT` gets spans correlated into its own trace.

### Non-goals (for the initial implementation)

- OTel **metrics** and **log export** (the SDK setup leaves room; see §12 for follow-ups).
- Tracing *inside* the helm binary or cluster-side (helm/chart hooks are separate processes;
  context injection into them is an open question, §14).
- Fixing the pre-existing cancellation gaps discovered during design (kubedog path rooted at
  `context.Background()`, hooks at `context.TODO()` — §4.3). This proposal bridges them for
  *trace context only*, with byte-identical cancellation semantics; semantic fixes are
  reported separately.
- Any helmfile-config-file surface (`helmfile.yaml`) for telemetry — CLI flags + env vars only,
  matching how observability tooling is usually injected by the platform, not the state author.

## 3. User-facing design

### 3.1 Activation

| Mechanism | Default | Description |
|---|---|---|
| `--otel-tracing` (persistent flag) | `false` | Enable tracing for this run |
| `HELMFILE_OTEL_TRACING=true` (env) | `false` | Same, via environment (CI-friendly) |

The feature ships as **experimental** initially and is listed in
`docs/experimental-features.md` (see §11 Rollout). We do *not* require
`HELMFILE_EXPERIMENTAL=otel-tracing`: tracing is purely additive, invisible when off, and
gating it twice would complicate CI adoption. The experimental label sets stability
expectations only. (Note: this differs from the current entries in that document, which are
gated by `HELMFILE_EXPERIMENTAL`; the entry will say so explicitly.)

### 3.2 Exporter & SDK configuration — standard OTel env vars only

When `--otel-tracing` is set, helmfile initializes the OTel SDK honoring the standard
environment variables. Exporter selection is delegated to
`go.opentelemetry.io/contrib/exporters/autoexport` (already in the module graph, §8) rather
than hand-rolled, so helmfile maintains no exporter-construction code of its own:

| Variable | Default in helmfile | Read by | Notes |
|---|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | otlp client (SDK) | default follows the protocol: 4318 with `http/protobuf` (below), 4317 with `grpc` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` / `..._TRACES_PROTOCOL` | `http/protobuf` | autoexport | `grpc`, `http/protobuf` (traces-specific var wins; `http/json` is not supported by autoexport traces in v0.67.0) |
| `OTEL_EXPORTER_OTLP_HEADERS` / `..._TRACES_HEADERS` | — | otlp client (SDK) | e.g. collector auth tokens |
| `OTEL_EXPORTER_OTLP_TIMEOUT` / `..._TRACES_TIMEOUT` | `10s` | otlp client (SDK) | per-export timeout |
| `OTEL_EXPORTER_OTLP_INSECURE` | per endpoint scheme | otlp client (SDK) | plaintext export for local collectors |
| `OTEL_TRACES_EXPORTER` | `otlp` | autoexport | `otlp` \| `console` \| `none` (verified against autoexport v0.67.0; more values possible via `RegisterSpanExporter`) |
| `OTEL_TRACES_SAMPLER` (+`..._ARG`) | `parentbased_always_on` | our wrapper | standard samplers (the Go SDK core does not parse this env itself) |
| `OTEL_SERVICE_NAME` | `helmfile` | SDK resource | service identity (`resource` `WithFromEnv`; verified in sdk v1.44.0 `resource/env.go`) |
| `OTEL_RESOURCE_ATTRIBUTES` | — | SDK resource | e.g. `deployment.environment=ci,cicd.pipeline=release` (same verified reader) |
| `OTEL_PROPAGATORS` | `tracecontext,baggage` | our wrapper | extract parent from CI-injected `TRACEPARENT` (SDK core does not parse this env either) |
| `OTEL_SDK_DISABLED` | `false` | our wrapper | standard kill switch (the Go SDK does not read this one itself) |

`console` (JSON spans on **stdout**, via `stdouttrace`) exists for local debugging without a
collector — note stdout, so it does not interleave with helmfile's stderr logs;
`none` produces no export (used by tests and for pure-propagation setups).

HTTP proxies (`HTTPS_PROXY`/`HTTP_PROXY`) are honored automatically by the Go HTTP/gRPC
stacks — no helmfile-specific proxy configuration.

Minimal CI example:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://otel.example.com"
export OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer ${OTEL_TOKEN}"
export OTEL_SERVICE_NAME="helmfile-ci"
export OTEL_RESOURCE_ATTRIBUTES="cicd.pipeline=deploy,cicd.run_id=4821"
helmfile --otel-tracing -e production -l tier=backend apply
```

## 4. Architecture

### 4.1 New package `pkg/telemetry`

Single, dependency-light façade over the OTel SDK. No other package imports OTel SDK
exporters directly.

```
pkg/telemetry/
├── telemetry.go      // Setup/Shutdown lifecycle, CommandContext(), Tracer(name)
├── exporter.go       // thin autoexport wiring + propagators (no exporter construction)
└── telemetry_test.go
```

Public surface sketch:

```go
package telemetry

type Options struct {
    Enabled bool
    Version string          // helmfile version, recorded as service.version
    Logger  *zap.SugaredLogger // for one-line diagnostics, never span data
}

// Setup initializes the global provider. Idempotent; a no-op when
// opts.Enabled is false. Exporter/sampler misconfiguration and
// OTEL_SDK_DISABLED=true degrade to disabled with a warning — telemetry
// problems never fail a helmfile run.
func Setup(ctx context.Context, opts Options)

// StartCommandSpan starts the root span for one command invocation, joins a
// remote parent from TRACEPARENT/TRACESTATE/BAGGAGE env vars, and makes its
// context the one returned by CommandContext. No-op when disabled.
func StartCommandSpan(command string, attrs ...attribute.KeyValue)

// CommandContext returns the root command span's context, or
// context.Background() when telemetry is disabled. Single source of truth
// for deriving App.ctx.
func CommandContext() context.Context

// Tracer returns a tracer for the given instrumentation scope. Never nil:
// returns the OTel no-op tracer when telemetry is disabled, so callers need
// no `if enabled` branches. Sanctioned scopes: ScopeHelmfile, ScopeHelm.
func Tracer(name string) trace.Tracer

// Shutdown ends the command span (recording runErr and exitCode), flushes
// buffered spans bounded by ctx, and reverts to disabled. Idempotent and
// nil-safe (safe before Setup, twice, or on a signal that raced Setup).
// ShutdownTimeout is the recommended flush bound.
func Shutdown(ctx context.Context, runErr error, exitCode int) error
```

Design constraints for maintainability:

- **No domain knowledge in this package.** Span-attribute redaction rules live with the
  existing redaction code in `pkg/helmexec` (§6); telemetry only consumes the result.
  A telemetry package that knows about `--set` semantics would be a layering violation.
- Sanctioned instrumentation scopes are only `"helmfile"` (app/state layer) and `"helm"`
  (helmexec layer), documented in `telemetry.go`.

### 4.2 Lifecycle

```
main.go                          cmd/root.go
──────                          ───────────
rootCmd.Execute() ─────────────► PersistentPreRunE:
                                    logger setup (existing)
                                    telemetry.Setup(ctx, opts)      ← provider + exporters
                                    start root span "helmfile <sub>"
   RunE / subcommand execution …   (child spans attach via context)
errChan <- Execute()
                                  ┌────────────────────────────────┐
shutdown:                         │ end root span (status=error on │
  rootSpan.End()                  │ failure, exit code attr)       │
  provider.Shutdown(ctx 5s) ◄─────┘                                │
```

- `PersistentPreRunE` (`cmd/root.go`) is the natural init point: it already centralizes
  logger construction from `GlobalOptions`, and receives the `*cobra.Command`, so
  `c.Name()` gives the root span name (`helmfile sync`, ...).
- **`App.ctx` derivation requires no signature changes.** `app.New(conf)` is called from
  every subcommand (`cmd/*.go`, 22 call sites) and currently roots its context at
  `context.Background()` (`pkg/app/app.go`). Instead of threading a parameter through all
  callers, `app.New` replaces that single `context.Background()` with
  `telemetry.CommandContext()` — Background-identical when tracing is off, span-rooted
  when on. Cancellation is unaffected (`context.WithCancel` on either parent behaves the
  same for SIGINT handling in `main.go`).
- **Shutdown must run even on failure and on signals.** The end of
  `rootCmd.Execute()` in `main.go` (both the `errChan` and the `SIGINT`/`SIGTERM` paths;
  on the error path *before* `errors.HandleExitCoder`, which terminates via `OsExiter`,
  and on the signal path after `app.CleanWaitGroup.Wait()` and *before*
  `os.Exit(130/143)`) calls the returned shutdown func with a 5s-timeout context so buffered
  spans are flushed. The stored shutdown must be nil-safe — a signal arriving before
  `PersistentPreRunE` completed (i.e. before `Setup`) must be a no-op, never a panic. This
  is the one behavior that is easy to get wrong and is explicitly tested (§10).
- Setup failures (e.g. unusable exporter configuration) are logged as a warning and
  **do not fail the run** — telemetry must never break deployments. Export errors after
  startup surface only through OTel's own error handler, wired to the zap logger.

### 4.3 Verified context-reality map (as of this writing)

All claims below were checked against the code; line numbers are anchors for reviewers:

| Path | Today | Consequence for tracing |
|---|---|---|
| cmd → app | `app.New` roots at `context.Background()`; `ctx, Cancel = WithCancel(ctx)` (`pkg/app/app.go`, `New`) | span must be injected here (§4.2) |
| app → all helm execs | `getHelm()` constructs the `ShellRunner` with `Ctx: a.ctx` (`pkg/app/app.go:1008`); the resulting `execer` is **cached per (helm binary, kube-context)** in `a.helms` and shared by all releases and workers (`pkg/app/app.go:982–1021`) | once `App.ctx` is span-rooted, every non-kubedog helm call nests automatically, with **zero changes** to `getHelm`. The shared-instance cache is also why per-release contexts must ride per-call parameters, never mutation of the shared execer |
| kubedog path (sync with tracking) | `startBackgroundKubedogTracking(gocontext.Background(), …)` (`pkg/state/state.go:1294`) → `bufferHelmOutput` derives `releaseCtx := context.WithCancel(ctx)` and swaps it in via `execer.WithContext(releaseCtx)` (`pkg/state/helmx.go:363–365`) | helm execs on this path run on a **Background-rooted** context; runner-level spans would become **orphan traces**. Bridged in §4.4 |
| hooks | both `event.Bus` constructions (`triggerGlobalReleaseEvent`, `triggerReleaseEvent`, `pkg/state/state.go:3666, 3703`) duplicate the same literal and pass **no** `Runner`, so the default kicks in: `ShellRunner{Dir: bus.BasePath, Logger: bus.Logger, Ctx: goContext.TODO()}` with an inline comment acknowledging it should be `app.Ctx` (`pkg/event/bus.go:61–71`) | hook execs are detached; spans would be orphans. Bridged in §4.4 |
| non-kubedog release workers | release loops (`SyncReleases` etc., `pkg/state/state.go:1212 ff.`) call the shared `helmexec.Interface` with a `HelmContext` (`pkg/helmexec/context.go`) that carries **no go-context** | per-release spans need the §4.4 mechanism |
| subprocess funnel | exactly three `ShellRunner` construction sites exist (verified exhaustive): `pkg/app/app.go:129` (`Init`, `Ctx: a.ctx`), `pkg/app/app.go:1006` (`getHelm`, `Ctx: a.ctx`), and the hooks default (`pkg/event/bus.go:62`, `Ctx: TODO` — §4.4 bridge). Every external process helmfile itself starts goes through `Execute`/`ExecuteStdIn` (`pkg/helmexec/runner.go`); helm commands additionally funnel through `execer.exec()` (`pkg/helmexec/exec.go:1207`). Exception: kustomize executes inside the chartify library, outside this funnel (§12) | one instrumentation point covers everything except chartify-internal execs; spans nest wherever the runner's `Ctx` carries a span |

Two pre-existing gaps surfaced by this analysis — kubedog tracking not being cancellable via
`App.ctx`, and hooks likewise — are **out of scope** for this proposal beyond trace-context
bridging (§4.4), because fixing their *cancellation* semantics would be a behavior change.
They should be reported as separate issues.

### 4.4 Context plan

**Phase 1 (no state-package changes):**

1. Root span ctx via `telemetry.CommandContext()` → `app.New` (§4.2). Every `a.ctx`-rooted
   exec nests. Discover/load/render spans in `pkg/app` also need no signature changes:
   the `helmfile.load` span starts inside `loadDesiredStateFromYamlWithBaseDir`
   (`pkg/app/app.go:932`) with `a.ctx` as parent — its two call sites (app.go:1045 and
   the nested-helmfile path at app.go:1280) are thereby both covered — and `helmfile.render`
   children parent through an unexported `ctx` field on the `desiredStateLoader` struct
   (`pkg/app/desired_state_file_loader.go:29`), set once at its single construction site
   (`pkg/app/app.go:938`). Zero method-signature changes, zero exported API.
2. Runner-level spans in `ShellRunner.Execute`/`ExecuteStdIn` nest for all non-kubedog,
   non-hook execs automatically.
3. **Orphan-bridge for kubedog and hooks, with identical cancellation semantics:**
   - `pkg/state/state.go:1294`: pass `context.WithoutCancel(telemetry.CommandContext())`
     instead of `gocontext.Background()`. `WithoutCancel` preserves values (the span)
     while dropping cancellation — and `Background` never carried cancellation anyway, so
     SIGINT/timeout behavior is **bit-for-bit unchanged**; only trace context is added.
     (Phase 1 uses the root span from `telemetry.CommandContext()`, which requires no new
     plumbing in `pkg/state`; phase 2 re-parents under the per-release `st.traceCtx`.)
   - `pkg/event/bus.go`: add an optional `Ctx context.Context` field to `Bus`; the default
     runner construction uses `bus.Ctx` when set, `TODO` when nil (so behavior is unchanged
     for any nil-Ctx caller). The two construction sites in `pkg/state/state.go:3666, 3703`
     set it from the same span-rooted, cancel-stripped context. `Dir`/`Logger` wiring of
     the default runner is untouched.

**Phase 2 (per-release spans, still zero changes to `helmexec.Interface` signatures):**

- Start `helmfile.release.<verb>` spans in the seven release-worker loops — six
  `scatterGather` sites in `pkg/state/state.go` (`prepareSyncReleases`:894,
  `DeleteReleasesForSync`:1135, `SyncReleases`:1241, `PrepareCharts`:2328,
  `prepareDiffReleases`:3068, `DiffReleases`:3266) plus `iterateOnReleases`
  (`pkg/state/state_run.go:59`, the shared loop behind test/lint/unittest-style
  iteration) — all following the same `scatterGather` shape. (An eighth `scatterGather`
  sites, `scatterGatherEnvSecretFiles` at `pkg/state/create.go:486`, decrypts environment
  secrets rather than processing releases; an optional `helmfile.env_secrets` span there
  is a follow-up in the spirit of §4.5 item 6.)
- Release spans are rooted via a new unexported `traceCtx` field on `HelmState`, exposed
  through a purely additive exported setter called by `pkg/app` right after state
  creation. (Why a setter and not a constructor parameter: `st.logger` is injected through
  `state.NewCreator` — a 9-positional-parameter exported function,
  `pkg/state/create.go:80` — so threading a context through it would churn a public
  signature used by the app loader; an additive setter touches no existing signature and
  defaults to nil, i.e. current behavior.)
- Carry the span context per call by adding an optional `Ctx context.Context` field to
  `HelmContext` (`pkg/helmexec/context.go`), stamped **inside** `createHelmContext`
  (`pkg/state/state.go:3168`) so all eight call sites (state.go:1007, 1026, 1147, 1261,
  3034, 3184, 3358, 3374) inherit it without per-call-site edits.
- Funnel it inside `helmexec`: the `execer` methods that take a `HelmContext` pass it to a
  new internal helper `execCtx(ctx, args, env, live)` beside the existing
  `exec`/`execStdIn` funnels (`pkg/helmexec/exec.go:1207`), falling back to the runner's
  context when `HelmContext.Ctx` is nil. Cancellation stays exactly as today:
   `HelmContext.Ctx` derives from `st.traceCtx` ← `a.ctx`, and non-kubedog releases
   already run under `a.ctx` (§4.3), so propagation is unchanged; the kubedog path stays
   cancel-detached via the §4.4 step-3 `WithoutCancel` bridge. The `exectest.Helm` fake is unaffected: app tests
  pre-seed `App.helms` with it (`pkg/app/app_template_test.go:115–116`), so it replaces
  the whole `helmexec.Interface` and bypasses `execer` internals entirely.
  *Why per-call threading rather than the existing `WithContext` clone: `WithContext`
  (`pkg/helmexec/exec.go:251`) is suited to whole-execution substitution (kubedog path),
  but release workers share one cached execer across concurrent workers (§4.3), so a
  per-release context must travel with the per-release `HelmContext` parameter.*

### 4.5 Instrumentation points (in priority order)

1. **`helmexec.ShellRunner.Execute` / `ExecuteStdIn`** (`pkg/helmexec/runner.go`) — the single
   choke point for *every* external process started by helmfile itself: helm invocations,
   hooks, and helmfile plugin execs. One span per subprocess: name `helm.exec` when `cmd`
   is the helm binary, else `os.exec`. The one exception is kustomize, which runs inside
   the `github.com/helmfile/chartify` library (see §5/§12). This instrumentation alone
   delivers most of the requested value (where does time go).
2. **Root command span** — `cmd/root.go` (see §4.2).
3. **State loading** — span `helmfile.load` started inside
   `loadDesiredStateFromYamlWithBaseDir` (`pkg/app/app.go:932`, covering both callers
   incl. nested helmfiles), with `helmfile.render`/`helmfile.parse` children in
   `two_pass_renderer.go` parented via the loader-struct `ctx` field (§4.4 step 1) —
   rendering is frequently the hidden time sink.
4. **Per-release operations** — release loops in `pkg/state/state.go` (phase 2, §4.4).
5. **Hooks** — `pkg/event/bus.go` `Trigger` (`pkg/event/bus.go:56`): span `helmfile.hook`
   with `hook.event`, `hook.name` attributes; naturally parents the `os.exec` span of the
   hook command once the §4.4 bridge is in place.
6. **(Phase 2)** kubedog wait spans, vals/remote secret resolution, `helm repo` retry loops.

## 5. Span taxonomy

| Span name | Attributes (beyond standard `otel.*`) | Notes |
|---|---|---|
| `helmfile <subcommand>` (root) | `helmfile.command`, `helmfile.file`, `helmfile.environment`, `helmfile.selectors`, `helmfile.exit_code` | `error` status + recorded error on failure. Service identity (`service.name`, `service.version`) lives on the OTel resource, not on spans |
| `helmfile.discover_states` | `helmfile.path` | `findDesiredStateFiles` (`pkg/app/app.go:1642`) |
| `helmfile.load` | `helmfile.state_file` | one per file in `helmfile.d`, nested helmfiles |
| `helmfile.render` | `helmfile.state_file`, `helmfile.pass`=`values`\|`main` | two-pass rendering (`pkg/app/two_pass_renderer.go`) |
| `helmfile.repos.update` | — | wraps `helm repo update` |
| `helmfile.release.prepare` | `helmfile.release`, `helmfile.namespace`, `helmfile.chart`, `helmfile.chart_version` | chart pull/build/registry login |
| `helmfile.release.sync` / `.diff` / `.template` / `.delete` / `.test` / `.lint` / `.unittest` | same as above + `helmfile.labels` | one per selected release (phase 2) |
| `helmfile.hook` | `hook.event` (presync/…), `hook.name` | |
| `helm.exec` | `helm.subcommand`, `exec.exit_code`, `exec.redacted` | runner level; **release identity is not derivable here** — it comes from the parent release span (phase 2). Until then these spans sit directly under root/load |
| `os.exec` | `exec.command`, `exec.exit_code` | hooks, helmfile plugin execs. **Kustomize is not visible here**: it executes inside `github.com/helmfile/chartify`, outside `ShellRunner`'s reach (§12) |
| `helmfile.wait` | `helmfile.release` | kubedog tracking (phase 2) |

Attribute values are strings/ints only; no structured payloads, no output capture in spans
(output already flows through logs).

## 6. Security and redaction

Traces leave the machine they run on. Ground rules, checked against what exists today:

1. **The existing redaction is not sufficient on its own.** The exit-error path
   (`pkg/helmexec/exit_error.go:8–20`) redacts only the argument *following* a flag whose
   name starts with `--set` (two-argument form). It does not cover the single-argument
   `--set=key=value` form, nor credential-bearing flags such as `--username` or
   `--password`. (Registry passwords themselves already travel via stdin —
   `--password-stdin`, `pkg/helmexec/exec.go` `RegistryLogin` — but usernames appear in
   args.) Therefore:
2. **One shared redaction implementation, two profiles — spans get the strict superset
   without touching existing error output.** Extract the exit-error redaction into an
   exported helper in `pkg/helmexec` (keeping it in the domain that owns flag semantics)
   with two profiles:
   - `legacy`: byte-identical to today's exit-error behavior — the current goldens in
     `pkg/helmexec/exit_error_test.go` (which assert the exact `--set` / `*** STRIP ***`
     shape) keep passing unchanged. The exit-error path switches to this profile, so
     error messages are unchanged.
   - `strict`: the superset required for spans — all `--set*` forms including
     `--set=k=v`, plus `--username`, `--password`, `--key-file`, `--set-file`, ...
   Both profiles are the same code path, so the span view is guaranteed at least as
   redacted as the error view. *Unifying* the two profiles (i.e. tightening exit-error
   messages too) would change observable output and is deliberately deferred to a
   separate follow-up PR with its own test updates — this proposal changes no existing
   message content.
3. **Never record**: `vals://`-resolved values, environment variables, exporter headers.
   `OTEL_EXPORTER_OTLP_HEADERS` is the only place collector credentials live; helmfile
   never logs it.
4. Chart/repo URLs go through the existing `redactedURL` (`pkg/helmexec/exec.go:184`) —
   credentials embedded in URLs are masked.
5. Release *names*, namespaces, chart names, and label selectors are assumed non-secret
   (consistent with existing helmfile log output).
6. Note for reviewers: `execer.exec` today logs the *full unredacted* command line at Debug
   level (`pkg/helmexec/exec.go:1218`). Span attributes deliberately do **not** mirror that
   log line; §10 pins this with a redaction test.

## 7. Performance and zero-impact guarantees

**When disabled (the default):**
- `telemetry.Tracer` returns the OTel no-op tracer. No-op span start/end is a few ns and
  allocation-free; provider setup, exporters, and the batch worker goroutine never start.
- `telemetry.CommandContext()` returns `context.Background()`; `app.New` behaves exactly as
  today. No flag checks appear in hot loops.

**When enabled:**
- Standard SDK BatchSpanProcessor (5s interval / 512-span batches). A
  `sync --concurrency=16` run produces at most one span per helm invocation plus one per
  release — hundreds, not tens of thousands. Export happens off the critical path; the
  only synchronous cost is the ≤5s shutdown flush, paid only when tracing is on.
- SDK spans and exporters are goroutine-safe; helmfile's parallel release workers need no
  extra locking.

**Functional-impact checklist (the review criteria for every PR in §11):**
1. No `helmexec.Interface` signature changes in any phase; `getHelm()` unchanged.
2. `app.New` changes one line (`Background()` → `telemetry.CommandContext()`), no call-site
   churn; cancellation semantics identical.
3. Kubedog and hook bridging uses `context.WithoutCancel`, which drops cancellation and
   keeps values — the swapped-out parents (`Background`/`TODO`) never propagated
   cancellation either, so SIGINT/timeout behavior is unchanged.
4. Telemetry setup or export failures never fail or slow the run (warning log only, export
   off the critical path).
5. All pre-existing behavior, including the known cancellation gaps of §4.3 and the exact
   content of exit-error messages (legacy redaction profile, §6.2), is preserved
   bit-for-bit; gap fixes and redaction unification are out of scope and filed separately.

## 8. Dependency impact

The OTel libraries are **already in the module graph as indirect dependencies**, required
transitively by existing direct dependencies (`helm.sh/helm/v4` v4.2.4 requires
`go.opentelemetry.io/otel` v1.44.0; helm v3 and `helmfile/vals` also carry otel modules).
Every module this proposal would import — `otel`, `otel/trace`, `otel/sdk`,
`otel/exporters/otlp/otlptrace/otlptracegrpc`, `.../otlptracehttp`,
`otel/exporters/stdout/stdouttrace`, and `contrib/exporters/autoexport` v0.67.0 — is
already pinned in `go.sum` (verified), so promoting them to direct requires brings
**zero new modules to download**. No conflict with existing OTel usage in the process:
none of helmfile's direct dependencies registers OTel globals in library code — verified
no `SetTracerProvider` call sites under helm v3/v4 `pkg/`, vals, or chartify (helm's own
OTel wiring, where present, lives in its CLI layer, not the libraries helmfile imports).
Choosing `autoexport` (§3.2) also means
helmfile maintains no exporter-construction code; its `RegisterSpanExporter` hook covers
any future backend (e.g. zipkin) without helmfile changes. Note the resulting defaults:
`http/protobuf` on `localhost:4318` unless the user overrides the protocol/endpoint.

## 9. Code layout of the change

```
cmd/root.go                      // --otel-tracing flag, Setup call, root span, shutdown handoff
main.go                          // shutdown on both exit and signal paths
pkg/config/global.go             // OtelTracing option (+ accessor on GlobalImpl)
pkg/envvar/const.go              // OtelTracing = "HELMFILE_OTEL_TRACING"
pkg/telemetry/…                  // new package (§4.1)
pkg/app/app.go                   // app.New: Background() → telemetry.CommandContext(); helmfile.load span; loader ctx wiring
pkg/app/desired_state_file_loader.go // ctx field on the unexported desiredStateLoader struct (render-span parent)
pkg/app/two_pass_renderer.go     // helmfile.render/helmfile.parse spans
pkg/helmexec/runner.go           // helm.exec / os.exec spans (single choke point)
pkg/helmexec/redact.go           // shared args redaction, legacy+strict profiles (extracted from exit_error.go)
pkg/helmexec/exit_error.go       // calls shared helper with legacy profile (output byte-identical)
pkg/helmexec/context.go          // HelmContext.Ctx field (phase 2)
pkg/helmexec/exec.go             // execCtx funnel beside exec/execStdIn (phase 2)
pkg/state/state.go               // WithoutCancel bridges (kubedog call site + both event.Bus constructions); release spans (phase 2)
pkg/state/helmx.go               // (no change — bridge happens at its caller)
pkg/event/bus.go                 // optional Ctx field consumed by the default runner
docs/experimental-features.md    // feature entry → promoted out when stable
docs/otel.md                     // user guide (config, backends, CI recipes, sample trace)
```

## 10. Testing strategy

1. **Unit (`pkg/telemetry`)**: table-driven tests for enabled/disabled no-op guarantees,
   `CommandContext()` identity, default resource attributes (`service.name=helmfile`,
   `service.version` from `pkg/app/version`), and propagator extraction of `TRACEPARENT`.
   Global state reset via `export_test.go` so tests stay isolated.
2. **Redaction (`pkg/helmexec`)**: table-driven tests for both profiles — `legacy` pinned
   byte-identical by the existing `exit_error_test.go` goldens; `strict` covering `--set v`
   and `--set=k=v`, `--set-string`/`--set-file`/`--set-json`, `--username`/`--password`,
   benign flags untouched. Pins §6.6.
3. **Span-hierarchy golden tests**: drive `App` with the existing fake helm
   (`pkg/exectest/helm.go`, pre-seeded into `App.helms` the way current app tests do) and
   an in-memory exporter; assert the span tree (names, parent
   links, order) for `template`/`sync` over a small fixture — catches context-plumbing
   regressions. Includes an **orphan-span regression test** for the kubedog and hook paths
   (§4.3): every exported span must have the root span as an ancestor.
4. **OTLP end-to-end**: an `httptest` server speaking OTLP/HTTP+protobuf, pointed at by
   `OTEL_EXPORTER_OTLP_ENDPOINT`; decode exported payloads (`go.opentelemetry.io/proto/otlp`,
   already in the graph) and assert count/attributes. Runs in unit-test context — no
   external collector needed in CI.
5. **Lifecycle tests**: shutdown flushes on command failure and on SIGINT, so spans never
   vanish on failing deploys — the case users care about most. (Requires extracting
   `main.go`'s signal/select/exit logic into a small pure function; the extraction itself
   is behavior-preserving and covered by the same tests.) Also covers the nil-shutdown
   race noted in §4.2.
6. **Regression suite**: existing `make test` must pass unchanged with tracing compiled in
   but disabled. Note the existing `pkg/app` tests construct `&App{...}` literals directly
   and never call `app.New`, so they do *not* guard the §4.2 one-liner — PR 1 adds a
   targeted test asserting `app.New` roots cancellation (and the span) exactly as before.

## 11. Documentation & rollout

1. `docs/otel.md` — user guide: enabling, env vars, collector recipes (Jaeger all-in-one,
   Grafana Tempo, vendor SaaS), CI correlation via `TRACEPARENT`, sample trace reading.
   Linked from `docs/index.md`.
2. Feature listed under **experimental** in `docs/experimental-features.md` for one or two
   minor releases (feedback on span taxonomy is the main thing that may change), then
   promoted to stable with a CHANGELOG entry.
3. PR sequence (each independently shippable and revertible, each measured against the
   §7 functional-impact checklist):
   - **PR 1**: `pkg/telemetry` + flag/env + root span + lifecycle + `app.New` one-liner +
     docs + tests (§4.1–4.2).
   - **PR 2**: `ShellRunner` instrumentation + shared redaction extraction/extension (§4.4
     phase-1 steps 2–3, §6) — the core value.
   - **PR 3**: load/render spans + hook bridging + per-release spans via
     `HelmContext.Ctx`/`execCtx` (§4.4 phase 2).
   - **PR 4 (post-stabilization)**: metrics (e.g. `helmfile.helm.exec.duration` histogram,
     `helmfile.release.count` by result), kubedog/vals spans.

## 12. Future work (explicitly out of scope for v1)

- Metrics pipeline on the same provider (`otel/sdk/metric` with the same env-var config).
- `TRACEPARENT` injection into helm subprocess env so chart-test hooks / plugins can extend
  the helmfile trace.
- Subprocesses started inside `github.com/helmfile/chartify` (kustomize, and any helm calls
  chartify makes) are invisible to `ShellRunner` instrumentation; options are a wrapper
  span around each chartify call in `pkg/state`, or upstream OTel support in chartify.
- Log-to-trace correlation (zap OTel appender).
- Fixing the kubedog/hook cancellation gaps (§4.3) — separate issues filed from this design.

## 13. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Structured logs + collector-side parsing | No hierarchy/timing guarantees; every backend needs custom parsing; poor UX. |
| Prometheus metrics only | Shows counts/durations but not critical paths or nesting; the request is explicitly about tracing where time goes. |
| Hand-rolled exporter selection in helmfile | `autoexport` already exists in the dependency graph and implements the spec env vars (verified: v0.67.0 supports `otlp`/`console`/`none`, protocol dispatch, `none` detection); hand-rolled code is pure maintenance burden and would drift from the spec. |
| Helmfile-specific env vars for endpoint/headers etc. | Duplicates the OTel spec; standard vars are already what platform teams configure. |
| Full `context.Context` refactor of `pkg/state` first | Large, risky churn unrelated to the feature; the `HelmContext.Ctx`/`execCtx` design achieves nesting without it. |
| Using the existing `WithContext` clone for per-release spans | The cached, shared execer (`a.helms`) serves concurrent workers; per-release contexts must travel with the per-call `HelmContext` parameter, not via instance substitution. |
| Config-file (`helmfile.yaml`) telemetry settings | Telemetry is an operational/platform concern, not state authoring; flag+env matches how it's injected in CI. |

## 14. Open questions

1. **Propagation into helm subprocesses**: should helmfile inject `TRACEPARENT` into the
   child process env (opt-in) so hooks/plugins can continue the trace? (§12)
2. **Span-name taxonomy stability**: do we commit to the §5 names as stable API for
   dashboard authors during the experimental window, or reserve the right to rename?
   Proposal: rename freely while experimental, freeze on promotion.
3. **`helmfile.d` parallel mode**: state-file spans are siblings under the root span — is a
   synthetic `helmfile.parallel` grouping span wanted, or does flat suffice?
4. **Trace output when `--log-level=debug`**: duplicate a compact span tree to stderr at
   shutdown for quick local triage without a collector?
5. **Root span noise for trivial commands**: `helmfile version`/`help` also run
   `PersistentPreRunE` — export a root span for them, or skip? (Proposal: skip via a
   short denylist; cosmetic.)
