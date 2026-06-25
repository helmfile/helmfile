# CLI Reference

## CLI Reference

```
Declaratively deploy your Kubernetes manifests, Kustomize configs, and Charts as Helm releases in one shot
V1 mode = false
YAML library = go.yaml.in/yaml/v3

Usage:
  helmfile [command]

Available Commands:
  apply        Apply all resources from state file only when there are changes
  build        Build all resources from state file
  create       Create a helmfile deployment project scaffold
  cache        Cache management
  charts       DEPRECATED: sync releases from state file (helm upgrade --install)
  completion   Generate the autocompletion script for the specified shell
  delete       DEPRECATED: delete releases from state file (helm delete)
  deps         Update charts based on their requirements
  destroy      Destroys and then purges releases
  diff         Diff releases defined in state file
  fetch        Fetch charts from state file
  help         Help about any command
  init         Initialize the helmfile, includes version checking and installation of helm and plug-ins
  lint         Lint charts from state file (helm lint)
  list         List releases defined in state file
  repos        Add chart repositories defined in state file
  show-dag     It prints a table with 3 columns, GROUP, RELEASE, and DEPENDENCIES. GROUP is the unsigned, monotonically increasing integer starting from 1. All the releases with the same GROUP are deployed concurrently. Everything in GROUP 2 starts being deployed only after everything in GROUP 1 got successfully deployed. RELEASE is the release that belongs to the GROUP. DEPENDENCIES is the list of releases that the RELEASE depends on. It should always be empty for releases in GROUP 1. DEPENDENCIES for a release in GROUP 2 should have some or all dependencies appeared in GROUP 1. It can be "some" because Helmfile simplifies the DAGs of releases into a DAG of groups, so that Helmfile always produce a single DAG for everything written in helmfile.yaml, even when there are technically two or more independent DAGs of releases in it.
  status       Retrieve status of releases in state file
  sync         Sync releases defined in state file
  template     Template releases defined in state file
  test         Test charts from state file (helm test)
  unittest     Unit test charts from state file using helm-unittest plugin
  version      Print the CLI version
  write-values Write values files for releases. Similar to `helmfile template`, write values files instead of manifests.

Flags:
      --allow-no-matching-release             Do not exit with an error code if the provided selector has no matching releases.
  -c, --chart string                          Set chart. Uses the chart set in release by default, and is available in template as {{ .Chart }}
      --color                                 Output with color
      --debug                                 Enable verbose output for Helm and set log-level to debug, this disables --quiet/-q effect. Overrides "HELMFILE_DEBUG" OS environment variable when specified
      --disable-force-update                  do not force helm repos to update when executing "helm repo add"
      --enable-live-output                    Show live output from the Helm binary Stdout/Stderr into Helmfile own Stdout/Stderr.
                                              It only applies for the Helm CLI commands, Stdout/Stderr for Hooks are still displayed only when it's execution finishes.
  -e, --environment string                    specify the environment name. Overrides "HELMFILE_ENVIRONMENT" OS environment variable when specified. defaults to "default"
  -f, --file helmfile.yaml                    load config from file or directory. defaults to "helmfile.yaml" or "helmfile.yaml.gotmpl" or "helmfile.d" (means "helmfile.d/*.yaml" or "helmfile.d/*.yaml.gotmpl") in this preference. Specify - to load the config from the standard input.
  -b, --helm-binary string                    Path to the helm binary. Overrides "HELMFILE_HELM_BINARY" OS environment variable when specified (default "helm")
  -h, --help                                  help for helmfile
  -i, --interactive                           Request confirmation before attempting to modify clusters
      --kube-context string                   Set kubectl context. Overrides "HELMFILE_KUBE_CONTEXT" OS environment variable when specified. Uses current kubectl context by default
  -k, --kustomize-binary string               Path to the kustomize binary. Overrides "HELMFILE_KUSTOMIZE_BINARY" OS environment variable when specified (default "kustomize")
      --log-level string                      Set log level. Overrides "HELMFILE_LOG_LEVEL" OS environment variable when specified (default "info")
  -n, --namespace string                      Set namespace. Overrides "HELMFILE_NAMESPACE" OS environment variable when specified. Uses the namespace set in the context by default, and is available in templates as {{ .Namespace }}
      --no-color                              Output without color. Overrides "HELMFILE_NO_COLOR" and "NO_COLOR" OS environment variables when specified
  -q, --quiet                                 Silence output. Equivalent to log-level warn. Overrides "HELMFILE_QUIET" OS environment variable when specified
  -l, --selector stringArray                  Only run using the releases that match labels. Labels can take the form of foo=bar or foo!=bar.
                                              A release must match all labels in a group in order to be used. Multiple groups can be specified at once.
                                              "--selector tier=frontend,tier!=proxy --selector tier=backend" will match all frontend, non-proxy releases AND all backend releases.
                                              The name of a release can be used as a label: "--selector name=myrelease"
      --skip-deps                             skip running "helm repo update" and "helm dependency build"
      --state-values-file stringArray         specify state values in a YAML file. Used to override .Values within the helmfile template (not values template).
      --state-values-set stringArray          set state values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).
      --state-values-set-string stringArray   set state STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).
      --sequential-helmfiles                   Process helmfile.d files sequentially in alphabetical order instead of in parallel
      --strip-args-values-on-exit-error       Strip the potential secret values of the helm command args contained in a helmfile error message (default true)
  -v, --version                               version for helmfile

Use "helmfile [command] --help" for more information about a command.
```

