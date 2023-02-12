FROM golang:1.20-alpine as builder

RUN apk add --no-cache make git
WORKDIR /workspace/helmfile

COPY go.mod go.sum /workspace/helmfile/
RUN go mod download

COPY . /workspace/helmfile
RUN make static-linux

# -----------------------------------------------------------------------------

FROM alpine:3.16

LABEL org.opencontainers.image.source https://github.com/helmfile/helmfile

RUN apk add --no-cache ca-certificates git bash curl jq openssh-client gnupg

# Set Helm home variables so that also non-root users can use plugins etc.
ARG HOME="/helm"
ENV HOME="${HOME}"
ARG HELM_CACHE_HOME="${HOME}/.cache/helm"
ENV HELM_CACHE_HOME="${HELM_CACHE_HOME}"
ARG HELM_CONFIG_HOME="${HOME}/.config/helm"
ENV HELM_CONFIG_HOME="${HELM_CONFIG_HOME}"
ARG HELM_DATA_HOME="${HOME}/.local/share/helm"
ENV HELM_DATA_HOME="${HELM_DATA_HOME}"

ARG HELM_VERSION="v3.11.1"
ENV HELM_VERSION="${HELM_VERSION}"
ARG HELM_SHA256="0b1be96b66fab4770526f136f5f1a385a47c41923d33aab0dcb500e0f6c1bf7c"
ARG HELM_LOCATION="https://get.helm.sh"
ARG HELM_FILENAME="helm-${HELM_VERSION}-linux-amd64.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "${HELM_LOCATION}/${HELM_FILENAME}" && \
    echo Verifying ${HELM_FILENAME}... && \
    echo "${HELM_SHA256}  ${HELM_FILENAME}" | sha256sum -c && \
    echo Extracting ${HELM_FILENAME}... && \
    tar xvf "${HELM_FILENAME}" -C /usr/local/bin --strip-components 1 linux-amd64/helm && \
    rm "${HELM_FILENAME}" && \
    [ "$(helm version --template '{{.Version}}')" = "${HELM_VERSION}" ]

# using the install documentation found at https://kubernetes.io/docs/tasks/tools/install-kubectl/
# for now but in a future version of alpine (in the testing version at the time of writing)
# we should be able to install using apk add.
ENV KUBECTL_VERSION="v1.25.2"
ARG KUBECTL_SHA256="8639f2b9c33d38910d706171ce3d25be9b19fc139d0e3d4627f38ce84f9040eb"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    echo "${KUBECTL_SHA256}  kubectl" | sha256sum -c && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/kubectl && \
    [ "$(kubectl version -o json | jq -r '.clientVersion.gitVersion')" = "${KUBECTL_VERSION}" ]

ENV KUSTOMIZE_VERSION="v4.5.7"
ARG KUSTOMIZE_SHA256="701e3c4bfa14e4c520d481fdf7131f902531bfc002cb5062dcf31263a09c70c9"
ARG KUSTOMIZE_FILENAME="kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/${KUSTOMIZE_FILENAME}" && \
    echo "${KUSTOMIZE_SHA256}  ${KUSTOMIZE_FILENAME}" | sha256sum -c && \
    tar xvf "${KUSTOMIZE_FILENAME}" -C /usr/local/bin && \
    rm "${KUSTOMIZE_FILENAME}" && \
    kustomize version --short | grep "kustomize/${KUSTOMIZE_VERSION}"

ENV SOPS_VERSION="v3.7.3"
ARG SOPS_FILENAME="sops-${SOPS_VERSION}.linux.amd64"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/mozilla/sops/releases/download/${SOPS_VERSION}/${SOPS_FILENAME}" && \
    chmod +x "${SOPS_FILENAME}" && \
    mv "${SOPS_FILENAME}" /usr/local/bin/sops && \
    sops --version | grep -E "^sops ${SOPS_VERSION#v}"

ENV AGE_VERSION="v1.0.0"
ARG AGE_FILENAME="age-${AGE_VERSION}-linux-amd64.tar.gz"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://github.com/FiloSottile/age/releases/download/${AGE_VERSION}/${AGE_FILENAME}" && \
    tar xvf "${AGE_FILENAME}" -C /usr/local/bin --strip-components 1 age/age age/age-keygen && \
    rm "${AGE_FILENAME}" && \
    [ "$(age --version)" = "${AGE_VERSION}" ] && \
    [ "$(age-keygen --version)" = "${AGE_VERSION}" ]

RUN helm plugin install https://github.com/databus23/helm-diff --version v3.6.0 && \
    helm plugin install https://github.com/jkroepke/helm-secrets --version v4.1.1 && \
    helm plugin install https://github.com/hypnoglow/helm-s3.git --version v0.14.0 && \
    helm plugin install https://github.com/aslafy-z/helm-git.git --version v0.12.0 && \
    rm -rf ${HELM_CACHE_HOME}/plugins

# Allow users other than root to use helm plugins located in root home
RUN chmod 751 ${HOME}

COPY --from=builder /workspace/helmfile/dist/helmfile_linux_amd64 /usr/local/bin/helmfile

CMD ["/usr/local/bin/helmfile"]
