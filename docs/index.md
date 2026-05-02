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

May 2025 Update

* Helmfile v1.0 and v1.1 has been released. We recommend upgrading directly to v1.1 if you are still using v0.x.
* If you haven't already upgraded, please go over this v1 proposal [here](https://github.com/helmfile/helmfile/blob/main/docs/proposals/towards-1.0.md) to see a small list of breaking changes.

## About

Helmfile is a declarative spec for deploying helm charts. It lets you...

* Keep a directory of chart value files and maintain changes in version control.
* Apply CI/CD to configuration changes.
* Periodically sync to avoid skew in environments.

To avoid upgrades for each iteration of `helm`, the `helmfile` executable delegates to `helm` - as a result, `helm` must be installed.

**NOTE**: Helmfile supports both Helm 3.x and Helm 4.x.

## Highlights

**Declarative**: Write, version-control, apply the desired state file for visibility and reproducibility.

**Modules**: Modularize common patterns of your infrastructure, distribute it via Git, S3, etc. to be reused across the entire company (See [#648](https://github.com/roboll/helmfile/pull/648))

**Versatility**: Manage your cluster consisting of charts, [kustomizations](https://github.com/kubernetes-sigs/kustomize), and directories of Kubernetes resources, turning everything to Helm releases (See [#673](https://github.com/roboll/helmfile/pull/673))

**Patch**: JSON/Strategic-Merge Patch Kubernetes resources before `helm-install`ing, without forking upstream charts (See [#673](https://github.com/roboll/helmfile/pull/673))

## Installation

* download one of [releases](https://github.com/helmfile/helmfile/releases)
* [run as a container](#running-as-a-container)
* Archlinux: install via `pacman -S helmfile`
* openSUSE: install via `zypper in helmfile` assuming you are on Tumbleweed; if you are on Leap you must add the [kubic](https://download.opensuse.org/repositories/devel:/kubic/) repo for your distribution version once before that command, e.g. `zypper ar https://download.opensuse.org/repositories/devel:/kubic/openSUSE_Leap_\$releasever kubic`
* Windows (using [scoop](https://scoop.sh/)): `scoop install helmfile`
* macOS (using [homebrew](https://brew.sh/)): `brew install helmfile`

### Running as a container

The [Helmfile Docker images are available in GHCR](https://github.com/helmfile/helmfile/pkgs/container/helmfile). Make sure you pick the right tag for your version. Example:

```sh-session
$ docker run --rm --net=host -v "${HOME}/.kube:/helm/.kube" -v "${HOME}/.config/helm:/helm/.config/helm" -v "${PWD}:/wd" --workdir /wd ghcr.io/helmfile/helmfile:v1.1.0 helmfile sync
```

You can also use a shim to make calling the binary easier:

```sh-session
$ printf '%s\n' '#!/bin/sh' 'docker run --rm --net=host -v "${HOME}/.kube:/helm/.kube" -v "${HOME}/.config/helm:/helm/.config/helm" -v "${PWD}:/wd" --workdir /wd ghcr.io/helmfile/helmfile:v1.1.0 helmfile "$@"' |
    tee helmfile
$ chmod +x helmfile
$ ./helmfile sync
```

## Getting Started

### Prerequisites

* A Kubernetes cluster (e.g., [minikube](https://minikube.sigs.k8s.io/), [kind](https://kind.sigs.k8s.io/), or a cloud provider)
* [Helm 3+](https://helm.sh/docs/intro/install/) installed (`helm version` to verify)

### Step 1: Install Helmfile

Choose one of the following:

```bash
# macOS
brew install helmfile

# Linux - download from GitHub releases
curl -L https://github.com/helmfile/helmfile/releases/latest/download/helmfile_$(uname -s)_$(uname -m) -o /usr/local/bin/helmfile && chmod +x /usr/local/bin/helmfile

# Windows
scoop install helmfile
```

Verify: `helmfile version`

### Step 2: Create your first helmfile.yaml

Create a file named `helmfile.yaml`:

```yaml
repositories:
  - name: prometheus-community
    url: https://prometheus-community.github.io/helm-charts

releases:
  - name: my-prometheus
    namespace: monitoring
    createNamespace: true
    chart: prometheus-community/prometheus
    values:
      - server:
          persistentVolume:
            enabled: false
```

**What does this do?**
* `repositories` — tells Helm where to find charts (like a package registry)
* `releases` — each entry is a Helm release to deploy
  * `name` — a unique name for this deployment
  * `namespace` — which Kubernetes namespace to deploy into
  * `chart` — which Helm chart to use (`repository-name/chart-name`)
  * `values` — customize the chart's default settings

### Step 3: Initialize and deploy

```bash
# Initialize - checks helm and installs required plugins
helmfile init

# See what would be deployed (dry-run)
helmfile diff

# Deploy to your cluster
helmfile apply
```

Congratulations! You now have Prometheus running in your cluster.

### Step 4: Make changes and re-apply

Edit `helmfile.yaml` to add another release:

```yaml
repositories:
  - name: prometheus-community
    url: https://prometheus-community.github.io/helm-charts

releases:
  - name: my-prometheus
    namespace: monitoring
    createNamespace: true
    chart: prometheus-community/prometheus
    values:
      - server:
          persistentVolume:
            enabled: false

  - name: my-grafana
    namespace: monitoring
    chart: grafana/grafana
```

Run `helmfile apply` again — Helmfile will detect the new release and only deploy what changed.

### Step 5: Clean up

```bash
# Remove everything
helmfile destroy
```

### Next Steps

Now that you have the basics, explore these topics:

**Core concepts** (read in order):
1. [Writing Helmfile](writing-helmfile.md) — patterns and best practices for structuring helmfiles
2. [Values and Merging](values-and-merging.md) — how Helmfile merges values from multiple sources
3. [Environments](environments.md) — manage dev/staging/production with a single helmfile

**Reference material** (look up as needed):
* [Configuration Reference](configuration.md) — complete `helmfile.yaml` schema
* [CLI Reference](cli.md) — all commands and flags
* [Template Functions](templating_funcs.md) — functions available in Go templates

## Labels Overview

A selector can be used to only target a subset of releases when running Helmfile. This is useful for large helmfiles with releases that are logically grouped together.

Labels are simple key value pairs that are an optional field of the release spec. When selecting by label, the search can be inverted. `tier!=backend` would match all releases that do NOT have the `tier: backend` label. `tier=frontend` would only match releases with the `tier: frontend` label.

Multiple labels can be specified using `,` as a separator. A release must match all selectors in order to be selected for the final helm command.

The `selector` parameter can be specified multiple times. Each parameter is resolved independently so a release that matches any parameter will be used.

`--selector tier=frontend --selector tier=backend` will select all the charts.

In addition to user supplied labels, the name, the namespace, and the chart are available to be used as selectors.  The chart will just be the chart name excluding the repository (Example `stable/filebeat` would be selected using `--selector chart=filebeat`).

`commonLabels` can be used when you want to apply the same label to all releases and use [templating](templating.md) based on that.
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

## Advanced

* [Advanced Features](advanced-features.md) - Kubedog, Kustomize, chartify, vals integration
* [Hooks](hooks.md) - Lifecycle hooks (prepare, presync, postsync, cleanup)
* [Secrets](remote-secrets.md) - Remote secrets (vault, SSM, etc.)
* [Shared Configuration Across Teams](shared-configuration-across-teams.md) - Multi-team Helmfile patterns

## Community

Join our friendly slack community in the [`#helmfile`](https://slack.sweetops.com) channel to ask questions and get help. Check out our [slack archive](https://archive.sweetops.com/helmfile/) for good examples of how others are using it.

## Experimental Features

Some experimental features may be available for testing in perspective of being (or not) included in a future release.
Those features are set using the environment variable `HELMFILE_EXPERIMENTAL`. Here is the current experimental feature :

* `explicit-selector-inheritance` : remove today implicit cli selectors inheritance for composed helmfiles, see [composition selector](releases.md#selectors)

If you want to enable all experimental features set the env var to `HELMFILE_EXPERIMENTAL=true`

## Examples

For more examples, see the [examples/README.md](https://github.com/helmfile/helmfile/blob/master/examples/README.md) or the [`helmfile`](https://github.com/cloudposse/helmfiles/tree/master/releases) distribution by [Cloud Posse](https://github.com/cloudposse/).

