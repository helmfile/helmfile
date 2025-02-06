<!-- markdownlint-configure-file {
  "MD013": {
    "code_blocks": false,
    "tables": false
  },
  "MD033": false,
  "MD041": false
} -->

<div align="center" markdown="1">

# Helmfile

[![Tests](https://github.com/helmfile/helmfile/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/helmfile/helmfile/actions/workflows/ci.yaml?query=branch%3Amain)
[![Container Image Repository on GHCR](https://ghcr-badge.egpl.dev/helmfile/helmfile/latest_tag?trim=major&label=latest "Docker Repository on ghcr")](https://github.com/helmfile/helmfile/pkgs/container/helmfile)
[![Go Report Card](https://goreportcard.com/badge/github.com/helmfile/helmfile)](https://goreportcard.com/report/github.com/helmfile/helmfile)
[![Slack Community #helmfile](https://slack.sweetops.com/badge.svg)](https://slack.sweetops.com)
[![Documentation](https://readthedocs.org/projects/helmfile/badge/?version=latest&style=flat)](https://helmfile.readthedocs.io/en/latest/)

Deploy Kubernetes Helm Charts
<br />

</div>

## Status

March 2022 Update - The helmfile project has been moved to [helmfile/helmfile](https://github.com/helmfile/helmfile) from the former home `roboll/helmfile`. Please see [roboll/helmfile#1824](https://github.com/roboll/helmfile/issues/1824) for more information.

Even though Helmfile is used in production environments [across multiple organizations](users.md), it is still in its early stage of development, hence versioned 0.x.

Helmfile complies to Semantic Versioning 2.0.0 in which v0.x means that there could be backward-incompatible changes for every release.

Note that we will try our best to document any backward incompatibility. And in reality, helmfile had no breaking change for a year or so.

## About

Helmfile is a declarative spec for deploying helm charts. It lets you...

* Keep a directory of chart value files and maintain changes in version control.
* Apply CI/CD to configuration changes.
* Periodically sync to avoid skew in environments.

To avoid upgrades for each iteration of `helm`, the `helmfile` executable delegates to `helm` - as a result, `helm` must be installed.

## Highlights

**Declarative**: Write, version-control, apply the desired state file for visibility and reproducibility.

**Modules**: Modularize common patterns of your infrastructure, distribute it via Git, S3, etc. to be reused across the entire company (See [#648](https://github.com/roboll/helmfile/pull/648))

**Versatility**: Manage your cluster consisting of charts, [kustomizations](https://github.com/kubernetes-sigs/kustomize), and directories of Kubernetes resources, turning everything to Helm releases (See [#673](https://github.com/roboll/helmfile/pull/673))

**Patch**: JSON/Strategic-Merge Patch Kubernetes resources before `helm-install`ing, without forking upstream charts (See [#673](https://github.com/roboll/helmfile/pull/673))

## Installation

* download one of [releases](https://github.com/helmfile/helmfile/releases)
* [run as a container](https://helmfile.readthedocs.io/en/latest/#running-as-a-container)
* Archlinux: install via `pacman -S helmfile`
* openSUSE: install via `zypper in helmfile` assuming you are on Tumbleweed; if you are on Leap you must add the [kubic](https://download.opensuse.org/repositories/devel:/kubic/) repo for your distribution version once before that command, e.g. `zypper ar https://download.opensuse.org/repositories/devel:/kubic/openSUSE_Leap_\$releasever kubic`
* Windows (using [scoop](https://scoop.sh/)): `scoop install helmfile`
* macOS (using [homebrew](https://brew.sh/)): `brew install helmfile`

### Running as a container

The [Helmfile Docker images are available in GHCR](https://github.com/helmfile/helmfile/pkgs/container/helmfile). There is no `latest` tag, since the `0.x` versions can contain breaking changes, so make sure you pick the right tag. Example using `helmfile 0.156.0`:

```sh-session
$ docker run --rm --net=host -v "${HOME}/.kube:/helm/.kube" -v "${HOME}/.config/helm:/helm/.config/helm" -v "${PWD}:/wd" --workdir /wd ghcr.io/helmfile/helmfile:v0.156.0 helmfile sync
```

You can also use a shim to make calling the binary easier:

```sh-session
$ printf '%s\n' '#!/bin/sh' 'docker run --rm --net=host -v "${HOME}/.kube:/helm/.kube" -v "${HOME}/.config/helm:/helm/.config/helm" -v "${PWD}:/wd" --workdir /wd ghcr.io/helmfile/helmfile:v0.156.0 helmfile "$@"' |
    tee helmfile
$ chmod +x helmfile
$ ./helmfile sync
```

## Getting Started

Let's start with a simple `helmfile` and gradually improve it to fit your use-case!

Suppose the `helmfile.yaml` representing the desired state of your helm releases looks like:

```yaml
repositories:
 - name: prometheus-community
   url: https://prometheus-community.github.io/helm-charts

releases:
- name: prom-norbac-ubuntu
  namespace: prometheus
  chart: prometheus-community/prometheus
  set:
  - name: rbac.create
    value: false
```

Install required dependencies using [init](https://helmfile.readthedocs.io/en/latest/#init):

```console
helmfile init
```

Sync your Kubernetes cluster state to the desired one by running:

```console
helmfile apply
```

Congratulations! You now have your first Prometheus deployment running inside your cluster.

Iterate on the `helmfile.yaml` by referencing:

* [Configuration](#configuration)
* [CLI reference](#cli-reference).
* [Helmfile Best Practices Guide](writing-helmfile.md)

## Configuration

**CAUTION**: This documentation is for the development version of Helmfile. If you are looking for the documentation for any of releases, please switch to the corresponding release tag like [v0.143.4](https://github.com/helmfile/helmfile/tree/v0.143.4).

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
helmBinary: path/to/helm3

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
  templateArgs:
    - "--dry-run=server"
  # verify the chart before upgrading (only works with packaged charts not directories) (default false)
  verify: true
  keyring: path/to/keyring.gpg
  #  --skip-schema-validation flag to helm 'install', 'upgrade' and 'lint', starts with helm 3.16.0 (default false)
  skipSchemaValidation: false
  # wait for k8s resources via --wait. (default false)
  wait: true
  # if set and --wait enabled, will retry any failed check on resource state subject to the specified number of retries (default 0)
  waitRetries: 3
  # if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout (default false, Implemented in Helm3.5)
  waitForJobs: true
  # time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks, and waits on pod/pvc/svc/deployment readiness) (default 300)
  timeout: 600
  # performs pods restart for the resource if applicable (default false)
  recreatePods: true
  # forces resource update through delete/recreate if needed (default false)
  force: false
  # limit the maximum number of revisions saved per release. Use 0 for no limit. (default 10)
  historyMax: 10
  # when using helm 3.2+, automatically create release namespaces if they do not exist (default true)
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
    createNamespace: true                  # helm 3.2+ automatically create release namespace (default true)
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
    # Override helmDefaults options for verify, wait, waitForJobs, timeout, recreatePods and force.
    verify: true
    keyring: path/to/keyring.gpg
    #  --skip-schema-validation flag to helm 'install', 'upgrade' and 'lint', starts with helm 3.16.0 (default false)
    skipSchemaValidation: false
    wait: true
    waitRetries: 3
    waitForJobs: true
    timeout: 60
    recreatePods: true
    force: false
    # set `false` to uninstall this release on sync.  (default true)
    installed: true
    # restores previous state in case of failed release (default false)
    atomic: true
    # when true, cleans up any new resources created during a failed release (default false)
    cleanupOnFail: false
    # --kube-context to be passed to helm commands
    # See https://github.com/roboll/helmfile/issues/642
    # (default "", which means the standard kubeconfig, either ~/kubeconfig or the file pointed by $KUBECONFIG environment variable)
    kubeContext: kube-context
    # passes --disable-validation to helm 3 diff plugin, this requires diff plugin >= 3.1.2
    # It may be helpful to deploy charts with helm api v1 CRDS
    # https://github.com/roboll/helmfile/pull/1373
    disableValidation: false
    # passes --disable-validation to helm 3 diff plugin, this requires diff plugin >= 3.1.2
    # It is useful when any release contains custom resources for CRDs that is not yet installed onto the cluster.
    # https://github.com/roboll/helmfile/pull/1618
    disableValidationOnInstall: false
    # passes --disable-openapi-validation to helm 3 diff plugin, this requires diff plugin >= 3.1.2
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
# If set to "Error", return an error when a subhelmfile points to a
# non-existent path. The default behavior is to print a warning and continue.
missingFileHandler: Error

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

## Templating

Helmfile uses [Go templates](https://godoc.org/text/template) for templating your helmfile.yaml. While go ships several built-in functions, we have added all of the functions in the [Sprig library](https://godoc.org/github.com/Masterminds/sprig).

We also added the following functions:

* [`env`](templating_funcs.md#env)
* [`requiredEnv`](templating_funcs.md#requiredenv)
* [`exec`](templating_funcs.md#exec)
* [`envExec`](templating_funcs.md#envexec)
* [`readFile`](templating_funcs.md#readfile)
* [`readDir`](templating_funcs.md#readdir)
* [`readDirEntries`](templating_funcs.md#readdirentries)
* [`toYaml`](templating_funcs.md#toyaml)
* [`fromYaml`](templating_funcs.md#fromyaml)
* [`setValueAtPath`](templating_funcs.md#setvalueatpath)
* [`get`](templating_funcs.md#get) (Sprig's original `get` is available as `sprigGet`)
* [`getOrNil`](templating_funcs.md#getornil)
* [`tpl`](templating_funcs.md#tpl)
* [`required`](templating_funcs.md#required)
* [`fetchSecretValue`](templating_funcs.md#fetchsecretvalue)
* [`expandSecretRefs`](templating_funcs.md#expandsecretrefs)
* [`include`](templating_funcs.md#include)

More details on each function can be found at the ["Template Functions" page in our documentation](templating_funcs.md).


## Using environment variables

Environment variables can be used in most places for templating the helmfile. Currently this is supported for `name`, `namespace`, `value` (in set), `values` and `url` (in repositories).

Examples:

```yaml
repositories:
- name: your-private-git-repo-hosted-charts
  url: https://{{ requiredEnv "GITHUB_TOKEN"}}@raw.githubusercontent.com/kmzfs/helm-repo-in-github/master/
```

```yaml
releases:
  - name: {{ requiredEnv "NAME" }}-vault
    namespace: {{ requiredEnv "NAME" }}
    chart: roboll/vault-secret-manager
    values:
      - db:
          username: {{ requiredEnv "DB_USERNAME" }}
          password: {{ requiredEnv "DB_PASSWORD" }}
    set:
      - name: proxy.domain
        value: {{ requiredEnv "PLATFORM_ID" }}.my-domain.com
      - name: proxy.scheme
        value: {{ env "SCHEME" | default "https" }}
```

### Note

If you wish to treat your enviroment variables as strings always, even if they are boolean or numeric values you can use `{{ env "ENV_NAME" | quote }}` or `"{{ env "ENV_NAME" }}"`. These approaches also work with `requiredEnv`.

### Useful internal Helmfile environment variables

Helmfile uses some OS environment variables to override default behaviour:

* `HELMFILE_DISABLE_INSECURE_FEATURES` - disable insecure features, expecting `true` lower case
* `HELMFILE_DISABLE_RUNNER_UNIQUE_ID` - disable unique logging ID, expecting any non-empty value
* `HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS` - disable insecure template functions, expecting `true` lower case
* `HELMFILE_USE_HELM_STATUS_TO_CHECK_RELEASE_EXISTENCE` - expecting non-empty value to use `helm status` to check release existence, instead of `helm list` which is the default behaviour
* `HELMFILE_EXPERIMENTAL` - enable experimental features, expecting `true` lower case
* `HELMFILE_ENVIRONMENT` - specify [Helmfile environment](https://helmfile.readthedocs.io/en/latest/#environment), it has lower priority than CLI argument `--environment`
* `HELMFILE_TEMPDIR` - specify directory to store temporary files
* `HELMFILE_UPGRADE_NOTICE_DISABLED` - expecting any non-empty value to skip the check for the latest version of Helmfile in [helmfile version](https://helmfile.readthedocs.io/en/latest/#version)
* `HELMFILE_V1MODE` - Helmfile v0.x behaves like v1.x with `true`, Helmfile v1.x behaves like v0.x with `false` as value
* `HELMFILE_GOCCY_GOYAML` - use *goccy/go-yaml* instead of *gopkg.in/yaml.v2*.  It's `false` by default in Helmfile v0.x and `true` by default for Helmfile v1.x.
* `HELMFILE_CACHE_HOME` - specify directory to store cached files for remote operations
* `HELMFILE_FILE_PATH` - specify the path to the helmfile.yaml file
* `HELMFILE_INTERACTIVE` - enable interactive mode, expecting `true` lower case. The same as `--interactive` CLI flag

## CLI Reference

```
Declaratively deploy your Kubernetes manifests, Kustomize configs, and Charts as Helm releases in one shot
V1 mode = false
YAML library = gopkg.in/yaml.v2

Usage:
  helmfile [command]

Available Commands:
  apply        Apply all resources from state file only when there are changes
  build        Build all resources from state file
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
  version      Print the CLI version
  write-values Write values files for releases. Similar to `helmfile template`, write values files instead of manifests.

Flags:
      --allow-no-matching-release             Do not exit with an error code if the provided selector has no matching releases.
  -c, --chart string                          Set chart. Uses the chart set in release by default, and is available in template as {{ .Chart }}
      --color                                 Output with color
      --debug                                 Enable verbose output for Helm and set log-level to debug, this disables --quiet/-q effect
      --disable-force-update                  do not force helm repos to update when executing "helm repo add"
      --enable-live-output                    Show live output from the Helm binary Stdout/Stderr into Helmfile own Stdout/Stderr.
                                              It only applies for the Helm CLI commands, Stdout/Stderr for Hooks are still displayed only when it's execution finishes.
  -e, --environment string                    specify the environment name. Overrides "HELMFILE_ENVIRONMENT" OS environment variable when specified. defaults to "default"
  -f, --file helmfile.yaml                    load config from file or directory. defaults to "helmfile.yaml" or "helmfile.yaml.gotmpl" or "helmfile.d" (means "helmfile.d/*.yaml" or "helmfile.d/*.yaml.gotmpl") in this preference. Specify - to load the config from the standard input.
  -b, --helm-binary string                    Path to the helm binary (default "helm")
  -h, --help                                  help for helmfile
  -i, --interactive                           Request confirmation before attempting to modify clusters
      --kube-context string                   Set kubectl context. Uses current context by default
  -k, --kustomize-binary string               Path to the kustomize binary (default "kustomize")
      --log-level string                      Set log level, default info (default "info")
  -n, --namespace string                      Set namespace. Uses the namespace set in the context by default, and is available in templates as {{ .Namespace }}
      --no-color                              Output without color
  -q, --quiet                                 Silence output. Equivalent to log-level warn
  -l, --selector stringArray                  Only run using the releases that match labels. Labels can take the form of foo=bar or foo!=bar.
                                              A release must match all labels in a group in order to be used. Multiple groups can be specified at once.
                                              "--selector tier=frontend,tier!=proxy --selector tier=backend" will match all frontend, non-proxy releases AND all backend releases.
                                              The name of a release can be used as a label: "--selector name=myrelease"
      --skip-deps                             skip running "helm repo update" and "helm dependency build"
      --state-values-file stringArray         specify state values in a YAML file. Used to override .Values within the helmfile template (not values template).
      --state-values-set stringArray          set state values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).
      --state-values-set-string stringArray   set state STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).
      --strip-args-values-on-exit-error       Strip the potential secret values of the helm command args contained in a helmfile error message (default true)
  -v, --version                               version for helmfile

Use "helmfile [command] --help" for more information about a command.
```

### init

The `helmfile init` sub-command checks the dependencies required for helmfile operation, such as `helm`, `helm diff plugin`, `helm secrets plugin`, `helm helm-git plugin`, `helm s3 plugin`. When it does not exist or the version is too low, it can be installed automatically.

### cache

The `helmfile cache` sub-command is designed for cache management. Go-getter-backed remote file system are cached by `helmfile`. There is no TTL implemented, if you need to update the cached files or directories, you need to clean individually or run a full cleanup with `helmfile cache cleanup`

### sync

The `helmfile sync` sub-command sync your cluster state as described in your `helmfile`. The default helmfile is `helmfile.yaml`, but any YAML file can be passed by specifying a `--file path/to/your/yaml/file` flag.

Under the covers, Helmfile executes `helm upgrade --install` for each `release` declared in the manifest, by optionally decrypting [secrets](#secrets) to be consumed as helm chart values. It also updates specified chart repositories and updates the
dependencies of any referenced local charts.

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

## Paths Overview

Using manifest files in conjunction with command line argument can be a bit confusing.

A few rules to clear up this ambiguity:

* Absolute paths are always resolved as absolute paths
* Relative paths referenced *in* the Helmfile manifest itself are relative to that manifest
* Relative paths referenced on the command line are relative to the current working directory the user is in
- Relative paths referenced from within the helmfile loaded from the standard input using `helmfile -f -` are relative to the current working directory

For additional context, take a look at [paths examples](paths.md).

## Labels Overview

A selector can be used to only target a subset of releases when running Helmfile. This is useful for large helmfiles with releases that are logically grouped together.

Labels are simple key value pairs that are an optional field of the release spec. When selecting by label, the search can be inverted. `tier!=backend` would match all releases that do NOT have the `tier: backend` label. `tier=fronted` would only match releases with the `tier: frontend` label.

Multiple labels can be specified using `,` as a separator. A release must match all selectors in order to be selected for the final helm command.

The `selector` parameter can be specified multiple times. Each parameter is resolved independently so a release that matches any parameter will be used.

`--selector tier=frontend --selector tier=backend` will select all the charts.

In addition to user supplied labels, the name, the namespace, and the chart are available to be used as selectors.  The chart will just be the chart name excluding the repository (Example `stable/filebeat` would be selected using `--selector chart=filebeat`).

`commonLabels` can be used when you want to apply the same label to all releases and use [templating](##Templates) based on that.
For instance, you install a number of charts on every customer but need to provide different values file per customer.

templates/common.yaml:

```yaml
templates:
  nginx: &nginx
    name: nginx
    chart: stable/nginx-ingress
    values:
    - ../values/common/{{ .Release.Name }}.yaml
    - ../values/{{ .Release.Labels.customer }}/{{ .Release.Name }}.yaml

  cert-manager: &cert-manager
    name: cert-manager
    chart: jetstack/cert-manager
    values:
    - ../values/common/{{ .Release.Name }}.yaml
    - ../values/{{ .Release.Labels.customer }}/{{ .Release.Name }}.yaml
```

helmfile.yaml:

```yaml
{{ readFile "templates/common.yaml" }}

commonLabels:
  customer: company

releases:
- <<: *nginx
- <<: *cert-manager
```

## Templates

You can use go's text/template expressions in `helmfile.yaml` and `values.yaml.gotmpl` (templated helm values files). `values.yaml` references will be used verbatim. In other words:

* for value files ending with `.gotmpl`, template expressions will be rendered
* for plain value files (ending in `.yaml`), content will be used as-is

In addition to built-in ones, the following custom template functions are available:

* `readFile` reads the specified local file and generate a golang string
* `readDir` reads the files within provided directory path. (folders are excluded)
* `readDirEntries` Returns a list of [https://pkg.go.dev/os#DirEntry](DirEntry) within provided directory path
* `fromYaml` reads a golang string and generates a map
* `setValueAtPath PATH NEW_VALUE` traverses a golang map, replaces the value at the PATH with NEW_VALUE
* `toYaml` marshals a map into a string
* `get` returns the value of the specified key if present in the `.Values` object, otherwise will return the default value defined in the function

### Values Files Templates

You can reference a template of values file in your `helmfile.yaml` like below:

```yaml
releases:
- name: myapp
  chart: mychart
  values:
  - values.yaml.gotmpl
```

Every values file whose file extension is `.gotmpl` is considered as a template file.

Suppose `values.yaml.gotmpl` was something like:

```yaml
{{ readFile "values.yaml" | fromYaml | setValueAtPath "foo.bar" "FOO_BAR" | toYaml }}
```

And `values.yaml` was:

```yaml
foo:
  bar: ""
```

The resulting, temporary values.yaml that is generated from `values.yaml.gotmpl` would become:

```yaml
foo:
  # Notice `setValueAtPath "foo.bar" "FOO_BAR"` in the template above
  bar: FOO_BAR
```

## Refactoring `helmfile.yaml` with values files templates

One of expected use-cases of values files templates is to keep `helmfile.yaml` small and concise.

See the example `helmfile.yaml` below:

```yaml
releases:
  - name: {{ requiredEnv "NAME" }}-vault
    namespace: {{ requiredEnv "NAME" }}
    chart: roboll/vault-secret-manager
    values:
      - db:
          username: {{ requiredEnv "DB_USERNAME" }}
          password: {{ requiredEnv "DB_PASSWORD" }}
    set:
      - name: proxy.domain
        value: {{ requiredEnv "PLATFORM_ID" }}.my-domain.com
      - name: proxy.scheme
        value: {{ env "SCHEME" | default "https" }}
```

The `values` and `set` sections of the config file can be separated out into a template:

`helmfile.yaml`:

```yaml
releases:
  - name: {{ requiredEnv "NAME" }}-vault
    namespace: {{ requiredEnv "NAME" }}
    chart: roboll/vault-secret-manager
    values:
    - values.yaml.gotmpl
```

`values.yaml.gotmpl`:

```yaml
db:
  username: {{ requiredEnv "DB_USERNAME" }}
  password: {{ requiredEnv "DB_PASSWORD" }}
proxy:
  domain: {{ requiredEnv "PLATFORM_ID" }}.my-domain.com
  scheme: {{ env "SCHEME" | default "https" }}
```

## Environment

When you want to customize the contents of `helmfile.yaml` or `values.yaml` files per environment, use this feature.

You can define as many environments as you want under `environments` in `helmfile.yaml`.
`environments` section should be separated from `releases` with `---`.

The environment name defaults to `default`, that is, `helmfile sync` implies the `default` environment.
The selected environment name can be referenced from `helmfile.yaml` and `values.yaml.gotmpl` by `{{ .Environment.Name }}`.

If you want to specify a non-default environment, provide a `--environment NAME` flag to `helmfile` like `helmfile --environment production sync`.

The below example shows how to define a production-only release:

```yaml
environments:
  default:
  production:

---

releases:
- name: newrelic-agent
  installed: {{ eq .Environment.Name "production" | toYaml }}
  # snip
- name: myapp
  # snip
```

### Environment Values
Helmfile supports 3 values languages :
- Straight yaml
- Go templates to generate straight yaml
- HCL

Environment Values allows you to inject a set of values specific to the selected environment, into `values.yaml` templates.
Use it to inject common values from the environment to multiple values files, to make your configuration DRY.

Suppose you have three files `helmfile.yaml`, `production.yaml` and `values.yaml.gotmpl`:

`helmfile.yaml`

```yaml
environments:
  production:
    values:
    - production.yaml

---

releases:
- name: myapp
  values:
  - values.yaml.gotmpl
```

`production.yaml`

```yaml
domain: prod.example.com
releaseName: prod
```

`values.yaml.gotmpl`

```yaml
domain: {{ .Values | get "domain" "dev.example.com" }}
```

`helmfile sync` installs `myapp` with the value `domain=dev.example.com`,
whereas `helmfile --environment production sync` installs the app with the value `domain=prod.example.com`.

For even more flexibility, you can now use values declared in the `environments:` section in other parts of your helmfiles:

consider:
`default.yaml`

```yaml
domain: dev.example.com
releaseName: dev
```

```yaml
environments:
  default:
    values:
    - default.yaml
  production:
    values:
    - production.yaml #  bare .yaml file, content will be used verbatim
    - other.yaml.gotmpl  #  template directives with potential side-effects like `exec` and `readFile` will be honoured

---

releases:
- name: myapp-{{ .Values.releaseName }} # release name will be one of `dev` or `prod` depending on selected environment
  values:
  - values.yaml.gotmpl
- name: production-specific-release
  # this release would be installed only if selected environment is `production`
  installed: {{ eq .Values.releaseName "prod" | toYaml }}
  ...
```

#### HCL specifications

Since Helmfile v0.164.0, HCL language is supported for environment values only.
HCL values supports interpolations and sharing values across files

* Only `.hcl` suffixed files will be interpreted as is
* Helmfile supports 2 differents blocks: `values` and `locals`
* `values` block is a shared block where all values are accessible everywhere in all loaded files
* `locals` block can't reference external values apart from the ones in the block itself, and where its defined values are only accessible in its local file
* Only values in `values` blocks are made available to the final root `.Values` (e.g : ` values { myvar = "var" }` is accessed through `{{ .Values.myvar }}`)
* There can only be 1 `locals` block per file
* Helmfile hcl `values` are referenced using the `hv` accessor.
* Helmfile hcl `locals` are referenced using the `local` accessor.
* Duplicated variables across .hcl `values` blocks are forbidden (An error will pop up specifying where are the duplicates)
* All cty [standard library functions](`https://pkg.go.dev/github.com/zclconf/go-cty@v1.14.3/cty/function/stdlib`) are available and custom functions could be created in the future

Consider the following example :

```terraform
# values1.hcl
locals {
  hostname = "host1"
}
values {
  domain = "DEV.EXAMPLE.COM"
  hostnameV1 = "${local.hostname}.${lower(hv.domain)}" # "host1.dev.example.com"
}
```
```terraform
# values2.hcl
locals {
  hostname = "host2"
}

values {
  hostnameV2 = "${local.hostname}.${hv.domain}" # "host2.DEV.EXAMPLE.COM"
}
```
#### Note on Environment.Values vs Values

The `{{ .Values.foo }}` syntax is the recommended way of using environment values.

Prior to this [pull request](https://github.com/roboll/helmfile/pull/647), environment values were made available through the `{{ .Environment.Values.foo }}` syntax.
This is still working but is **deprecated** and the new `{{ .Values.foo }}` syntax should be used instead.

You can read more infos about the feature proposal [here](https://github.com/roboll/helmfile/issues/640).

### Environment Secrets

Environment Secrets *(not to be confused with Kubernetes Secrets)* are encrypted versions of `Environment Values`.
You can list any number of `secrets.yaml` files created using `helm secrets` or `sops`, so that
Helmfile could automatically decrypt and merge the secrets into the environment values.

First you must have the [helm-secrets](https://github.com/jkroepke/helm-secrets) plugin installed along with a
`.sops.yaml` file to configure the method of encryption (this can be in the same directory as your helmfile or
in the subdirectory containing your secrets files).

Then suppose you have a secret `foo.bar` defined in `environments/production/secrets.yaml`:

```yaml
foo.bar: "mysupersecretstring"
```

You can then encrypt it with `helm secrets enc environments/production/secrets.yaml`

Then reference that encrypted file in `helmfile.yaml`:

```yaml
environments:
  production:
    secrets:
    - environments/production/secrets.yaml

---

releases:
- name: myapp
  chart: mychart
  values:
  - values.yaml.gotmpl
```

Then the environment secret `foo.bar` can be referenced by the below template expression in your `values.yaml.gotmpl`:

```yaml
{{ .Values.foo.bar }}
```

#### Loading remote Environment secrets files

Since Helmfile v0.149.0, you can use `go-getter`-style URLs to refer to remote secrets files, the same way as in values files:
```yaml
environments:
  staging:
    secrets:
      - git::https://{{ env "GITHUB_PAT" }}@github.com/org/repo.git@/environments/staging.secret.yaml?ref=main
      - http://$HOSTNAME/artifactory/example-repo-local/test.tgz@environments/staging.secret.yaml
  production:
    secrets:
      - git::https://{{ env "GITHUB_PAT" }}@github.com/org/repo.git@/environments/production.secret.yaml?ref=main
      - http://$HOSTNAME/artifactory/example-repo-local/test.tgz@environments/production.secret.yaml
```

### Loading remote Environment values files

Since Helmfile v0.118.8, you can use `go-getter`-style URLs to refer to remote values files:

```yaml
environments:
  cluster-azure-us-west:
    values:
      - git::https://git.company.org/helmfiles/global/azure.yaml?ref=master
      - git::https://git.company.org/helmfiles/global/us-west.yaml?ref=master
      - git::https://gitlab.com/org/repository-name.git@/config/config.test.yaml?ref=main # Public Gilab Repo
  cluster-gcp-europe-west:
    values:
      - git::https://git.company.org/helmfiles/global/gcp.yaml?ref=master
      - git::https://git.company.org/helmfiles/global/europe-west.yaml?ref=master
      - git::https://ci:{{ env "CI_JOB_TOKEN" }}@gitlab.com/org/repository-name.git@/config.dev.yaml?ref={{ env "APP_COMMIT_SHA" }}  # Private Gitlab Repo
  staging:
     values:
      - git::https://{{ env "GITHUB_PAT" }}@github.com/[$GITHUB_ORGorGITHUB_USER]/repository-name.git@/values.dev.yaml?ref=main #Github Private repo
      - http://$HOSTNAME/artifactory/example-repo-local/test.tgz@values.yaml #Artifactory url
---

releases:
  - ...
```

Since Helmfile v0.158.0, support more protocols, such as: s3, https, http
```
values:
  - s3::https://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml
  - s3://helm-s3-values-example/subdir/values.yaml
  - https://john:doe@helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml
  - http://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml
```

For more information about the supported protocols see: [go-getter Protocol-Specific Options](https://github.com/hashicorp/go-getter#protocol-specific-options-1).

This is particularly useful when you co-locate helmfiles within your project repo but want to reuse the definitions in a global repo.

### Environment values precedence
With the introduction of HCL, a new value precedence was introduced over environment values.
Here is the order of precedence from least to greatest (the last one overrides all others)
1. `yaml` / `yaml.gotmpl`
2. `hcl`
3. `yaml` secrets

Example:

---

```yaml
# values1.yaml
domain: "dev.example.com"
```

```terraform
# values2.hcl
values {
  domain = "overdev.example.com"
  willBeOverriden = "override_me"
}
```

```yaml
# secrets.yml (assuming this one has been encrypted)
willBeOverriden: overrided
```

```
# helmfile.yaml.gotmpl
environments:
  default:
    values:
    - value1.yaml
    - value2.hcl
    secrets:
    - secrets.yml
---
releases:
- name: random-release
  [...]
  values:
    domain: "{{ .Values.domain }}" # == "overdev.example.com"
    willBeOverriden: "{{ .Values.willBeOverriden }}" # == "overrided"
```
## DAG-aware installation/deletion ordering with `needs`

`needs` controls the order of the installation/deletion of the release:

```yaml
releases:
- name: somerelease
  needs:
  - [[KUBECONTEXT/]NAMESPACE/]anotherelease
```

Be aware that you have to specify the kubecontext and namespace name if you configured one for the release(s).

All the releases listed under `needs` are installed before(or deleted after) the release itself.

For the following example, `helmfile [sync|apply]` installs releases in this order:

1. logging
2. servicemesh
3. myapp1 and myapp2

```yaml
  - name: myapp1
    chart: charts/myapp
    needs:
    - servicemesh
    - logging
  - name: myapp2
    chart: charts/myapp
    needs:
    - servicemesh
    - logging
  - name: servicemesh
    chart: charts/istio
    needs:
    - logging
  - name: logging
    chart: charts/fluentd
```

Note that all the releases in a same group is installed concurrently. That is, myapp1 and myapp2 are installed concurrently.

On `helmfile [delete|destroy]`, deletions happen in the reverse order.

That is, `myapp1` and `myapp2` are deleted first, then `servicemesh`, and finally `logging`.

### Selectors and `needs`

When using selectors/labels, `needs` are ignored by default. This behaviour can be overruled with a few parameters:

| Parameter | default | Description |
|---|---|---|
| `--skip-needs` | `true` | `needs` are ignored (default behavior).  |
| `--include-needs` | `false` | The direct `needs` of the selected release(s) will be included. |
| `--include-transitive-needs` | `false` | The direct and transitive `needs` of the selected release(s) will be included. |

Let's look at an example to illustrate how the different parameters work:

```yaml
releases:
- name: serviceA
  chart: my/chart
  needs:
  - serviceB
- name: serviceB
  chart: your/chart
  needs:
  - serviceC
- name: serviceC
  chart: her/chart
- name: serviceD
  chart: his/chart
```

| Command | Included Releases Order | Explanation |
|---|---|---|
| `helmfile -l name=serviceA sync` | - `serviceA` | By default no needs are included. |
| `helmfile -l name=serviceA sync --include-needs` | - `serviceB`<br>- `serviceA` | `serviceB` is now part of the release as it is a direct need of `serviceA`.  |
| `helmfile -l name=serviceA sync --include-transitive-needs` | - `serviceC`<br>- `serviceB`<br>- `serviceA` | `serviceC` is now also part of the release as it is a direct need of `serviceB` and therefore a transitive need of `serviceA`.  |

Note that `--include-transitive-needs` will override any potential exclusions done by selectors or conditions. So even if you explicitly exclude a release via a selector it will still be part of the deployment in case it is a direct or transitive need of any of the specified releases.

## Separating helmfile.yaml into multiple independent files

Once your `helmfile.yaml` got to contain too many releases,
split it into multiple yaml files.

Recommended granularity of helmfile.yaml files is "per microservice" or "per team".
And there are two ways to organize your files.

* Single directory
* Glob patterns

### Single directory

`helmfile -f path/to/directory` loads and runs all the yaml files under the specified directory, each file as an independent helmfile.yaml.
The default helmfile directory is `helmfile.d`, that is,
in case helmfile is unable to locate `helmfile.yaml`, it tries to locate `helmfile.d/*.yaml`.

All the yaml files under the specified directory are processed in the alphabetical order. For example, you can use a `<two digit number>-<microservice>.yaml` naming convention to control the sync order.

* `helmfile.d`/
  * `00-database.yaml`
  * `00-backend.yaml`
  * `01-frontend.yaml`

### Glob patterns

In case you want more control over how multiple `helmfile.yaml` files are organized, use `helmfiles:` configuration key in the `helmfile.yaml`:

Suppose you have multiple microservices organized in a Git repository that looks like:

* `myteam/` (sometimes it is equivalent to a k8s ns, that is `kube-system` for `clusterops` team)
    * `apps/`
        * `filebeat/`
            * `helmfile.yaml` (no `charts/` exists because it depends on the stable/filebeat chart hosted on the official helm charts repository)
            * `README.md` (each app managed by my team has a dedicated README maintained by the owners of the app)
        * `metricbeat/`
            * `helmfile.yaml`
            * `README.md`
        * `elastalert-operator/`
            * `helmfile.yaml`
            * `README.md`
            * `charts/`
                * `elastalert-operator/`
                    * `<the content of the local helm chart>`

The benefits of this structure is that you can run `git diff` to locate in which directory=microservice a git commit has changes.
It allows your CI system to run a workflow for the changed microservice only.

A downside of this is that you don't have an obvious way to sync all microservices at once. That is, you have to run:

```bash
for d in apps/*; do helmfile -f $d diff; if [ $? -eq 2 ]; then helmfile -f $d sync; fi; done
```

At this point, you'll start writing a `Makefile` under `myteam/` so that `make sync-all` will do the job.

It does work, but you can rely on the Helmfile feature instead.

Put `myteam/helmfile.yaml` that looks like:

```yaml
helmfiles:
- apps/*/helmfile.yaml
```

So that you can get rid of the `Makefile` and the bash snippet.
Just run `helmfile sync` inside `myteam/`, and you are done.

All the files are sorted alphabetically per group = array item inside `helmfiles:`, so that you have granular control over ordering, too.

#### selectors

When composing helmfiles you can use selectors from the command line as well as explicit selectors inside the parent helmfile to filter the releases to be used.

```yaml
helmfiles:
- apps/*/helmfile.yaml
- path: apps/a-helmfile.yaml
  selectors:          # list of selectors
  - name=prometheus
  - tier=frontend
- path: apps/b-helmfile.yaml # no selector, so all releases are used
  selectors: []
- path: apps/c-helmfile.yaml # parent selector to be used or cli selector for the initial helmfile
  selectorsInherited: true
```

* When a selector is specified, only this selector applies and the parents or CLI selectors are ignored.
* When not selector is specified there are 2 modes for the selector inheritance because we would like to change the current inheritance behavior (see [issue #344](https://github.com/roboll/helmfile/issues/344)  ).
  * Legacy mode, sub-helmfiles without selectors inherit selectors from their parent helmfile. The initial helmfiles inherit from the command line selectors.
  * explicit mode, sub-helmfile without selectors do not inherit from their parent or the CLI selector. If you want them to inherit from their parent selector then use `selectorsInherited: true`. To enable this explicit mode you need to set the following environment variable `HELMFILE_EXPERIMENTAL=explicit-selector-inheritance` (see [experimental](#experimental-features)).
* Using `selector: []` will select all releases regardless of the parent selector or cli for the initial helmfile
* using `selectorsInherited: true` make the sub-helmfile selects releases with the parent selector or the cli for the initial helmfile. You cannot specify an explicit selector while using `selectorsInherited: true`

## Importing values from any source

The `exec` template function that is available in `values.yaml.gotmpl` is useful for importing values from any source
that is accessible by running a command:

A usual usage of `exec` would look like this:

```yaml
mysetting: |
{{ exec "./mycmd" (list "arg1" "arg2" "--flag1") | indent 2 }}
```

Or even with a pipeline:

```yaml
mysetting: |
{{ yourinput | exec "./mycmd-consume-stdin" (list "arg1" "arg2") | indent 2 }}
```

The possibility is endless. Try importing values from your golang app, bash script, jsonnet, or anything!

Then `envExec` same as `exec`, but it can receive a dict as the envs.

A usual usage of `envExec` would look like this:

```yaml
mysetting: |
{{ envExec (dict "envkey" "envValue") "./mycmd" (list "arg1" "arg2" "--flag1") | indent 2 }}
```

## Hooks

A Helmfile hook is a per-release extension point that is composed of:

* `events`
* `command`
* `args`
* `showlogs`

Helmfile triggers various `events` while it is running.
Once `events` are triggered, associated `hooks` are executed, by running the `command` with `args`. The standard output of the `command` will be displayed if `showlogs` is set and it's value is `true`.

Hooks exec order follows the order of definition in the helmfile state.

Currently supported `events` are:

* `prepare`
* `preapply`
* `presync`
* `preuninstall`
* `postuninstall`
* `postsync`
* `cleanup`

Hooks associated to `prepare` events are triggered after each release in your helmfile is loaded from YAML, before execution.
`prepare` hooks are triggered on the release as long as it is not excluded by the helmfile selector(e.g. `helmfile -l key=value`).

Hooks associated to `presync` events are triggered before each release is synced (installed or upgraded) on the cluster.
This is the ideal event to execute any commands that may mutate the cluster state as it will not be run for read-only operations like `lint`, `diff` or `template`.

`preapply` hooks are triggered before a release is uninstalled, installed, or upgraded as part of `helmfile apply`.
This is the ideal event to hook into when you are going to use `helmfile apply` for every kind of change and you want the hook to be triggered regardless of whether the releases have changed or not. Be sure to make each `preapply` hook command idempotent. Otherwise, rerunning helmfile-apply on a transient failure may end up either breaking your cluster, or the hook that runs for the second time will never succeed.

`preuninstall` hooks are triggered immediately before a release is uninstalled as part of `helmfile apply`, `helmfile sync`, `helmfile delete`, and `helmfile destroy`.

`postuninstall` hooks are triggered immediately after successful uninstall of a release while running `helmfile apply`, `helmfile sync`, `helmfile delete`, `helmfile destroy`.

`postsync` hooks are triggered after each release is synced (installed or upgraded) on the cluster, regardless if the sync was successful or not.
This is the ideal place to execute any commands that may mutate the cluster state as it will not be run for read-only operations like `lint`, `diff` or `template`.

`cleanup` hooks are triggered after each release is processed.
This is the counterpart to `prepare`, as any release on which `prepare` has been triggered gets `cleanup` triggered as well.

The following is an example hook that just prints the contextual information provided to hook:

```yaml
releases:
- name: myapp
  chart: mychart
  # *snip*
  hooks:
  - events: ["prepare", "cleanup"]
    showlogs: true
    command: "echo"
    args: ["{{`{{.Environment.Name}}`}}", "{{`{{.Release.Name}}`}}", "{{`{{.HelmfileCommand}}`}}\
"]
```

Let's say you ran `helmfile --environment prod sync`, the above hook results in executing:

```
echo {{Environment.Name}} {{.Release.Name}} {{.HelmfileCommand}}
```

Whereas the template expressions are executed thus the command becomes:

```
echo prod myapp sync
```

Now, replace `echo` with any command you like, and rewrite `args` that actually conforms to the command, so that you can integrate any command that does:

* templating
* linting
* testing

Hooks expose additional template expressions:

`.Event.Name` is the name of the hook event.

`.Event.Error` is the error generated by a failed release, exposed for `postsync` hooks only when a release fails, otherwise its value is `nil`.

You can use the hooks event expressions to send notifications to platforms such as `Slack`, `MS Teams`, etc.

The following example passes arguments to a script which sends a notification:

```yaml
releases:
- name: myapp
  chart: mychart
  # *snip*
  hooks:
  - events:
    - presync
    - postsync
    showlogs: true
    command: notify.sh
    args:
    - --event
    - '{{`{{ .Event.Name }}`}}'
    - --status
    - '{{`{{ if .Event.Error }}failure{{ else }}success{{ end }}`}}'
    - --environment
    - '{{`{{ .Environment.Name }}`}}'
    - --namespace
    - '{{`{{ .Release.Namespace }}`}}'
    - --release
    - '{{`{{ .Release.Name }}`}}'
```

For templating, imagine that you created a hook that generates a helm chart on-the-fly by running an external tool like ksonnet, kustomize, or your own template engine.
It will allow you to write your helm releases with any language you like, while still leveraging goodies provided by helm.

### Hooks, Kubectl and Environments

Hooks can also be used in combination with small tasks using `kubectl` directly,
e.g.: in order to install a custom storage class.

In the following example, a specific release depends on a custom storage class.
Further, all enviroments have a default kube context configured where releases are deployed into.
The `.Environment.KubeContext` is used in order to apply / remove the YAML to the correct context depending on the environment.

`environments.yaml`:

```yaml
environments:
  dev:
    values:
    - ../values/default.yaml
    - ../values/dev.yaml
    kubeContext: dev-cluster
  prod:
    values:
    - ../values/default.yaml
    - ../values/prod.yaml
    kubeContext: prod-cluster
```

`helmfile.yaml`:

```yaml
bases:
  - ./environments.yaml

---
releases:
  - name: myService
    namespace: my-ns
    installed: true
    chart: mychart
    version: "1.2.3"
    values:
      - ../services/my-service/values.yaml.gotmpl
    hooks:
      - events: ["presync"]
        showlogs: true
        command: "kubectl"
        args:
          - "apply"
          - "-f"
          - "./custom-storage-class.yaml"
          - "--context"
          - "{{`{{.Environment.KubeContext}}`}}"
      - events: ["postuninstall"]
        showlogs: true
        command: "kubectl"
        args:
          - "delete"
          - "-f"
          - "./custom-storage-class.yaml"
          - "--context"
          - "{{`{{.Environment.KubeContext}}`}}"
```

### Global Hooks

In contrast to the per release hooks mentioned above these are run only once at the very beginning and end of the execution of a helmfile command and only the `prepare` and `cleanup` hooks are available respectively.

They use the same syntax as per release hooks, but at the top level of your helmfile:

```yaml
hooks:
- events: ["prepare", "cleanup"]
  showlogs: true
  command: "echo"
  args: ["{{`{{.Environment.Name}}`}}", "{{`{{.HelmfileCommand}}`}}\
"]
```

### Helmfile + Kustomize

Do you prefer `kustomize` to write and organize your Kubernetes apps, but still want to leverage helm's useful features
like rollback, history, and so on? This section is for you!

The combination of `hooks` and [helmify-kustomize](https://gist.github.com/mumoshu/f9d0bd98e0eb77f636f79fc2fb130690)
enables you to integrate [kustomize](https://github.com/kubernetes-sigs/kustomize) into Helmfile.

That is, you can use `kustomize` to build a local helm chart from a kustomize overlay.

Let's assume you have a kustomize project named `foo-kustomize` like this:

```
foo-kustomize/
├── base
│   ├── configMap.yaml
│   ├── deployment.yaml
│   ├── kustomization.yaml
│   └── service.yaml
└── overlays
    ├── default
    │   ├── kustomization.yaml
    │   └── map.yaml
    ├── production
    │   ├── deployment.yaml
    │   └── kustomization.yaml
    └── staging
        ├── kustomization.yaml
        └── map.yaml

5 directories, 10 files
```

Write `helmfile.yaml`:

```yaml
- name: kustomize
  chart: ./foo
  hooks:
  - events: ["prepare", "cleanup"]
    command: "../helmify"
    args: ["{{`{{if eq .Event.Name \"prepare\"}}build{{else}}clean{{end}}`}}", "{{`{{.Release.Ch\
art}}`}}", "{{`{{.Environment.Name}}`}}"]
```

Run `helmfile --environment staging sync` and see it results in helmfile running `kustomize build foo-kustomize/overlays/staging > foo/templates/all.yaml`.

Voilà! You can mix helm releases that are backed by remote charts, local charts, and even kustomize overlays.

## Guides

Use the [Helmfile Best Practices Guide](writing-helmfile.md) to write advanced helmfiles that feature:

* Default values
* Layering

We also have dedicated documentation on the following topics which might interest you:

* [Shared Configurations Across Teams](shared-configuration-across-teams.md)

Or join our friendly slack community in the [`#helmfile`](https://slack.sweetops.com) channel to ask questions and get help. Check out our [slack archive](https://archive.sweetops.com/helmfile/) for good examples of how others are using it.

## Using .env files

Helmfile itself doesn't have an ability to load .env files. But you can write some bash script to achieve the goal:

```console
set -a; . .env; set +a; helmfile sync
```

Please see #203 for more context.

## Running Helmfile interactively

`helmfile --interactive [apply|destroy|delete|sync]` requests confirmation from you before actually modifying your cluster.

Use it when you're running `helmfile` manually on your local machine or a kind of secure administrative hosts.

For your local use-case, aliasing it like `alias hi='helmfile --interactive'` would be convenient.

Another way to use it is to set the environment variable `HELMFILE_INTERACTIVE=true` to enable the interactive mode by default.
Anything other than `true` will disable the interactive mode. The precedence has the `--interactive` flag.

## Running Helmfile without an Internet connection

Once you download all required charts into your machine, you can run `helmfile sync --skip-deps` to deploy your apps.
With the `--skip-deps` option, you can skip running "helm repo update" and "helm dependency build".

## Experimental Features

Some experimental features may be available for testing in perspective of being (or not) included in a future release.
Those features are set using the environment variable `HELMFILE_EXPERIMENTAL`. Here is the current experimental feature :

* `explicit-selector-inheritance` : remove today implicit cli selectors inheritance for composed helmfiles, see [composition selector](#selectors)

If you want to enable all experimental features set the env var to `HELMFILE_EXPERIMENTAL=true`

## `bash` and `zsh` completion

helmfile completion --help

## Examples

For more examples, see the [examples/README.md](https://github.com/helmfile/helmfile/blob/master/examples/README.md) or the [`helmfile`](https://github.com/cloudposse/helmfiles/tree/master/releases) distribution by [Cloud Posse](https://github.com/cloudposse/).

## Integrations

* [renovate](https://github.com/renovatebot/renovate) automates chart version updates. See [this PR for more information](https://github.com/renovatebot/renovate/pull/5257).
  * For updating container image tags and git tags embedded within helmfile.yaml and values, you can use [renovate's regexManager](https://docs.renovatebot.com/modules/manager/regex/). Please see [this comment in the renovate repository](https://github.com/renovatebot/renovate/issues/6130#issuecomment-624061289) for more information.
* [ArgoCD Integration](#argocd-integration)
* [Azure ACR Integration](#azure-acr-integration)

### ArgoCD Integration

Use [ArgoCD](https://argoproj.github.io/argo-cd/) with `helmfile template` for GitOps.

ArgoCD has support for kustomize/manifests/helm chart by itself. Why bother with Helmfile?

The reasons may vary:

1. You do want to manage applications with ArgoCD, while letting Helmfile manage infrastructure-related components like Calico/Cilium/WeaveNet, Linkerd/Istio, and ArgoCD itself.

* This way, any application deployed by ArgoCD has access to all the infrastructure.
* Of course, you can use ArgoCD's [Sync Waves and Phases](https://argoproj.github.io/argo-cd/user-guide/sync-waves/) for ordering the infrastructure and application installations. But it may be difficult to separate the concern between the infrastructure and apps and annotate K8s resources consistently when you have different teams for managing infra and apps.

2. You want to review the exact K8s manifests being applied on pull-request time, before ArgoCD syncs.

* This is often better than using a kind of `HelmRelease` custom resources that obfuscates exactly what manifests are being applied, which makes reviewing harder.

3. Use Helmfile as the single-pane of glass for all the K8s resources deployed to your cluster(s).

* Helmfile can reduce repetition in K8s manifests across ArgoCD application

For 1, you run `helmfile apply` on CI to deploy ArgoCD and the infrastructure components.

> helmfile config for this phase often reside within the same directory as your Terraform project. So connecting the two with [terraform-provider-helmfile](https://github.com/mumoshu/terraform-provider-helmfile) may be helpful

For 2, another app-centric CI or bot should render/commit manifests by running:

```
helmfile template --output-dir-template $(pwd)/gitops//{{.Release.Name}}
cd gitops
git add .
git commit -m 'some message'
git push origin $BRANCH
```

> Note that `$(pwd)` is necessary when `helmfile.yaml` has one or more sub-helmfiles in nested directories,
> because setting a relative file path in `--output-dir` or `--output-dir-template` results in each sub-helmfile render
> to the directory relative to the specified path.

so that they can be deployed by Argo CD as usual.

The CI or bot can optionally submit a PR to be review by human, running:

```
hub pull-request -b main -l gitops -m 'some description'
```

Recommendations:

* Do create ArgoCD `Application` custom resource per Helm/Helmfile release, each point to respective sub-directory generated by `helmfile template --output-dir-template`
* If you don't directly push it to the main Git branch and instead go through a pull-request, do lint rendered manifests on your CI, so that you can catch easy mistakes earlier/before ArgoCD finally deploys it
* See [this ArgoCD issue](https://github.com/argoproj/argo-cd/issues/2143#issuecomment-570478329) for why you may want this, and see [this helmfile issue](https://github.com/roboll/helmfile/pull/1357) for how `--output-dir-template` works.

### Azure ACR Integration

Azure offers helm repository [support for Azure Container Registry](https://docs.microsoft.com/en-us/azure/container-registry/container-registry-helm-repos) as a preview feature.

To use this you must first `az login` and then `az acr helm repo add -n <MyRegistry>`. This will extract a token for the given ACR and configure `helm` to use it, e.g. `helm repo update` should work straight away.

To use `helmfile` with ACR, on the other hand, you must either include a username/password in the repository definition for the ACR in your `helmfile.yaml` or use the `--skip-deps` switch, e.g. `helmfile template --skip-deps`.

An ACR repository definition in `helmfile.yaml` looks like this:

```yaml
repositories:
  - name: <MyRegistry>
    url: https://<MyRegistry>.azurecr.io/helm/v1/repo
```

## OCI Registries

In order to use OCI chart registries firstly they must be marked in the repository list as OCI enabled, e.g.

```yaml
repositories:
  - name: myOCIRegistry
    url: myregistry.azurecr.io
    oci: true
```

It is important not to include a scheme for the URL as helm requires that these are not present for OCI registries

Secondly the credentials for the OCI registry can either be specified within `helmfile.yaml` similar to

```yaml
repositories:
  - name: myOCIRegistry
    url: myregistry.azurecr.io
    oci: true
    username: spongebob
    password: squarepants
```

or for CI scenarios these can be sourced from the environment with the format `<registryName>_USERNAME` and `<registryName_PASSWORD>`, e.g.

```shell
export MYOCIREGISTRY_USERNAME=spongebob
export MYOCIREGISTRY_PASSWORD=squarepants
```

If `<registryName>` contains hyphens, the environment variable to be read is the hyphen replaced by an underscore., e.g.

```yaml
repositories:
  - name: my-oci-registry
    url: myregistry.azurecr.io
    oci: true
```

```shell
export MY_OCI_REGISTRY_USERNAME=spongebob
export MY_OCI_REGISTRY_PASSWORD=squarepants
```

## Attribution

We use:

* [semtag](https://github.com/pnikosis/semtag) for automated semver tagging. I greatly appreciate the author(pnikosis)'s effort on creating it and their kindness to share it!
