# Configuration Reference

This page is a comprehensive reference for all options available in `helmfile.yaml`.

**If you're new to Helmfile**, start with the [Getting Started](index.md#getting-started) tutorial on the home page, then read [Writing Helmfile](writing-helmfile.md) for patterns. Come back here when you need to look up a specific field.

**CAUTION**: This documentation is for the development version of Helmfile. If you are looking for the documentation for any of releases, please switch to the corresponding release tag like [v0.143.4](https://github.com/helmfile/helmfile/tree/v0.143.4).

## Quick Reference

A `helmfile.yaml` has these top-level sections:

| Section | Purpose |
|---------|---------|
| `repositories` | Helm chart repositories to use |
| `releases` | The Helm releases to deploy (the core of helmfile) |
| `helmDefaults` | Default Helm options for all releases |
| `environments` | Environment-specific values (dev, staging, prod) |
| `helmfiles` | Include other helmfile.yaml files (nesting) |
| `bases` | Shared base files merged before this helmfile |
| `values` | Default values available in templates |
| `commonLabels` | Labels applied to all releases |
| `templates` | Reusable release templates |
| `hooks` | Global lifecycle hooks |
| `apiVersions` / `kubeVersion` | Kubernetes version capabilities |

## Full Reference

The default name for a helmfile is `helmfile.yaml`:

```yaml
# Chart repositories used from within this state file
#
# Use `helm-s3` and `helm-git` and whatever Helm Downloader plugins
# to use repositories other than the official repository or one backend by chartmuseum.
repositories:
# To use official "stable" charts a.k.a https://github.com/helm/charts/tree/master/stable
- name: stable
  url: https://charts.helm.sh/stable
# To use official "incubator" charts a.k.a https://github.com/helm/charts/tree/master/incubator
- name: incubator
  url: https://charts.helm.sh/incubator
# helm-git powered repository: You can treat any Git repository as a charts repository
- name: polaris
  url: git+https://github.com/reactiveops/polaris@deploy/helm?ref=master
# Advanced configuration: You can setup basic or tls auth and optionally enable helm OCI integration
- name: roboll
  url: roboll.io/charts
  certFile: optional_client_cert
  keyFile: optional_client_key
  # username is retrieved from the environment with the format <registryNameUpperCase>_USERNAME for CI usage, here ROBOLL_USERNAME
  username: optional_username
  # password is retrieved from the environment with the format <registryNameUpperCase>_PASSWORD for CI usage, here ROBOLL_PASSWORD
  password: optional_password
  oci: true
  passCredentials: true
  verify: true
  keyring: path/to/keyring.gpg
# Advanced configuration: You can use a ca bundle to use an https repo
# with a self-signed certificate
- name: insecure
  url: https://charts.my-insecure-domain.com
  caFile: optional_ca_crt
# Advanced configuration: You can skip the verification of TLS for an https repo
- name: skipTLS
  url: https://ss.my-insecure-domain.com
  skipTLSVerify: true
# Advanced configuration: Connect to a repo served over plain http
- name: plainHTTP
  url: http://just.http.domain.com
  plainHttp: true

# context: kube-context # this directive is deprecated, please consider using helmDefaults.kubeContext

# Path to alternative helm binary (--helm-binary)
# Supports both Helm 3.x and Helm 4.x
helmBinary: path/to/helm

# Path to alternative kustomize binary (--kustomize-binary)
kustomizeBinary: path/to/kustomize

# Path to alternative lock file. The default is <state file name>.lock, i.e for helmfile.yaml it's helmfile.lock.
lockFilePath: path/to/lock.file

# Default values to set for args along with dedicated keys that can be set by contributors, cli args take precedence over these.
# In other words, unset values results in no flags passed to helm.
# See the helm usage (helm SUBCOMMAND -h) for more info on default values when those flags aren't provided.
helmDefaults:
  kubeContext: kube-context          #dedicated default key for kube-context (--kube-context)
  cleanupOnFail: false               #dedicated default key for helm flag --cleanup-on-fail
  # additional and global args passed to helm (default "")
  args:
    - "--set k=v"
  diffArgs:
    - "--suppress-secrets"
  syncArgs:
    - "--labels=app.kubernetes.io/managed-by=helmfile"
  # verify the chart before upgrading (only works with packaged charts not directories) (default false)
  verify: true
  keyring: path/to/keyring.gpg
  #  --skip-schema-validation flag to helm 'install', 'upgrade' and 'lint' (default false)
  skipSchemaValidation: false
  # wait for k8s resources via --wait. (default false)
  wait: true
  # DEPRECATED: waitRetries is no longer supported as the --wait-retries flag was removed from Helm.
  # This configuration is ignored and preserved only for backward compatibility.
  # waitRetries: 3
  # if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout (default false)
  waitForJobs: true
  # time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks, and waits on pod/pvc/svc/deployment readiness) (default 300)
  timeout: 600
  # performs pods restart for the resource if applicable (default false)
  recreatePods: true
  # forces resource update through delete/recreate if needed (default false)
  force: false
  # limit the maximum number of revisions saved per release. Use 0 for no limit. (default 10)
  historyMax: 10
  # automatically create release namespaces if they do not exist (default true)
  createNamespace: true
  # if used with charts museum allows to pull unstable charts for deployment, for example: if 1.2.3 and 1.2.4-dev versions exist and set to true, 1.2.4-dev will be pulled (default false)
  devel: true
  # When set to `true`, skips running `helm dep up` and `helm dep build` on this release's chart.
  # Useful when the chart is broken, like seen in https://github.com/roboll/helmfile/issues/1547
  skipDeps: false
  # If set to true, reuses the last release's values and merges them with ones provided in helmfile.
  # This attribute, can be overriden in CLI with --reset/reuse-values flag of apply/sync/diff subcommands
  reuseValues: false
  # propagate `--post-renderer` to helmv3 template and helm install
  postRenderer: "path/to/postRenderer"
  # propagate `--post-renderer-args` to helmv3 template and helm install. This allows using Powershell
  # scripts on Windows as a post renderer
  postRendererArgs:
  - PowerShell
  - "-Command"
  - "theScript.ps1"
  #	cascade `--cascade` to helmv3 delete, available values: background, foreground, or orphan, default: background
  cascade: "background"
  # insecureSkipTLSVerify is true if the TLS verification should be skipped when fetching remote chart
  insecureSkipTLSVerify: false
  # plainHttp is true if fetching the remote chart should be done using HTTP
  plainHttp: false
  # --wait flag for destroy/delete, if set to true, will wait until all resources are deleted before mark delete command as successful
  deleteWait: false
  # Timeout is the time in seconds to wait for helmfile destroy/delete (default 300)
  deleteTimeout: 300
  # suppressOutputLineRegex is a list of regex patterns to suppress output lines from helm diff (default []), available in helmfile v0.162.0
  suppressOutputLineRegex:
    - "version"
  # syncReleaseLabels is a list of labels to be added to the release when syncing.
  syncReleaseLabels: false


# these labels will be applied to all releases in a Helmfile. Useful in templating if you have a helmfile per environment or customer and don't want to copy the same label to each release
commonLabels:
  hello: world

# The desired states of Helm releases.
#
# Helmfile runs various helm commands to converge the current state in the live cluster to the desired state defined here.
releases:
  # Published chart example
  - name: vault                            # name of this release
    namespace: vault                       # target namespace
    createNamespace: true                  # automatically create release namespace (default true)
    labels:                                # Arbitrary key value pairs for filtering releases
      foo: bar
    chart: roboll/vault-secret-manager     # the chart being installed to create this release, referenced by `repository/chart` syntax
    version: ~1.24.1                       # the semver of the chart. range constraint is supported
    condition: vault.enabled               # The values lookup key for filtering releases. Corresponds to the boolean value of `vault.enabled`, where `vault` is an arbitrary value
    missingFileHandler: Warn # set to either "Error" or "Warn". "Error" instructs helmfile to fail when unable to find a values or secrets file. When "Warn", it prints the file and continues.
    missingFileHandlerConfig:
      # Ignores missing git branch error so that the Debug/Info/Warn handler can treat a missing branch as non-error.
      # See https://github.com/helmfile/helmfile/issues/392
      ignoreMissingGitBranch: true
    # Values files used for rendering the chart
    values:
      # Value files passed via --values
      - vault.yaml
      # Inline values, passed via a temporary values file and --values, so that it doesn't suffer from type issues like --set
      - address: https://vault.example.com
      # Go template available in inline values and values files.
      - image:
          # The end result is more or less YAML. So do `quote` to prevent number-like strings from accidentally parsed into numbers!
          # See https://github.com/roboll/helmfile/issues/608
          tag: {{ requiredEnv "IMAGE_TAG" | quote }}
          # Otherwise:
          #   tag: "{{ requiredEnv "IMAGE_TAG" }}"
          #   tag: !!string {{ requiredEnv "IMAGE_TAG" }}
        db:
          username: {{ requiredEnv "DB_USERNAME" }}
          # value taken from environment variable. Quotes are necessary. Will throw an error if the environment variable is not set. $DB_PASSWORD needs to be set in the calling environment ex: export DB_PASSWORD='password1'
          password: {{ requiredEnv "DB_PASSWORD" }}
        proxy:
          # Interpolate environment variable with a fixed string
          domain: {{ requiredEnv "PLATFORM_ID" }}.my-domain.com
          scheme: {{ env "SCHEME" | default "https" }}
    # Use `values` whenever possible!
    # `setString` translates to helm's `--set-string key=val`
    setString:
    # set a single array value in an array, translates to --set-string bar[0]={1,2}
    - name: bar[0]
      values:
      - 1
      - 2
    # set a templated value
    - name: namespace
      value: {{ .Namespace }}
    # `set` translates to helm's `--set key=val`, that is known to suffer from type issues like https://github.com/roboll/helmfile/issues/608
    set:
    # single value loaded from a local file, translates to --set-file foo.config=path/to/file
    - name: foo.config
      file: path/to/file
    # set a single array value in an array, translates to --set bar[0]={1,2}
    - name: bar[0]
      values:
      - 1
      - 2
    # set a templated value
    - name: namespace
      value: {{ .Namespace }}
    # will attempt to decrypt it using helm-secrets plugin
    secrets:
      - vault_secret.yaml
    # Override helmDefaults options for verify, wait, waitForJobs, timeout, recreatePods, force and reuseValues.
    verify: true
    keyring: path/to/keyring.gpg
    #  --skip-schema-validation flag to helm 'install', 'upgrade' and 'lint' (default false)
    skipSchemaValidation: false
    wait: true
    # DEPRECATED: waitRetries is no longer supported - see documentation above
    # waitRetries: 3
    waitForJobs: true
    timeout: 60
    recreatePods: true
    force: false
    reuseValues: false
    # set `false` to uninstall this release on sync.  (default true)
    installed: true
    # Defines the strategy to use when updating. Possible value is:
    # - "reinstallIfForbidden": Performs an uninstall before the update only if the update is forbidden (e.g., due to permission issues or conflicts).
    updateStrategy: ""
    # restores previous state in case of failed release (default false)
    atomic: true
    # when true, cleans up any new resources created during a failed release (default false)
    cleanupOnFail: false
    # --kube-context to be passed to helm commands
    # See https://github.com/roboll/helmfile/issues/642
    # (default "", which means the standard kubeconfig, either ~/kubeconfig or the file pointed by $KUBECONFIG environment variable)
    kubeContext: kube-context
    # passes --disable-validation to helm diff plugin, this requires diff plugin >= 3.1.2
    # It may be helpful to deploy charts with helm api v1 CRDS
    # https://github.com/roboll/helmfile/pull/1373
    disableValidation: false
    # passes --disable-validation to helm diff plugin, this requires diff plugin >= 3.1.2
    # It is useful when any release contains custom resources for CRDs that is not yet installed onto the cluster.
    # https://github.com/roboll/helmfile/pull/1618
    disableValidationOnInstall: false
    # passes --disable-openapi-validation to helm diff plugin, this requires diff plugin >= 3.1.2
    # It may be helpful to deploy charts with helm api v1 CRDS
    # https://github.com/roboll/helmfile/pull/1373
    disableOpenAPIValidation: false
    # limit the maximum number of revisions saved per release. Use 0 for no limit (default 10)
    historyMax: 10
    # When set to `true`, skips running `helm dep up` and `helm dep build` on this release's chart.
    # Useful when the chart is broken, like seen in https://github.com/roboll/helmfile/issues/1547
    skipDeps: false
    # propagate `--post-renderer` to helmv3 template and helm install
    postRenderer: "path/to/postRenderer"
    # propagate `--post-renderer-args` to helmv3 template and helm install. This allows using Powershell
    # scripts on Windows as a post renderer
    postRendererArgs:
    - PowerShell
    - "-Command"
    - "theScript.ps1"
    # cascade `--cascade` to helmv3 delete, available values: background, foreground, or orphan, default: background
    cascade: "background"
    # insecureSkipTLSVerify is true if the TLS verification should be skipped when fetching remote chart
    insecureSkipTLSVerify: false
    # plainHttp is true if fetching the remote chart should be done using HTTP
    plainHttp: false
    # suppressDiff skip the helm diff output. Useful for charts which produces large not helpful diff, default: false
    suppressDiff: false
    # suppressOutputLineRegex is a list of regex patterns to suppress output lines from helm diff (default []), available in helmfile v0.162.0
    suppressOutputLineRegex:
      - "version"
    # syncReleaseLabels is a list of labels to be added to the release when syncing.
    syncReleaseLabels: false
    # unitTests is a list of test file or directory paths for helm-unittest integration.
    # When specified, `helmfile unittest` will run `helm unittest` with the merged values and these test paths.
    # Requires the helm-unittest plugin: https://github.com/helm-unittest/helm-unittest
    unitTests:
      - tests/vault


  # Local chart example
  - name: grafana                            # name of this release
    namespace: another                       # target namespace
    chart: ../my-charts/grafana              # the chart being installed to create this release, referenced by relative path to local helmfile
    values:
    - "../../my-values/grafana/values.yaml"             # Values file (relative path to manifest)
    - ./values/{{ requiredEnv "PLATFORM_ENV" }}/config.yaml # Values file taken from path with environment variable. $PLATFORM_ENV must be set in the calling environment.
    wait: true

#
# Advanced Configuration: Nested States
#
helmfiles:
- # Path to the helmfile state file being processed BEFORE releases in this state file
  path: path/to/subhelmfile.yaml
  # Label selector used for filtering releases in the nested state.
  # For example, `name=prometheus` in this context is equivalent to processing the nested state like
  #   helmfile -f path/to/subhelmfile.yaml -l name=prometheus sync
  selectors:
  - name=prometheus
  # Override state values
  values:
  # Values files merged into the nested state's values
  - additional.values.yaml
  # One important aspect of using values here is that they first need to be defined in the values section
  # of the origin helmfile, so in this example key1 needs to be in the values or environments.NAME.values of path/to/subhelmfile.yaml
  # Inline state values merged into the nested state's values
  - key1: val1
- # All the nested state files under `helmfiles:` is processed in the order of definition.
  # So it can be used for preparation for your main `releases`. An example would be creating CRDs required by `releases` in the parent state file.
  path: path/to/mycrd.helmfile.yaml
- # Terraform-module-like URL for importing a remote directory and use a file in it as a nested-state file
  # The nested-state file is locally checked-out along with the remote directory containing it.
  # Therefore all the local paths in the file are resolved relative to the file
  path: git::https://github.com/cloudposse/helmfiles.git@releases/kiam.yaml?ref=0.40.0
- # By default git repositories aren't updated unless the ref is updated.
  # Alternatively, refer to a named ref and disable the caching.
  path: git::ssh://git@github.com/cloudposse/helmfiles.git@releases/kiam.yaml?ref=main&cache=false
# If set to "Error", return an error when a subhelmfile points to a
# non-existent path. The default behavior is to print a warning and continue.
missingFileHandler: Error
missingFileHandlerConfig:
  # Ignores missing git branch error so that the Debug/Info/Warn handler can treat a missing branch as non-error.
  # See https://github.com/helmfile/helmfile/issues/392
  ignoreMissingGitBranch: true

#
# Advanced Configuration: Environments
#

# The list of environments managed by helmfile.
#
# The default is `environments: {"default": {}}` which implies:
#
# - `{{ .Environment.Name }}` evaluates to "default"
# - `{{ .Values }}` being empty
environments:
  # The "default" environment is available and used when `helmfile` is run without `--environment NAME`.
  default:
    # Everything from the values.yaml is available via `{{ .Values.KEY }}`.
    # Suppose `{"foo": {"bar": 1}}` contained in the values.yaml below,
    # `{{ .Values.foo.bar }}` is evaluated to `1`.
    values:
    - environments/default/values.yaml
    # Everything from the values.hcl in the `values` block is available via `{{ .Values.KEY }}`.
    # More details in its dedicated section
    - environments/default/values.hcl
    # Each entry in values can be either a file path or inline values.
    # The below is an example of inline values, which is merged to the `.Values`
    - myChartVer: 1.0.0-dev
  # Any environment other than `default` is used only when `helmfile` is run with `--environment NAME`.
  # That is, the "production" env below is used when and only when it is run like `helmfile --environment production sync`.
  production:
    values:
    - environments/production/values.yaml
    - myChartVer: 1.0.0
    # disable vault release processing
    - vault:
        enabled: false
    ## `secrets.yaml` is decrypted by `helm-secrets` and available via `{{ .Environment.Values.KEY }}`
    secrets:
    - environments/production/secrets.yaml
    # Instructs helmfile to fail when unable to find a environment values file listed under `environments.NAME.values`.
    #
    # Possible values are  "Error", "Warn", "Info", "Debug". The default is "Error".
    #
    # Use "Warn", "Info", or "Debug" if you want helmfile to not fail when a values file is missing, while just leaving
    # a message about the missing file at the log-level.
    missingFileHandler: Error
    missingFileHandlerConfig:
      # Ignores missing git branch error so that the Debug/Info/Warn handler can treat a missing branch as non-error.
      # See https://github.com/helmfile/helmfile/issues/392
      ignoreMissingGitBranch: true
    # kubeContext to use for this environment
    kubeContext: kube-context

#
# Advanced Configuration: Layering
#
# Helmfile merges all the "base" state files and this state file before processing.
#
# Assuming this state file is named `helmfile.yaml`, all the files are merged in the order of:
#   environments.yaml <- defaults.yaml <- templates.yaml <- helmfile.yaml
bases:
- environments.yaml
- defaults.yaml
- templates.yaml

#
# Advanced Configuration: API Capabilities
#
# 'helmfile template' renders releases locally without querying an actual cluster,
# and in this case `.Capabilities.APIVersions` cannot be populated.
# When a chart queries for a specific CRD or the Kubernetes version, this can lead to unexpected results.
#
# Note that `Capabilities.KubeVersion` is deprecated in Helm 3 and `helm template` won't populate it.
# All you can do is fix your chart to respect `.Capabilities.APIVersions` instead, rather than trying to figure out
# how to set `Capabilities.KubeVersion` in Helmfile.
#
# Configure a fixed list of API versions to pass to 'helm template' via the --api-versions flag with the below:
apiVersions:
- example/v1

# Set the kubeVersion to render the chart with your desired Kubernetes version.
# The flag --kube-version was deprecated in helm v3 but it was added again.
# For further information https://github.com/helm/helm/issues/7326
kubeVersion: v1.21
```

### Additional helmDefaults fields

The following `helmDefaults` fields are also available but not shown in the example above:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enableDNS` | bool | false | Enable DNS lookups when rendering templates |
| `skipCRDs` | bool | false | Skip CRDs during installation |
| `skipRefresh` | bool | false | Skip running `helm dependency up` |
| `forceConflicts` | bool | false | Force server-side apply changes against conflicts (Helm 4 only) |
| `takeOwnership` | bool | false | Take ownership of existing resources |
| `trackMode` | string | `""` | Default tracking mode for resources. See [Advanced Features](advanced-features.md#resource-tracking-with-kubedog) |
| `disableAutoDetectedKubeVersionForDiff` | bool | false | Disable auto-detected kubeVersion being passed to helm diff |

### Additional release fields

The following per-release fields are also available:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `valuesTemplate` | list | | Like `values` but template expressions are rendered before being passed to Helm |
| `setTemplate` | list | | Like `set` but template expressions are rendered before being passed to Helm |
| `apiVersions` | list | | Per-release API versions (overrides top-level `apiVersions`) |
| `kubeVersion` | string | | Per-release kube version (overrides top-level `kubeVersion`) |
| `valuesPathPrefix` | string | | Prefix for values file paths |
| `verifyTemplate` | string | | Templated verify flag (e.g., `{{ .Values.verify \| default "false" }}`) |
| `waitTemplate` | string | | Templated wait flag |
| `installedTemplate` | string | | Templated installed flag |
| `adopt` | list | | List of resources to adopt (passes `--adopt` to Helm) |
| `forceGoGetter` | bool | false | Force go-getter URL parsing for the chart field. Useful when go-getter URL parsing fails unexpectedly |
| `forceNamespace` | string | | Force namespace on all K8s resources rendered by the chart, even when the template doesn't use `{{ .Namespace }}`. Use with caution |
| `skipRefresh` | bool | false | Per-release skip for `helm dependency up` |
| `disableAutoDetectedKubeVersionForDiff` | bool | false | Disable auto-detected kubeVersion for helm diff on this release |
| `takeOwnership` | bool | false | Take ownership of existing resources for this release |
| `forceConflicts` | bool | false | Force server-side apply against conflicts (Helm 4 only) |
| `description` | string | | Description of the release |
| `enableDNS` | bool | false | Enable DNS lookups when rendering templates |

### Release tracking fields (kubedog)

See [Advanced Features](advanced-features.md#resource-tracking-with-kubedog) for more details:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `trackMode` | string | `""` | Track mode: `helm`, `helm-legacy`, or `kubedog` |
| `trackTimeout` | int | 300 | Tracking timeout in seconds |
| `trackLogs` | bool | false | Enable real-time log streaming |
| `trackKinds` | list | | Whitelist of resource kinds to track |
| `skipKinds` | list | | Blacklist of resource kinds to skip |
| `trackResources` | list | | Specific resources to track (objects with `kind`, `name`, `namespace`) |
| `kubedogQPS` | float | | QPS for kubedog kubernetes client |
| `kubedogBurst` | int | | Burst for kubedog kubernetes client |

### Hook kubectlApply

Hooks also support a `kubectlApply` field for running `kubectl apply` directly:

```yaml
releases:
- name: myapp
  chart: mychart
  hooks:
  - events: ["presync"]
    showlogs: true
    kubectlApply:
      filename: manifests/my-resource.yaml
```

Or with kustomize:

```yaml
  hooks:
  - events: ["presync"]
    showlogs: true
    kubectlApply:
      kustomize: overlays/default/
```

### Repository additional fields

| Field | Type | Description |
|-------|------|-------------|
| `registryConfig` | string | Path to registry configuration file |
| `managed` | string | Managed repository mode |

### Template Partials

Files matching `_*.tpl` in the same directory as the helmfile are automatically loaded as helper templates. For example, a file named `_helpers.tpl` can define named templates that are reusable across your helmfile:

`_helpers.tpl`:
```
{{- define "myapp.labels" -}}
app: myapp
env: {{ .Environment.Name }}
{{- end -}}
```

`helmfile.yaml`:
```yaml
releases:
- name: myapp
  chart: mychart
  values:
  - labels: {{ include "myapp.labels" . | toYaml | nindent 4 }}
```