**Note:** Each command has its own specific flags. Use `helmfile [command] --help` to see command-specific options. For example, `helmfile sync --help` shows operational flags like `--timeout`, `--wait`, and `--wait-for-jobs`.

### init

The `helmfile init` sub-command checks the dependencies required for helmfile operation, such as `helm`, `helm diff plugin`, `helm secrets plugin`, `helm helm-git plugin`, `helm s3 plugin`. When it does not exist or the version is too low, it can be installed automatically.

### cache

The `helmfile cache` sub-command is designed for cache management. Go-getter-backed remote file system are cached by `helmfile`. There is no TTL implemented, if you need to update the cached files or directories, you need to clean individually or run a full cleanup with `helmfile cache cleanup`

#### OCI Chart Cache

OCI charts are cached in the shared cache directory (`~/.cache/helmfile` by default, or `$HELMFILE_CACHE_HOME`). This cache is shared across all helmfile processes.

**Cache Behavior:**

- When a chart exists in the shared cache and is valid, it is reused without re-downloading
- The `--skip-refresh` flag can be used to skip checking for updates to cached charts stored in process-specific temporary directories (it does not affect charts already present in the shared cache)
- When running multiple helmfile processes in parallel (e.g., as an ArgoCD plugin), charts in the shared cache are not refreshed/deleted to prevent race conditions

**Forcing a Cache Refresh:**

To force a refresh of cached OCI charts, run:
```bash
helmfile cache cleanup
```

This will clear the shared cache, allowing the next helmfile command to re-download charts.

#### cache info

Display information about the cache directory.

#### cache cleanup

Remove all cached files from the cache directory.

### sync

The `helmfile sync` sub-command sync your cluster state as described in your `helmfile`. The default helmfile is `helmfile.yaml`, but any YAML file can be passed by specifying a `--file path/to/your/yaml/file` flag.

