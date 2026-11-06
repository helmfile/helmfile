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

部署 Kubernetes Helm Chart
<br />

</div>

[English](./README.md) | [简体中文](./README-zh_CN.md) | 繁體中文

## 關於

Helmfile 以宣告式規格部署 Helm Chart，讓你可以：

* 把 Chart 的 values 檔集中在一個目錄，並用版本控制管理變更。
* 為設定變更套用 CI/CD。
* 定期同步，避免各環境之間產生差異。

`helmfile` 執行檔把工作交給 `helm` 處理，這樣 `helm` 每次改版時就不必跟著升級。因此必須先安裝：
- [helm](https://helm.sh/docs/intro/install/)
- [helm-diff](https://github.com/databus23/helm-diff)

## 特色

**宣告式**：撰寫期望狀態檔、納入版本控制後套用，讓狀態一目瞭然，結果也能重現。

**模組**：把基礎架構的常見模式模組化，透過 Git、S3 等方式發佈，讓全公司重複使用（請見 [#648](https://github.com/roboll/helmfile/pull/648)）

**多樣性**：叢集可由 Chart、[kustomization](https://github.com/kubernetes-sigs/kustomize) 與 Kubernetes 資源目錄組成，全部都轉換成 Helm release 管理（請見 [#673](https://github.com/roboll/helmfile/pull/673)）

**修補**：在 `helm-install` 之前，以 JSON Patch 或 Strategic Merge Patch 修改 Kubernetes 資源，不必 fork 上游 Chart（請見 [#673](https://github.com/roboll/helmfile/pull/673)）

## 專案狀態

2025 年 5 月更新

* Helmfile v1.0 與 v1.1 已經發佈。如果你還在使用 v0.x，建議直接升級到 v1.1。
* 如果還沒有升級，請先看過這份 [v1 提案](docs/proposals/towards-1.0.md)，裡面列出了少數幾項不相容的變更。

## 安裝

**1：下載執行檔**

下載任一版 [releases](https://github.com/helmfile/helmfile/releases)

**2：套件管理工具**

* Arch Linux：以 `pacman -S helmfile` 安裝
* openSUSE：使用 Tumbleweed 時以 `zypper in helmfile` 安裝；使用 Leap 時，要先為你的發行版本加入一次 [kubic](https://download.opensuse.org/repositories/devel:/kubic/) 套件庫，再執行上述指令，例如 `zypper ar https://download.opensuse.org/repositories/devel:/kubic/openSUSE_Leap_\$releasever kubic`
* Windows（使用 [scoop](https://scoop.sh/)）：`scoop install helmfile`
* macOS（使用 [homebrew](https://brew.sh/)）：`brew install helmfile`
* Linux／macOS／Windows（使用 [mise](https://mise.jdx.dev)）：`mise use -g helmfile@latest`

**3：容器**

詳情請見 [以容器執行](https://helmfile.readthedocs.io/en/latest/#running-as-a-container)

> 安裝後請執行一次 `helmfile init`。Helmfile 會用到 [helm-diff](https://github.com/databus23/helm-diff) 外掛。

**4：從原始碼建置**

需求：[Go](https://golang.org/dl/)



` go install github.com/helmfile/helmfile@latest `
## 開始使用

從一個簡單的 `helmfile` 開始，再逐步調整成符合你需求的樣子！

產生一個採用最佳實務目錄結構的專案骨架：

```console
helmfile create my-project && cd my-project
```

也可以手動建立 `helmfile.yaml`。假設用來描述 Helm release 期望狀態的 `helmfile.yaml` 長這樣：

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

執行下面的指令，把 Kubernetes 叢集同步到期望狀態：

```console
helmfile apply
```

恭喜！叢集裡已經跑起你的第一個 Prometheus 部署。

接著可以參考下列文件，繼續調整 `helmfile.yaml`：

* [設定](https://helmfile.readthedocs.io/en/latest/#configuration)
* [CLI 參考](https://helmfile.readthedocs.io/en/latest/#cli-reference)
* [Helmfile 最佳實務指南](https://helmfile.readthedocs.io/en/latest/writing-helmfile/)

## 更多範例

請見：[multi-env-helmfile](https://github.com/helmfile/multi-env-helmfile)

## 文件

請閱讀[完整文件](https://helmfile.readthedocs.io/)

## 參與貢獻

歡迎一起讓 Helmfile 變得更好：[貢獻指南](https://helmfile.readthedocs.io/en/latest/contributing/)

## 致謝

我們使用：

* [semtag](https://github.com/pnikosis/semtag) 自動標記語意化版本。
非常感謝作者（pnikosis）製作這項工具，並慷慨分享！

## 使用者

許多使用者已在正式環境中使用 Helmfile：

* [gitlab.com](https://gitlab.com)
* [reddit.com](https://reddit.com)
* [Jenkins](https://jenkins.io)
* ...

更多使用者請見：[Users](https://helmfile.readthedocs.io/en/latest/users/)

## 授權

[MIT](https://github.com/helmfile/helmfile/blob/main/LICENSE)

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=helmfile/helmfile&type=Date)](https://star-history.dera.page/#helmfile/helmfile&Date)
