FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

RUN apk add --no-cache make git
WORKDIR /workspace/helmfile

COPY go.mod go.sum /workspace/helmfile/
RUN go mod download

COPY . /workspace/helmfile
ARG TARGETARCH TARGETOS
RUN make static-${TARGETOS}-${TARGETARCH}

# -----------------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS kustomize-builder

RUN apk add --no-cache git
ARG TARGETARCH TARGETOS
ENV KUSTOMIZE_VERSION="v5.8.0"
RUN set -x && \
    git clone --branch kustomize/${KUSTOMIZE_VERSION} --depth 1 https://github.com/kubernetes-sigs/kustomize.git /workspace/kustomize
WORKDIR /workspace/kustomize/kustomize
RUN GOFLAGS=-mod=readonly GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X sigs.k8s.io/kustomize/api/provenance.version=kustomize/${KUSTOMIZE_VERSION}" -o /out/kustomize .

# -----------------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS sops-builder

RUN apk add --no-cache git
ARG TARGETARCH TARGETOS
ENV SOPS_VERSION="v3.11.0"
RUN set -x && \
    git clone --branch ${SOPS_VERSION} --depth 1 https://github.com/getsops/sops.git /workspace/sops
WORKDIR /workspace/sops
RUN GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -mod=readonly -ldflags="-s -w -X github.com/getsops/sops/v3/version.Version=${SOPS_VERSION#v}" -o /out/sops ./cmd/sops

# -----------------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS age-builder

RUN apk add --no-cache git
ARG TARGETARCH TARGETOS
ENV AGE_VERSION="v1.3.1"
RUN set -x && \
    git clone --branch ${AGE_VERSION} --depth 1 https://github.com/FiloSottile/age.git /workspace/age
WORKDIR /workspace/age
RUN GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-X main.Version=${AGE_VERSION}" -o /out/age ./cmd/age && \
    GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-X main.Version=${AGE_VERSION}" -o /out/age-keygen ./cmd/age-keygen

# -----------------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS kubectl-builder

RUN apk add --no-cache git bash rsync
ARG TARGETARCH TARGETOS
ENV KUBECTL_VERSION="v1.34.3"
RUN set -x && \
    git clone --branch ${KUBECTL_VERSION} --depth 1 https://github.com/kubernetes/kubernetes.git /workspace/kubernetes
WORKDIR /workspace/kubernetes
RUN GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X k8s.io/component-base/version.gitVersion=${KUBECTL_VERSION}" -o /out/kubectl ./cmd/kubectl

# -----------------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS helm-diff-builder

RUN apk add --no-cache git
ARG TARGETARCH TARGETOS
ENV HELM_DIFF_VERSION="v3.15.0"
RUN set -x && \
    git clone --branch ${HELM_DIFF_VERSION} --depth 1 https://github.com/databus23/helm-diff.git /workspace/helm-diff
WORKDIR /workspace/helm-diff
RUN GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X github.com/databus23/helm-diff/v3/cmd.Version=${HELM_DIFF_VERSION}" -o /out/diff .

# -----------------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS helm-s3-builder

RUN apk add --no-cache git
ARG TARGETARCH TARGETOS
ENV HELM_S3_VERSION="v0.17.1"
RUN set -x && \
    git clone --branch ${HELM_S3_VERSION} --depth 1 https://github.com/hypnoglow/helm-s3.git /workspace/helm-s3
WORKDIR /workspace/helm-s3
RUN GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${HELM_S3_VERSION}" -o /out/helm-s3 ./cmd/helm-s3

# -----------------------------------------------------------------------------

FROM alpine:3.22

LABEL org.opencontainers.image.source=https://github.com/helmfile/helmfile

RUN apk add --no-cache ca-certificates git bash curl jq yq openssh-client gnupg

ARG TARGETARCH TARGETOS TARGETPLATFORM

# Set Helm home variables so that also non-root users can use plugins etc.
ARG HOME="/helm"
ENV HOME="${HOME}"
ARG HELM_CACHE_HOME="${HOME}/.cache/helm"
ENV HELM_CACHE_HOME="${HELM_CACHE_HOME}"
ARG HELM_CONFIG_HOME="${HOME}/.config/helm"
ENV HELM_CONFIG_HOME="${HELM_CONFIG_HOME}"
ARG HELM_DATA_HOME="${HOME}/.local/share/helm"
ENV HELM_DATA_HOME="${HELM_DATA_HOME}"

ARG HELM_VERSION="v4.1.0"
ENV HELM_VERSION="${HELM_VERSION}"
ENV HELM_BIN="/usr/local/bin/helm"
ARG HELM_LOCATION="https://get.helm.sh"
ARG HELM_FILENAME="helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "${HELM_LOCATION}/${HELM_FILENAME}" && \
    echo Verifying ${HELM_FILENAME}... && \
    case ${TARGETPLATFORM} in \
    "linux/amd64")  HELM_SHA256="8e7ae5cb890c56f53713bffec38e41cd8e7e4619ebe56f8b31cd383bfb3dbb83"  ;; \
    "linux/arm64")  HELM_SHA256="81315e404b6d09b65bee577a679ab269d6d44652ef2e1f66a8f922b51ca93f6b"  ;; \
    esac && \
    echo "${HELM_SHA256}  ${HELM_FILENAME}" | sha256sum -c && \
    echo Extracting ${HELM_FILENAME}... && \
    tar xvf "${HELM_FILENAME}" -C /usr/local/bin --strip-components 1 ${TARGETOS}-${TARGETARCH}/helm && \
    rm "${HELM_FILENAME}" && \
    [ "$(helm version --template '{{.Version}}')" = "${HELM_VERSION}" ]

COPY --from=kubectl-builder /out/kubectl /usr/local/bin/kubectl

COPY --from=kustomize-builder /out/kustomize /usr/local/bin/kustomize

COPY --from=sops-builder /out/sops /usr/local/bin/sops

COPY --from=age-builder /out/age /usr/local/bin/age
COPY --from=age-builder /out/age-keygen /usr/local/bin/age-keygen

ARG HELM_SECRETS_VERSION="4.7.4"
RUN helm plugin install https://github.com/databus23/helm-diff --version v3.15.0 --verify=false && \
    helm plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/secrets-${HELM_SECRETS_VERSION}.tgz --verify=false && \
    helm plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/secrets-getter-${HELM_SECRETS_VERSION}.tgz --verify=false && \
    helm plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/secrets-post-renderer-${HELM_SECRETS_VERSION}.tgz --verify=false && \
    helm plugin install https://github.com/hypnoglow/helm-s3.git --version v0.17.1 --verify=false && \
    helm plugin install https://github.com/aslafy-z/helm-git.git --version v1.4.1 --verify=false && \
    rm -rf ${HELM_CACHE_HOME}/plugins

COPY --from=helm-diff-builder /out/diff ${HELM_DATA_HOME}/plugins/helm-diff/bin/diff
COPY --from=helm-s3-builder /out/helm-s3 ${HELM_DATA_HOME}/plugins/helm-s3.git/bin/helm-s3

# Allow users other than root to use helm plugins located in root home
RUN chmod 751 ${HOME}

COPY --from=builder /workspace/helmfile/dist/helmfile_${TARGETOS}_${TARGETARCH} /usr/local/bin/helmfile

CMD ["/usr/local/bin/helmfile"]
