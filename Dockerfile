FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

RUN apk add --no-cache make git
WORKDIR /workspace/helmfile

COPY go.mod go.sum /workspace/helmfile/
RUN go mod download

COPY . /workspace/helmfile
ARG TARGETARCH TARGETOS
RUN make static-${TARGETOS}-${TARGETARCH}

# -----------------------------------------------------------------------------

FROM alpine:3.19

LABEL org.opencontainers.image.source=https://github.com/helmfile/helmfile

RUN apk add --no-cache ca-certificates git bash curl jq openssh-client gnupg

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

ARG HELM_VERSION="v3.17.3"
ENV HELM_VERSION="${HELM_VERSION}"
ARG HELM_LOCATION="https://get.helm.sh"
ARG HELM_FILENAME="helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "${HELM_LOCATION}/${HELM_FILENAME}" && \
    echo Verifying ${HELM_FILENAME}... && \
    case ${TARGETPLATFORM} in \
    "linux/amd64")  HELM_SHA256="ee88b3c851ae6466a3de507f7be73fe94d54cbf2987cbaa3d1a3832ea331f2cd"  ;; \
    "linux/arm64")  HELM_SHA256="7944e3defd386c76fd92d9e6fec5c2d65a323f6fadc19bfb5e704e3eee10348e"  ;; \
    esac && \
    echo "${HELM_SHA256}  ${HELM_FILENAME}" | sha256sum -c && \
    echo Extracting ${HELM_FILENAME}... && \
    tar xvf "${HELM_FILENAME}" -C /usr/local/bin --strip-components 1 ${TARGETOS}-${TARGETARCH}/helm && \
    rm "${HELM_FILENAME}" && \
    [ "$(helm version --template '{{.Version}}')" = "${HELM_VERSION}" ]

# using the install documentation found at https://kubernetes.io/docs/tasks/tools/install-kubectl/
# for now but in a future version of alpine (in the testing version at the time of writing)
# we should be able to install using apk add.
ENV KUBECTL_VERSION="v1.32.1"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl" && \
    case ${TARGETPLATFORM} in \
    "linux/amd64")  KUBECTL_SHA256="e16c80f1a9f94db31063477eb9e61a2e24c1a4eee09ba776b029048f5369db0c"  ;; \
    "linux/arm64")  KUBECTL_SHA256="98206fd83a4fd17f013f8c61c33d0ae8ec3a7c53ec59ef3d6a0a9400862dc5b2"  ;; \
    esac && \
    echo "${KUBECTL_SHA256}  kubectl" | sha256sum -c && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/kubectl && \
    [ "$(kubectl version -o json | jq -r '.clientVersion.gitVersion')" = "${KUBECTL_VERSION}" ]

ENV KUSTOMIZE_VERSION="v5.4.3"
ARG KUSTOMIZE_FILENAME="kustomize_${KUSTOMIZE_VERSION}_${TARGETOS}_${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/${KUSTOMIZE_FILENAME}" && \
    case ${TARGETPLATFORM} in \
    # Checksums are available at https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/checksums.txt
    "linux/amd64")  KUSTOMIZE_SHA256="3669470b454d865c8184d6bce78df05e977c9aea31c30df3c669317d43bcc7a7"  ;; \
    "linux/arm64")  KUSTOMIZE_SHA256="1b515578b0af12c15d9856720066ce2fe66756d63785b2cbccaf2885beb2381c"  ;; \
    esac && \
    echo "${KUSTOMIZE_SHA256}  ${KUSTOMIZE_FILENAME}" | sha256sum -c && \
    tar xvf "${KUSTOMIZE_FILENAME}" -C /usr/local/bin && \
    rm "${KUSTOMIZE_FILENAME}" && \
    [ "$(kustomize version)" = "${KUSTOMIZE_VERSION}" ]

ENV SOPS_VERSION="v3.9.3"
ARG SOPS_FILENAME="sops-${SOPS_VERSION}.${TARGETOS}.${TARGETARCH}"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/getsops/sops/releases/download/${SOPS_VERSION}/${SOPS_FILENAME}" && \
    chmod +x "${SOPS_FILENAME}" && \
    mv "${SOPS_FILENAME}" /usr/local/bin/sops && \
    sops --version --disable-version-check | grep -E "^sops ${SOPS_VERSION#v}"

ENV AGE_VERSION="v1.2.0"
ARG AGE_FILENAME="age-${AGE_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/FiloSottile/age/releases/download/${AGE_VERSION}/${AGE_FILENAME}" && \
    tar xvf "${AGE_FILENAME}" -C /usr/local/bin --strip-components 1 age/age age/age-keygen && \
    rm "${AGE_FILENAME}" && \
    [ "$(age --version)" = "${AGE_VERSION}" ] && \
    [ "$(age-keygen --version)" = "${AGE_VERSION}" ]

RUN helm plugin install https://github.com/databus23/helm-diff --version v3.11.0 && \
    helm plugin install https://github.com/jkroepke/helm-secrets --version v4.6.3 && \
    helm plugin install https://github.com/hypnoglow/helm-s3.git --version v0.16.3 && \
    helm plugin install https://github.com/aslafy-z/helm-git.git --version v1.3.0 && \
    rm -rf ${HELM_CACHE_HOME}/plugins

# Allow users other than root to use helm plugins located in root home
RUN chmod 751 ${HOME}

COPY --from=builder /workspace/helmfile/dist/helmfile_${TARGETOS}_${TARGETARCH} /usr/local/bin/helmfile

CMD ["/usr/local/bin/helmfile"]
