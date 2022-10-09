FROM golang:1.19.2-alpine as builder

RUN apk add --no-cache make git
WORKDIR /workspace/helmfile

COPY go.mod go.sum /workspace/helmfile/
RUN go mod download

COPY . /workspace/helmfile
RUN make static-linux

# -----------------------------------------------------------------------------

FROM alpine:3.16

LABEL org.opencontainers.image.source https://github.com/helmfile/helmfile

RUN apk add --no-cache ca-certificates git bash curl jq openssh-client

ARG HELM_VERSION="v3.10.0"
ARG HELM_SHA256="bf56beb418bb529b5e0d6d43d56654c5a03f89c98400b409d1013a33d9586474"
ARG HELM_LOCATION="https://get.helm.sh"
ARG HELM_FILENAME="helm-${HELM_VERSION}-linux-amd64.tar.gz"

RUN set -x && \
    wget ${HELM_LOCATION}/${HELM_FILENAME} && \
    echo Verifying ${HELM_FILENAME}... && \
    sha256sum ${HELM_FILENAME} | grep -q "${HELM_SHA256}" && \
    echo Extracting ${HELM_FILENAME}... && \
    tar zxvf ${HELM_FILENAME} && mv /linux-amd64/helm /usr/local/bin/ && \
    rm ${HELM_FILENAME} && rm -r /linux-amd64

# using the install documentation found at https://kubernetes.io/docs/tasks/tools/install-kubectl/
# for now but in a future version of alpine (in the testing version at the time of writing)
# we should be able to install using apk add.
# the sha256 sum can be found at https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256
# maybe a good idea to automate in the future?
ENV KUBECTL_VERSION="v1.25.2"
ENV KUBECTL_SHA256="8639f2b9c33d38910d706171ce3d25be9b19fc139d0e3d4627f38ce84f9040eb"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    sha256sum kubectl | grep ${KUBECTL_SHA256} && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/kubectl

ENV KUSTOMIZE_VERSION="v4.5.7"
ENV KUSTOMIZE_SHA256="0019dfc4b32d63c1392aa264aed2253c1e0c2fb09216f8e2cc269bbfb8bb49b5"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz && \
    sha256sum kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | grep ${KUSTOMIZE_SHA256} && \
    tar zxvf kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz && \
    rm kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz && \
    mv kustomize /usr/local/bin/kustomize

ENV SOPS_VERSION="v3.7.3"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO https://github.com/mozilla/sops/releases/download/${SOPS_VERSION}/sops-${SOPS_VERSION}.linux.amd64 && \
    chmod +x sops-${SOPS_VERSION}.linux.amd64  && \
    mv sops-${SOPS_VERSION}.linux.amd64 /usr/local/bin/sops

ENV AGE_VERSION="v1.0.0"
RUN set -x && \
    curl --retry 5 --retry-connrefused -LO https://github.com/FiloSottile/age/releases/download/${AGE_VERSION}/age-${AGE_VERSION}-linux-amd64.tar.gz && \
    tar zxvf age-${AGE_VERSION}-linux-amd64.tar.gz && \
    mv age/age /usr/local/bin/age && \
    mv age/age-keygen /usr/local/bin/age-keygen && \
    rm -rf age-${AGE_VERSION}-linux-amd64.tar.gz age

RUN helm plugin install https://github.com/databus23/helm-diff --version v3.6.0 && \
    helm plugin install https://github.com/jkroepke/helm-secrets --version v4.1.1 && \
    helm plugin install https://github.com/hypnoglow/helm-s3.git --version v0.14.0 && \
    helm plugin install https://github.com/aslafy-z/helm-git.git --version v0.12.0

# Allow users other than root to use helm plugins located in root home
RUN chmod 751 /root

COPY --from=builder /workspace/helmfile/dist/helmfile_linux_amd64 /usr/local/bin/helmfile

CMD ["/usr/local/bin/helmfile"]
