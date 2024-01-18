FROM --platform=$BUILDPLATFORM golang:1.21-alpine as builder

RUN apk add --no-cache make git
WORKDIR /workspace/helmfile

COPY go.mod go.sum /workspace/helmfile/
RUN go mod download

COPY . /workspace/helmfile
ARG TARGETARCH TARGETOS
RUN make static-${TARGETOS}-${TARGETARCH}

# -----------------------------------------------------------------------------

FROM alpine:3.16

LABEL org.opencontainers.image.source https://github.com/helmfile/helmfile

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

ARG HELM_VERSION="v3.14.0"
ENV HELM_VERSION="${HELM_VERSION}"
ARG HELM_LOCATION="https://get.helm.sh"
ARG HELM_FILENAME="helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "${HELM_LOCATION}/${HELM_FILENAME}" && \
    echo Verifying ${HELM_FILENAME}... && \
    case ${TARGETPLATFORM} in \
        "linux/amd64")  HELM_SHA256="f43e1c3387de24547506ab05d24e5309c0ce0b228c23bd8aa64e9ec4b8206651"  ;; \
        "linux/arm64")  HELM_SHA256="b29e61674731b15f6ad3d1a3118a99d3cc2ab25a911aad1b8ac8c72d5a9d2952"  ;; \
    esac && \
    echo "${HELM_SHA256}  ${HELM_FILENAME}" | sha256sum -c && \
    echo Extracting ${HELM_FILENAME}... && \
    tar xvf "${HELM_FILENAME}" -C /usr/local/bin --strip-components 1 ${TARGETOS}-${TARGETARCH}/helm && \
    rm "${HELM_FILENAME}" && \
    [ "$(helm version --template '{{.Version}}')" = "${HELM_VERSION}" ]

# using the install documentation found at https://kubernetes.io/docs/tasks/tools/install-kubectl/
# for now but in a future version of alpine (in the testing version at the time of writing)
# we should be able to install using apk add.
ENV KUBECTL_VERSION="v1.25.16"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl" && \
    case ${TARGETPLATFORM} in \
        # checksums are available at https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl.sha256
        "linux/amd64")  KUBECTL_SHA256="5a9bc1d3ebfc7f6f812042d5f97b82730f2bdda47634b67bddf36ed23819ab17"  ;; \
        "linux/arm64")  KUBECTL_SHA256="d6c23c80828092f028476743638a091f2f5e8141273d5228bf06c6671ef46924"  ;; \
    esac && \
    echo "${KUBECTL_SHA256}  kubectl" | sha256sum -c && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/kubectl && \
    [ "$(kubectl version -o json | jq -r '.clientVersion.gitVersion')" = "${KUBECTL_VERSION}" ]

ENV KUSTOMIZE_VERSION="v5.2.1"
ARG KUSTOMIZE_FILENAME="kustomize_${KUSTOMIZE_VERSION}_${TARGETOS}_${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/${KUSTOMIZE_FILENAME}" && \
    case ${TARGETPLATFORM} in \
        # checksim are available at https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/checksums.txt
        "linux/amd64")  KUSTOMIZE_SHA256="88346543206b889f9287c0b92c70708040ecd5aad54dd33019c4d6579cd24de8"  ;; \
        "linux/arm64")  KUSTOMIZE_SHA256="5566f7badece5a72d42075d8dffa6296a228966dd6ac2390de7afbb9675c3aaa"  ;; \
    esac && \
    echo "${KUSTOMIZE_SHA256}  ${KUSTOMIZE_FILENAME}" | sha256sum -c && \
    tar xvf "${KUSTOMIZE_FILENAME}" -C /usr/local/bin && \
    rm "${KUSTOMIZE_FILENAME}" && \
    [ "$(kustomize version)" = "${KUSTOMIZE_VERSION}" ]

ENV SOPS_VERSION="v3.8.1"
ARG SOPS_FILENAME="sops-${SOPS_VERSION}.${TARGETOS}.${TARGETARCH}"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/getsops/sops/releases/download/${SOPS_VERSION}/${SOPS_FILENAME}" && \
    chmod +x "${SOPS_FILENAME}" && \
    mv "${SOPS_FILENAME}" /usr/local/bin/sops && \
    sops --version | grep -E "^sops ${SOPS_VERSION#v}"

ENV AGE_VERSION="v1.1.1"
ARG AGE_FILENAME="age-${AGE_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/FiloSottile/age/releases/download/${AGE_VERSION}/${AGE_FILENAME}" && \
    tar xvf "${AGE_FILENAME}" -C /usr/local/bin --strip-components 1 age/age age/age-keygen && \
    rm "${AGE_FILENAME}" && \
    [ "$(age --version)" = "${AGE_VERSION}" ] && \
    [ "$(age-keygen --version)" = "${AGE_VERSION}" ]

RUN helm plugin install https://github.com/databus23/helm-diff --version v3.9.2 && \
    helm plugin install https://github.com/jkroepke/helm-secrets --version v4.5.1 && \
    helm plugin install https://github.com/hypnoglow/helm-s3.git --version v0.15.1 && \
    helm plugin install https://github.com/aslafy-z/helm-git.git --version v0.15.1 && \
    rm -rf ${HELM_CACHE_HOME}/plugins

# Allow users other than root to use helm plugins located in root home
RUN chmod 751 ${HOME}

COPY --from=builder /workspace/helmfile/dist/helmfile_${TARGETOS}_${TARGETARCH} /usr/local/bin/helmfile

CMD ["/usr/local/bin/helmfile"]
