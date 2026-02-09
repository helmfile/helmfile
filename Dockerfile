FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

RUN apk add --no-cache make git
WORKDIR /workspace/helmfile

COPY go.mod go.sum /workspace/helmfile/
RUN go mod download

COPY . /workspace/helmfile
ARG TARGETARCH TARGETOS
RUN make static-${TARGETOS}-${TARGETARCH}

# -----------------------------------------------------------------------------
# Build kustomize from source (pre-built v5.8.0 uses Go 1.24.0)

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS kustomize-builder

RUN apk add --no-cache git
ARG TARGETARCH TARGETOS
ENV KUSTOMIZE_VERSION="v5.8.0"
RUN set -x && \
    git clone --branch kustomize/${KUSTOMIZE_VERSION} --depth 1 https://github.com/kubernetes-sigs/kustomize.git /workspace/kustomize && \
    cd /workspace/kustomize/kustomize && \
    GOFLAGS=-mod=readonly GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X sigs.k8s.io/kustomize/api/provenance.version=kustomize/${KUSTOMIZE_VERSION}" -o /out/kustomize . && \
    rm -rf /workspace/kustomize /root/.cache/go-build /go/pkg/mod

# -----------------------------------------------------------------------------
# Build kubectl from source (pre-built v1.34.3 uses Go 1.24.11)

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS kubectl-builder

RUN apk add --no-cache git bash rsync
ARG TARGETARCH TARGETOS
ENV KUBECTL_VERSION="v1.34.3"
RUN set -x && \
    git clone --branch ${KUBECTL_VERSION} --depth 1 https://github.com/kubernetes/kubernetes.git /workspace/kubernetes && \
    cd /workspace/kubernetes && \
    GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X k8s.io/component-base/version.gitVersion=${KUBECTL_VERSION}" -o /out/kubectl ./cmd/kubectl && \
    rm -rf /workspace/kubernetes /root/.cache/go-build /go/pkg/mod

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

ENV SOPS_VERSION="v3.11.0"
ARG SOPS_FILENAME="sops-${SOPS_VERSION}.${TARGETOS}.${TARGETARCH}"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/getsops/sops/releases/download/${SOPS_VERSION}/${SOPS_FILENAME}" && \
    chmod +x "${SOPS_FILENAME}" && \
    mv "${SOPS_FILENAME}" /usr/local/bin/sops && \
    sops --version --disable-version-check | grep -E "^sops ${SOPS_VERSION#v}"

ENV AGE_VERSION="v1.3.1"
ARG AGE_FILENAME="age-${AGE_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/FiloSottile/age/releases/download/${AGE_VERSION}/${AGE_FILENAME}" && \
    tar xvf "${AGE_FILENAME}" -C /usr/local/bin --strip-components 1 age/age age/age-keygen && \
    rm "${AGE_FILENAME}" && \
    [ "$(age --version)" = "${AGE_VERSION}" ] && \
    [ "$(age-keygen --version)" = "${AGE_VERSION}" ]

ARG HELM_SECRETS_VERSION="4.7.4"
RUN helm plugin install https://github.com/databus23/helm-diff --version v3.15.0 --verify=false && \
    helm plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/secrets-${HELM_SECRETS_VERSION}.tgz --verify=false && \
    helm plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/secrets-getter-${HELM_SECRETS_VERSION}.tgz --verify=false && \
    helm plugin install https://github.com/jkroepke/helm-secrets/releases/download/v${HELM_SECRETS_VERSION}/secrets-post-renderer-${HELM_SECRETS_VERSION}.tgz --verify=false && \
    helm plugin install https://github.com/hypnoglow/helm-s3.git --version v0.17.1 --verify=false && \
    helm plugin install https://github.com/aslafy-z/helm-git.git --version v1.4.1 --verify=false && \
    rm -rf ${HELM_CACHE_HOME}/plugins

# Allow users other than root to use helm plugins located in root home
RUN chmod 751 ${HOME}

COPY --from=builder /workspace/helmfile/dist/helmfile_${TARGETOS}_${TARGETARCH} /usr/local/bin/helmfile

CMD ["/usr/local/bin/helmfile"]
