FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

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

ARG HELM_VERSION="v3.15.4"
ENV HELM_VERSION="${HELM_VERSION}"
ARG HELM_LOCATION="https://get.helm.sh"
ARG HELM_FILENAME="helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "${HELM_LOCATION}/${HELM_FILENAME}" && \
    echo Verifying ${HELM_FILENAME}... && \
    case ${TARGETPLATFORM} in \
        "linux/amd64")  HELM_SHA256="11400fecfc07fd6f034863e4e0c4c4445594673fd2a129e701fe41f31170cfa9"  ;; \
        "linux/arm64")  HELM_SHA256="fa419ecb139442e8a594c242343fafb7a46af3af34041c4eac1efcc49d74e626"  ;; \
    esac && \
    echo "${HELM_SHA256}  ${HELM_FILENAME}" | sha256sum -c && \
    echo Extracting ${HELM_FILENAME}... && \
    tar xvf "${HELM_FILENAME}" -C /usr/local/bin --strip-components 1 ${TARGETOS}-${TARGETARCH}/helm && \
    rm "${HELM_FILENAME}" && \
    [ "$(helm version --template '{{.Version}}')" = "${HELM_VERSION}" ]

# using the install documentation found at https://kubernetes.io/docs/tasks/tools/install-kubectl/
# for now but in a future version of alpine (in the testing version at the time of writing)
# we should be able to install using apk add.
ENV KUBECTL_VERSION="v1.28.9"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl" && \
    case ${TARGETPLATFORM} in \
        # checksums are available at https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl.sha256
        "linux/amd64")  KUBECTL_SHA256="b4693d0b22f509250694b10c7727c42b427d570af04f2065fe23a55d6c0051f1"  ;; \
        "linux/arm64")  KUBECTL_SHA256="e0341d3973213f8099e7fcbbf6d1d506967bc2b7a4faac3fb3b4340f226e9b2f"  ;; \
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

ENV SOPS_VERSION="v3.9.0"
ARG SOPS_FILENAME="sops-${SOPS_VERSION}.${TARGETOS}.${TARGETARCH}"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/getsops/sops/releases/download/${SOPS_VERSION}/${SOPS_FILENAME}" && \
    chmod +x "${SOPS_FILENAME}" && \
    mv "${SOPS_FILENAME}" /usr/local/bin/sops && \
    sops --version --disable-version-check | grep -E "^sops ${SOPS_VERSION#v}"

ENV AGE_VERSION="v1.1.1"
ARG AGE_FILENAME="age-${AGE_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/FiloSottile/age/releases/download/${AGE_VERSION}/${AGE_FILENAME}" && \
    tar xvf "${AGE_FILENAME}" -C /usr/local/bin --strip-components 1 age/age age/age-keygen && \
    rm "${AGE_FILENAME}" && \
    [ "$(age --version)" = "${AGE_VERSION}" ] && \
    [ "$(age-keygen --version)" = "${AGE_VERSION}" ]

RUN helm plugin install https://github.com/databus23/helm-diff --version v3.9.8 && \
    helm plugin install https://github.com/jkroepke/helm-secrets --version v4.6.0 && \
    helm plugin install https://github.com/hypnoglow/helm-s3.git --version v0.16.2 && \
    helm plugin install https://github.com/aslafy-z/helm-git.git --version v0.16.0 && \
    rm -rf ${HELM_CACHE_HOME}/plugins

# Allow users other than root to use helm plugins located in root home
RUN chmod 751 ${HOME}

COPY --from=builder /workspace/helmfile/dist/helmfile_${TARGETOS}_${TARGETARCH} /usr/local/bin/helmfile

CMD ["/usr/local/bin/helmfile"]
