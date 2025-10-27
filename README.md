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
[![Gurubase](https://img.shields.io/badge/Gurubase-Ask%20Helmfile%20Guru-006BFF)](https://gurubase.io/g/helmfile)
[![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/helmfile/helmfile)

Deploy Kubernetes Helm Charts
<br />

</div>

English | [简体中文](./README-zh_CN.md)

## About

Helmfile is a declarative spec for deploying helm charts. It lets you...

* Keep a directory of chart value files and maintain changes in version control.
* Apply CI/CD to configuration changes.
* Periodically sync to avoid skew in environments.

To avoid upgrades for each iteration of `helm`, the `helmfile` executable delegates to `helm` - as a result, the following must be installed
- [helm](https://helm.sh/docs/intro/install/)
- [helm-diff](https://github.com/databus23/helm-diff)

## Highlights

**Declarative**: Write, version-control, apply the desired state file for visibility and reproducibility.

**Modules**: Modularize common patterns of your infrastructure, distribute it via Git, S3, etc. to be reused across the entire company (See [#648](https://github.com/roboll/helmfile/pull/648))

**Versatility**: Manage your cluster consisting of charts, [kustomizations](https://github.com/kubernetes-sigs/kustomize), and directories of Kubernetes resources, turning everything to Helm releases (See [#673](https://github.com/roboll/helmfile/pull/673))

**Patch**: JSON/Strategic-Merge Patch Kubernetes resources before `helm-install`ing, without forking upstream charts (See [#673](https://github.com/roboll/helmfile/pull/673))

## Status

May 2025 Update

* Helmfile v1.0 and v1.1 has been released. We recommend upgrading directly to v1.1 if you are still using v0.x.
* If you haven't already upgraded, please go over this v1 proposal [here](docs/proposals/towards-1.0.md) to see a small list of breaking changes.

## Installation

**1: Binary Installation**

download one of [releases](https://github.com/helmfile/helmfile/releases)

**2: Package Manager**

* Archlinux: install via `pacman -S helmfile`
* openSUSE: install via `zypper in helmfile` assuming you are on Tumbleweed; if you are on Leap you must add the [kubic](https://download.opensuse.org/repositories/devel:/kubic/) repo for your distribution version once before that command, e.g. `zypper ar https://download.opensuse.org/repositories/devel:/kubic/openSUSE_Leap_\$releasever kubic`
* Windows (using [scoop](https://scoop.sh/)): `scoop install helmfile`
* macOS (using [homebrew](https://brew.sh/)): `brew install helmfile`

**3: Container**

For more details, see [run as a container](https://helmfile.readthedocs.io/en/latest/#running-as-a-container)

> Make sure to run `helmfile init` once after installation. Helmfile uses the [helm-diff](https://github.com/databus23/helm-diff) plugin.

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

Sync your Kubernetes cluster state to the desired one by running:

```console
helmfile apply
```

Congratulations! You now have your first Prometheus deployment running inside
 your cluster.

Iterate on the `helmfile.yaml` by referencing:

* [Configuration](https://helmfile.readthedocs.io/en/latest/#configuration)
* [CLI reference](https://helmfile.readthedocs.io/en/latest/#cli-reference)
* [Helmfile Best Practices Guide](https://helmfile.readthedocs.io/en/latest/writing-helmfile/)

## More complex examples

See: [multi-env-helmfile](https://github.com/helmfile/multi-env-helmfile)

## Docs

Please read [complete documentation](https://helmfile.readthedocs.io/)

## Contributing

Welcome to contribute together to make helmfile better: [contributing doc](https://helmfile.readthedocs.io/en/latest/contributing/)

## Attribution

We use:

* [semtag](https://github.com/pnikosis/semtag) for automated semver tagging.
I greatly appreciate the author(pnikosis)'s effort on creating it and their
kindness to share it!

## Users

Helmfile has been used by many users in production:

* [gitlab.com](https://gitlab.com)
* [reddit.com](https://reddit.com)
* [Jenkins](https://jenkins.io)
* ...

For more users, please see: [Users](https://helmfile.readthedocs.io/en/latest/users/)

## License

[MIT](https://github.com/helmfile/helmfile/blob/main/LICENSE)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=helmfile/helmfile&type=Date)](https://star-history.com/#helmfile/helmfile&Date)
