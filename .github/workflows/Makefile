HELM_VERSION ?= v3.7.2
KUSTOMIZE_VERSION ?= v5.4.3
K8S_VERSION ?= v1.32.1
MINIKUBE_VERSION ?= v1.31.1
SOPS_VERSION ?= v3.9.3

# ---
CHANGE_MINIKUBE_NONE_USER ?= true
MINIKUBE_WANTUPDATENOTIFICATION ?= false
MINIKUBE_WANTREPORTERRORPROMPT ?= false

VAULT_ADDR := http://127.0.0.1:8200
VAULT_TOKEN := toor

tmp := $(shell mktemp -d)
HELM_FILENAME := helm-${HELM_VERSION}-linux-amd64.tar.gz
KUSTOMIZE_FILENAME := kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz


all: vault sops helm kustomize minikube/destroy minikube

helm:
	curl -sSLo $(tmp)/${HELM_FILENAME} "https://get.helm.sh/${HELM_FILENAME}"
	tar zxf $(tmp)/${HELM_FILENAME} --directory ${tmp} linux-amd64/helm
	chmod +x ${tmp}/linux-amd64/helm
	sudo mv ${tmp}/linux-amd64/helm /usr/local/bin/
.PHONY: helm

kustomize:
	curl -sSLo $(tmp)/${KUSTOMIZE_FILENAME} "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/${KUSTOMIZE_FILENAME}"
	tar zxf $(tmp)/${KUSTOMIZE_FILENAME} --directory ${tmp} kustomize
	chmod +x ${tmp}/kustomize
	sudo mv ${tmp}/kustomize /usr/local/bin/
.PHONY: kustomize

minikube/destroy:
	sudo -E minikube delete || true
	sudo -E rm -rf /etc/kubernetes || true
	sudo -E rm -rf $$HOME/.minikube/* || true
.PHONY: minikube/destroy
.EXPORT_ALL_VARIABLES: minikube/destroy

minikube:
	curl -sSLo ${tmp}/kubectl https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/amd64/kubectl
	chmod +x ${tmp}/kubectl && sudo mv ${tmp}/kubectl /usr/local/bin/
	curl -sSLo ${tmp}/minikube https://storage.googleapis.com/minikube/releases/${MINIKUBE_VERSION}/minikube-linux-amd64
	chmod +x ${tmp}/minikube && sudo mv ${tmp}/minikube /usr/local/bin/
	sudo -E minikube delete || true
	sudo -E rm -rf /etc/kubernetes || true
	sudo -E rm -rf $$HOME/.minikube/* || true
	sudo -E minikube start --vm-driver=none --kubernetes-version=${K8S_VERSION}
	sudo -E minikube update-context
	kubectl wait node/minikube --for=condition=Ready
.PHONY: minikube
.EXPORT_ALL_VARIABLES: minikube

vault:
	docker kill $$(docker ps -a --filter "name=vault" -q) || true
	docker run -d -p8200:8200 --rm --name vault vault:1.2.0 server -dev -dev-root-token-id=toor
	docker run --rm --network="host" --cap-add IPC_LOCK -e VAULT_ADDR=$$VAULT_ADDR -e VAULT_TOKEN=$$VAULT_TOKEN  vault:1.2.0 secrets enable -path=sops transit
	docker run --rm --network="host" --cap-add IPC_LOCK -e VAULT_ADDR=$$VAULT_ADDR -e VAULT_TOKEN=$$VAULT_TOKEN  vault:1.2.0 write sops/keys/key type=rsa-4096
.PHONY: vault

sops:
	curl -sSLo $(tmp)/sops "https://github.com/getsops/sops/releases/download/${SOPS_VERSION}/sops-${SOPS_VERSION}.linux.amd64"
	chmod +x $(tmp)/sops
	sudo mv ${tmp}/sops /usr/local/bin/
.PHONY: sops
