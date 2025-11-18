# Custom Dockerfile to install required helm plugins
FROM argoproj/argocd:latest

USER root
# Download OS dependencies
RUN apt-get update && \
    apt-get install -y \
        curl git wget unzip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
# Download helmfile
RUN wget https://github.com/roboll/helmfile/releases/download/v0.138.7/helmfile_linux_amd64 && \
    mv helmfile_linux_amd64 /usr/local/bin/helmfile && \
    chmod a+x /usr/local/bin/helmfile

# Download Vault
RUN wget https://releases.hashicorp.com/vault/1.5.0/vault_1.5.0_linux_amd64.zip -O /tmp/vault.zip --quiet && \
    unzip -p /tmp/vault.zip vault > /usr/local/bin/vault && \
    chmod a+x /usr/local/bin/vault && \
    rm /tmp/vault.zip

USER argocd

# Install helm-secrets plugin (as argocd user)
RUN helm plugin install https://github.com/jkroepke/helm-secrets --version v3.6.0 --verify=false

ENV HELM_PLUGINS="/home/argocd/.local/share/helm/plugins/"