Under the covers, Helmfile executes `helm upgrade --install` for each `release` declared in the manifest, by optionally decrypting [secrets](#secrets) to be consumed as helm chart values. It also updates specified chart repositories and updates the
dependencies of any referenced local charts.

#### Common sync flags

* `--timeout SECONDS` - Override the default timeout for all releases in this sync operation. This takes precedence over `helmDefaults.timeout` and per-release `timeout` settings.
* `--wait` - Override the default wait behavior for all releases
* `--wait-for-jobs` - Override the default wait-for-jobs behavior for all releases

Examples:

```bash
# Override timeout for all releases to 10 minutes
helmfile sync --timeout 600

# Combine timeout with wait flags
helmfile sync --timeout 900 --wait --wait-for-jobs

# Target specific releases with custom timeout
helmfile sync --selector tier=backend --timeout 1200
```

For Helm 2.9+ you can use a username and password to authenticate to a remote repository.

### deps

The `helmfile deps` sub-command locks your helmfile state and local charts dependencies.

It basically runs `helm dependency update` on your helmfile state file and all the referenced local charts, so that you get a "lock" file per each helmfile state or local chart.

All the other `helmfile` sub-commands like `sync` use chart versions recorded in the lock files, so that e.g. untested chart versions won't suddenly get deployed to the production environment.

For example, the lock file for a helmfile state file named `helmfile.1.yaml` will be `helmfile.1.lock`. The lock file for a local chart would be `requirements.lock`, which is the same as `helm`.

The lock file can be changed using `lockFilePath` in helm state, which makes it possible to for example have a different lock file per environment via templating.

It is recommended to version-control all the lock files, so that they can be used in the production deployment pipeline for extra reproducibility.

To bring in chart updates systematically, it would also be a good idea to run `helmfile deps` regularly, test it, and then update the lock files in the version-control system.

### diff

The `helmfile diff` sub-command executes the [helm-diff](https://github.com/databus23/helm-diff) plugin across all of
the charts/releases defined in the manifest.

To supply the diff functionality Helmfile needs the [helm-diff](https://github.com/databus23/helm-diff) plugin v2.9.0+1 or greater installed. For Helm 2.3+
you should be able to simply execute `helm plugin install https://github.com/databus23/helm-diff`. For more details
please look at their [documentation](https://github.com/databus23/helm-diff#helm-diff-plugin).

### doctor

`helmfile doctor` runs `helmfile diff` and asks an OpenAI-compatible LLM to summarize the changes and flag risks
(such as data loss, security exposure, breaking changes, downtime, performance, and best-practice issues).

**When no LLM is configured, `doctor` falls back to running `helmfile diff`**
with `--show-secrets` forced off. Most diff flags are accepted for
compatibility, but note two differences: `--output` is reserved for the doctor
report format (use `--diff-output` for helm-diff's plugin output format), and
`--show-secrets` is silently ignored (secrets are always redacted). This makes
it safe to swap into existing CI jobs: the worst case is you get the same diff
output you already had.

#### Configuration

The LLM endpoint speaks the OpenAI Chat Completions protocol (`/v1/chat/completions`). This means it works
with any compatible gateway:

- Direct providers: OpenAI, DeepSeek, Mistral, Together, Groq.
- Unified gateways: One-API, LiteLLM, Azure OpenAI proxy, Cloudflare AI Gateway.
- Local servers: Ollama (with OpenAI compatibility), vLLM, LocalAI.

Configuration precedence (low to high):

1. **Environment variables**: `HELMFILE_LLM_BASE_URL`, `HELMFILE_LLM_API_KEY`, `HELMFILE_LLM_MODEL`,
   `HELMFILE_LLM_TIMEOUT` (Go duration, e.g. `90s`), `HELMFILE_LLM_MAX_TOKENS`.
2. **`helmfile.yaml` top-level `llm:` block**:
   ```yaml
   llm:
     baseURL: https://one-api.internal/v1
     model: gpt-4o
     apiKey: {{ env "HELMFILE_LLM_API_KEY" }}
     timeout: 60s
     maxTokens: 4096
   ```
3. **CLI flags** (highest precedence): `--llm-base-url`, `--llm-api-key`, `--llm-model`,
   `--llm-timeout`, `--llm-max-tokens`.

A layer's non-zero fields override lower layers; empty fields fall through.

> **Note on zero values**: because merge treats "field == zero value" as "not set",
> `--llm-max-tokens 0` does NOT reset MaxTokens to 0 — it is treated as "flag not set"
> and the env/yaml value wins. This matches helmfile's existing flag-override convention
> (same as `--concurrency`, `--context`). Use the yaml block to explicitly request a zero
> value if you really need it.

`baseURL` is **optional**: if you use OpenAI's official endpoint, omit `baseURL` and
the client defaults to `https://api.openai.com/v1`. Set `baseURL` only when targeting
a gateway or non-OpenAI provider. Only `apiKey` and `model` are required to enable
LLM analysis.

#### Output

- **Default (markdown / text)**: a human-readable report with summary, risks sorted by severity
  (🔴 high → 🟡 medium → 🟢 low → ⚪ unknown), affected resources, and a footer showing
  model, duration, and secrets-redacted count.
- **`--output json`**: structured JSON including the model's analysis, the diff
  (always post-redaction), and metadata. Suitable for CI pipelines that want to
  post-process (e.g. comment on a pull request). The JSON shape is:
  ```json
  {
    "summary": "...",
    "risks": [...],
    "diff": "...",
    "secrets_redacted": 3,
    "model": "gpt-4o",
    "duration": "8.2s",
    "timestamp": "2026-..."
  }
  ```
  Key field semantics:
  - `risks`: always an array when an analysis ran (even empty: `[]`, never `null`).
    When doctor is unconfigured (no LLM), the field is omitted entirely. This lets
    CI distinguish "LLM said no risks" from "analysis never happened".
  - `diff`: always post-redaction. Doctor never exposes the raw pre-redaction diff
    through stdout/JSON. If you need to debug helm-diff itself, run `helmfile diff` directly.
  - `secrets_redacted`: always present, even when 0. Lets you confirm the redactor ran.

#### Exit codes

| Code | Meaning |
|---|---|
| 0 | success, or only low/medium risks, or LLM call failed (degraded to plain diff) |
| 2 | at least one high-severity risk and `--force` not passed (CI gate) |
| 1 | other error (state load failure, helm-diff runtime failure, etc.) |

The "detected changes" exit-2 signal from `helm diff --detailed-exitcode` is intentionally swallowed —
doctor's whole job is to react to changes, so reporting them via exit code would be noise.

#### Backend compatibility

Doctor uses `response_format: {type: "json_object"}` (JSON mode) by default for
reliable structured output. If the backend doesn't support JSON mode (common on
early One-API versions, some LiteLLM configs, or Ollama's OpenAI shim), doctor
automatically detects the 400 error, retries without `response_format`, and falls
back to extracting JSON from the model's response via a robust parser that handles
markdown fences and surrounding prose. No user action needed.

#### Safety

- `doctor` never invokes `apply` / `sync` / `destroy`. It is a read-only command.
- LLM calls inherit the global helmfile context timeout, plus a per-request timeout (default 60s).
- If the LLM call fails for any reason, doctor prints the redacted diff with a warning banner and exits 0,
  so AI outages never block deployments.
- Large diffs are defensively capped at 32 KB before being sent to the LLM.
  Doctor defaults `--context` to 3 (vs diff's 0) so the LLM sees enough surrounding
  YAML to ground its analysis. Adjust with `--context N`.

#### Secret handling (always redacted)

Secrets are **always** redacted before any byte leaves the process. This is a
hard safety contract, not a default you can disable.

Two layers enforce it:

1. **`--show-secrets` is silently ignored.** doctor wraps the diff config so
   `ShowSecrets()` always reports `false` to helm-diff, which causes
   helm-diff itself to substitute `<REDACTED>` for secret values. If you
   already pass `--show-secrets` in a wrapper script, doctor still redacts.
2. **Defense-in-depth text redactor.** A second pass strips any residual
   secret-looking content from the captured diff before it is sent to the
   LLM. It catches:
   - `kind: Secret` resource blocks (full YAML mode),
   - sensitive key/value lines (`password`, `token`, `apiKey`, `client_secret`,
     `bearer`, …) regardless of casing or separator (`api_key` = `apiKey` =
     `API_KEY`),
   - free-form base64 runs of 40+ chars (encoded keys/certs),
   - JWT-shaped tokens (`eyJ...`).

The redaction count is always surfaced — even when 0 — in the report footer
and JSON output, so you can confirm the redactor ran:

```markdown
---
Model: gpt-4o | Duration: 8.2s | Secrets redacted: 3
```

In `--output json` mode the field is `secrets_redacted` (always present).

If you want to drop the *structure* of Secret resources entirely (not just
their values), pass `--suppress-secrets`:

```bash
helmfile doctor --suppress-secrets
```

#### Prompt injection defense

Release names and environment values from `helmfile.yaml` are embedded in the
LLM prompt as context. Since helmfile.yaml can come from untrusted sources
(e.g. a GitOps pull request), doctor JSON-encodes these fields via
`encoding/json` before insertion. This ensures that a malicious release name
like `"foo\n\nIgnore previous instructions"` is treated as opaque string data
by the model, not as a directive.

#### Known limitations

- **Double state load**: doctor loads the helmfile state twice — once to
  peek at the `llm:` block and release names, and again inside `helmfile diff`.
  For large helmfiles with remote bases or heavy templating this can add
  noticeable latency. Caching across loads would require touching core code,
  which doctor intentionally avoids. In the unconfigured path (no LLM),
  the cost equals plain `helmfile diff`.
- **`--log-output stdout`**: if you point helmfile's logger at stdout (via
  `--log-output stdout`), log lines will be captured alongside the diff and
  sent to the LLM. Doctor warns when it detects this configuration. Prefer
  the default (stderr) when running doctor.

#### Examples

Quick local analysis (OpenAI official — no baseURL needed):

```bash
export HELMFILE_LLM_API_KEY=sk-...
export HELMFILE_LLM_MODEL=gpt-4o
helmfile doctor
```

CI gate that fails the build on high-severity risk:

```bash
helmfile --environment prod doctor --output json > doctor-report.json
# exit code 2 stops CI; --force bypasses when a human has approved the change
```

Pin the gateway per-environment via `helmfile.yaml`:

```yaml
environments:
  prod:
    values:
      - llmGateway: https://prod-llm-gateway.internal/v1
llm:
  baseURL: {{ .Values.llmGateway | quote }}
  model: gpt-4o
  apiKey: {{ env "HELMFILE_LLM_API_KEY" }}
```

### apply

The `helmfile apply` sub-command begins by executing `diff`. If `diff` finds that there is any changes, `sync` is executed. Adding `--interactive` instructs Helmfile to request your confirmation before `sync`.

An expected use-case of `apply` is to schedule it to run periodically, so that you can auto-fix skews between the desired and the current state of your apps running on Kubernetes clusters.

### destroy

The `helmfile destroy` sub-command uninstalls and purges all the releases defined in the manifests.

`helmfile --interactive destroy` instructs Helmfile to request your confirmation before actually deleting releases.

`destroy` basically runs `helm uninstall --purge` on all the targeted releases. If you don't want purging, use `helmfile delete` instead.
If `--skip-charts` flag is not set, destroy would prepare all releases, by fetching charts and templating them.

### delete (DEPRECATED)

The `helmfile delete` sub-command deletes all the releases defined in the manifests.

`helmfile --interactive delete` instructs Helmfile to request your confirmation before actually deleting releases.

Note that `delete` doesn't purge releases. So `helmfile delete && helmfile sync` results in sync failed due to that releases names are not deleted but preserved for future references. If you really want to remove releases for reuse, add `--purge` flag to run it like `helmfile delete --purge`.
If `--skip-charts` flag is not set, destroy would prepare all releases, by fetching charts and templating them.

### secrets

The `secrets` parameter in a `helmfile.yaml` causes the [helm-secrets](https://github.com/jkroepke/helm-secrets) plugin to be executed to decrypt the file.

To supply the secret functionality Helmfile needs the `helm secrets` plugin installed. For Helm 2.3+
you should be able to simply execute `helm plugin install https://github.com/jkroepke/helm-secrets
`.

### test

The `helmfile test` sub-command runs a `helm test` against specified releases in the manifest, default to all

Use `--cleanup` to delete pods upon completion.

### lint

The `helmfile lint` sub-command runs a `helm lint` across all of the charts/releases defined in the manifest. Non local charts will be fetched into a temporary folder which will be deleted once the task is completed.

### unittest

The `helmfile unittest` sub-command runs `helm unittest` (from the [helm-unittest plugin](https://github.com/helm-unittest/helm-unittest)) on releases that have `unitTests` defined. It automatically generates the final merged values files for each release and passes them to `helm unittest`.

This requires the `helm-unittest` plugin to be installed. You can install it with:

```bash
helm plugin install https://github.com/helm-unittest/helm-unittest
```

Releases without `unitTests` defined are skipped. Non-local charts will be fetched into a temporary folder which will be deleted once the task is completed.

Example helmfile configuration:

```yaml
releases:
  - name: my-app
    chart: ./charts/my-app
    values:
      - values.yaml
    unitTests:
      - tests
```

The `unitTests` paths are relative to the chart directory and follow helm-unittest conventions.
If a path does not contain glob characters, it is treated as a directory and `/*_test.yaml` is appended automatically.
You can also specify explicit glob patterns (e.g., `tests/**/*_test.yaml`).

Running `helmfile unittest` will:

1. Merge all values files defined for the release
2. Run `helm unittest ./charts/my-app --values <merged-values> --file tests/*_test.yaml`

You can pass additional flags:

```bash
# Run with additional values
helmfile unittest --values extra-values.yaml

# Run with --set overrides
helmfile unittest --set key=value

# Target specific releases
helmfile unittest --selector name=my-app

# Fail fast on first test failure
helmfile unittest --fail-fast

# Enable colored output (Helm 3 only; ignored on Helm 4 due to flag parsing issues)
helmfile unittest --color

# Enable verbose plugin output
helmfile unittest --debug-plugin

# Pass extra arguments to helm unittest
helmfile unittest --args "--strict"
```

### create

The `helmfile create` sub-command generates a helmfile deployment project scaffold with best-practice directory structure.

```bash
# Create a project in a new directory
helmfile create my-project

# Create a project in the current directory
helmfile create

# Specify a custom output directory
helmfile create my-project --output-dir /path/to/project

# Overwrite existing scaffold files
helmfile create my-project --force
```

This generates:

* `helmfile.yaml` — Main configuration with commented examples for repositories, environments, and releases
* `environments/default.yaml` — Default environment values file
* `values/.gitkeep` — Placeholder for release-specific value files

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-o`, `--output-dir` | `""` | Output directory (defaults to NAME or current directory) |
| `--force` | false | Overwrite existing scaffold files |

The command validates the project name (no path separators, `.`, `..`, or whitespace-only names). Without `--force`, it atomically checks all target paths before writing to avoid partial scaffolds.

### fetch

The `helmfile fetch` sub-command downloads or copies local charts to a local directory for debug purpose. The local directory
must be specified with `--output-dir`.

### list

The `helmfile list` sub-command lists releases defined in the manifest. Optional `--output` flag accepts `json` to output releases in JSON format.

If `--skip-charts` flag is not set, list would prepare all releases, by fetching charts and templating them.

### version

The `helmfile version` sub-command prints the version of Helmfile.Optional `-o` flag accepts `json` `yaml` `short` to output version in JSON, YAML or short format.

default it will check for the latest version of Helmfile and print a tip if the current version is not the latest. To disable this behavior, set environment variable `HELMFILE_UPGRADE_NOTICE_DISABLED` to any non-empty value.

### show-dag

It prints a table with 3 columns, GROUP, RELEASE, and DEPENDENCIES.

GROUP is the unsigned, monotonically increasing integer starting from 1. All the releases with the same GROUP are deployed concurrently. Everything in GROUP 2 starts being deployed only after everything in GROUP 1 got successfully deployed.

RELEASE is the release that belongs to the GROUP.

DEPENDENCIES is the list of releases that the RELEASE depends on. It should always be empty for releases in GROUP 1. DEPENDENCIES for a release in GROUP 2 should have some or all dependencies appeared in GROUP 1. It can be "some" because Helmfile simplifies the DAGs of releases into a DAG of groups, so that Helmfile always produce a single DAG for everything written in helmfile.yaml, even when there are technically two or more independent DAGs of releases in it.

### print-env

The `helmfile print-env` sub-command prints the parsed environment configuration including merged values (with decrypted secrets). This is useful for debugging environment configuration.

```bash
# Print environment in YAML format (default)
helmfile print-env

# Print environment in JSON format
helmfile print-env --output json

# Print a specific environment
helmfile print-env -e production
```

### status

The `helmfile status` sub-command retrieves the status of releases in the state file by running `helm status` for each release.

### Additional CLI Flags

The following global flags are also available but not shown in the main help output:

| Flag | Default | Description |
|------|---------|-------------|
| `--kubeconfig` | `""` | Use a particular kubeconfig file |
| `--skip-refresh` | false | Skip running `helm repo update` (lighter than `--skip-deps` which also skips dependency build) |
| `--enforce-plugin-verification` | false | Fail plugin installation if verification is not supported |
| `--oci-plain-http` | false | Use plain HTTP for OCI registries (required for local/insecure registries in Helm 4) |

#### fetch flags

| Flag | Default | Description |
|------|---------|-------------|
| `--output-dir` | temp dir | Directory to store charts. If not set, a temporary directory is used and deleted when the command terminates |
| `--output-dir-template` | (default template) | Go text template for generating the output directory. Available fields: `{{ .OutputDir }}`, `{{ .ChartName }}`, `{{ .Release.* }}`, `{{ .Environment.Name }}`, `{{ .Environment.KubeContext }}`, `{{ .Environment.Values.* }}` |
| `--write-output` | false | Write a helmfile.yaml to stdout with chart references updated to point to the downloaded local chart paths. Requires `--output-dir` |
| `--concurrency` | 0 | Maximum number of concurrent helm processes to run, 0 is unlimited |

This is useful for air-gapped environments: download charts with `--output-dir` and `--write-output`, then transfer the output directory and the generated helmfile.yaml to the air-gapped environment.

#### template-args (template / apply / sync / diff)

| Flag | Default | Available on | Description |
|------|---------|--------------|-------------|
| `--template-args` | `""` | `template`, `apply`, `sync`, `diff` | Extra args appended to the helm rendering invocation. Reaches the final `helm template` for `template`; reaches both the `helm diff` rendering and chartify's internal `helm template` pre-render for `apply`/`sync`/`diff`. |

The most common use case is enabling Helm's [`lookup`](https://helm.sh/docs/chart_template_guide/functions_and_pipelines/#using-the-lookup-function) function, which queries the live cluster during rendering. `lookup` requires a server-side connection, so pass `--dry-run=server`:

```bash
# Render manifests with cluster access so lookup() resolves live values
helmfile template --template-args="--dry-run=server"

# Enable lookup() during the helm-diff phase of apply (and during diff/sync)
helmfile apply  --template-args="--dry-run=server"
helmfile diff   --template-args="--dry-run=server"
```

Notes:

- For `template`, the args reach the final `helm template` output.
- For `apply` and `diff`, the args reach the `helm diff upgrade` rendering so `lookup()` resolves live values during the diff phase (helm-diff supports `--dry-run=server`, which enables the lookup template function).
- For `sync`, the real `helm upgrade` already connects to the cluster, so `lookup()` works without this flag; `--template-args` is only needed there to pass additional flags to chartify's pre-render step.
- Charts that use `lookup()` should always guard against the empty result (e.g. with `default dict`), because helm renders client-side whenever it has no server connection.

#### destroy flags

| Flag | Default | Description |
|------|---------|-------------|
| `--skip-charts` | false | Don't prepare charts when destroying releases |
| `--deleteWait` | false | Override helmDefaults.wait, sets `helm uninstall --wait` |
| `--deleteTimeout` | 300 | Time in seconds to wait for helm uninstall |
| `--cascade` | background | Pass cascade to helm exec |
| `--concurrency` | 0 | Maximum number of concurrent helm processes to run, 0 is unlimited |

#### list flags

| Flag | Default | Description |
|------|---------|-------------|
| `--skip-charts` | false | Don't prepare charts when listing releases |
| `--keep-temp-dir` | false | Keep temporary directory after listing |
| `--output` | `""` | Output format: `json` for JSON output |
