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

声明式Helm Chart管理工具
<br />

</div>

[English](./README.md) | [简体中文]

# 关于

Helmfile 是一个声明式Helm Chart管理工具

> Helmfile 项目已经从原仓库 roboll/helmfile 转移到了 helmfile/helmfile。有关更多信息，请参见 roboll/helmfile#1824

## 特性

- 通过一个YAML集中管理集群中多个Helm Chart， 类似于Docker Compose统一管理Docker
- 对Helm Chart根据部署环境区分管理
- Helm Chart版本控制，比如指定版本范围、锁定某一版本
- 快速识别 Kubernetes 集群内已经部署应用与新更改之间的差异
- Helmfile支持Go Templates语法定义Helm Chart
- 在部署阶段支持配置hook，可以执行脚本等，实现变量远程获取，报错清理，成功提醒等


## 安装

**方式1: 二进制安装**

下载 [releases](https://github.com/helmfile/helmfile/releases)

**方式2: 包管理工具**

* Archlinux: `pacman -S helmfile`
* openSUSE: `zypper in helmfile`
* Windows: ([scoop](https://scoop.sh/)): `scoop install helmfile`
* macOS ([homebrew](https://brew.sh/)): `brew install helmfile`

**方式3: 容器**

详细见：[run as a container](https://helmfile.readthedocs.io/en/latest/#running-as-a-container)

> 安装后请运行一次 `helmfile init`。 检查[helm-diff](https://github.com/databus23/helm-diff) 等插件安装正确。

## 使用

让我们从最简单的 helmfile 开始，逐渐改进它以适应您的用例！

假设表示您 helm releases 的期望状态的 helmfile.yaml 看起来像这样：

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

通过运行来同步您的Kubernetes集群状态到期望状态:

```console
helmfile apply
```

恭喜！您现在已经在集群内部运行了第一个Prometheus部署。


## 文档

[Documentation](https://helmfile.readthedocs.io/)


## 参与贡献

欢迎贡献！ 让我们一起使helmfile变得更好：[贡献指南](https://helmfile.readthedocs.io/en/latest/contributing/)


## 使用者

Helmfile 已经被许多用户在生产环境中使用:

* [gitlab.com](https://gitlab.com)
* [reddit.com](https://reddit.com)
* [Jenkins](https://jenkins.io)
* ...

更多用户请参见: [Users](https://helmfile.readthedocs.io/en/latest/users/)


## License

[MIT](https://github.com/helmfile/helmfile/blob/main/LICENSE)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=helmfile/helmfile&type=Date)](https://star-history.com/#helmfile/helmfile&Date)
