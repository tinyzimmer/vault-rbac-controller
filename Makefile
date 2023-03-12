SHELL := /bin/bash

IMG  ?= ghcr.io/tinyzimmer/vault-rbac-controller:latest

uname_m := $(shell uname -m)
ifeq ($(uname_m),x86_64)
	ARCH ?= amd64
else ifeq ($(uname_m),aarch64)
	ARCH ?= arm64
else ifeq ($(uname_m),armv7l)
	ARCH ?= arm
else
	ARCH ?= $(uname_m)
endif
OS ?= linux


##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

default: build

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build: ## Build the vault-rbac-controller image for the current architecture. This is the default target.
	go mod download -x
	CGO_ENABLED=0 GOARCH=$(ARCH) GOOS=$(OS) \
		go build -o dist/vault-rbac-controller_$(OS)_$(ARCH) .
	docker buildx build --load --platform $(OS)/$(ARCH) -t $(IMG) .

.PHONY: dist
PLATFORMS ?= linux/amd64 linux/arm64 linux/arm
dist: ## Build the vault-rbac-controller release images for all supported architectures
	rm -rf dist/
	go install github.com/mitchellh/gox@latest
	mkdir -p dist/
	CGO_ENABLED=0 gox \
		-rebuild \
		-tags netgo \
		-ldflags "-s -w -X main.version=$(shell git describe --tags) -X main.commit=$(shell git rev-parse HEAD)" \
		-osarch="$(PLATFORMS)" \
		-output="dist/vault-rbac-controller_{{.OS}}_{{.Arch}}" .
	upx --best --lzma dist/*

##@ Development

GINKGO_VERSION ?= v2.9.0
test: setup-envtest ## Run the unit tests
	go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
	$(shell setup-envtest use -p env) ; \
		ginkgo run -r -v \
			--race \
			--output-dir ./ \
			--junit-report junit_report.xml \
			--cover --covermode atomic \
			--coverprofile cover.out
	go tool cover -func cover.out

GOLANGCI_LINT_VERSION = v1.51.2
lint: ## Run the linter
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	golangci-lint run -v --timeout 600s

setup-envtest: ## Download the envtest binaries
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	setup-envtest use

##@ Local Cluster Testing

CLUSTER_NAME ?= vault-rbac-controller
KUBECONTEXT  ?= k3d-$(CLUSTER_NAME)

test-cluster: create-cluster install-vault init-vault configure-vault ## Create a local k3d cluster and install/configure vault

test-cluster-tls: create-cluster install-certmanager ## Create a local k3d cluster and install/configure vault with self-signed TLS
	$(MAKE) install-vault VAULT_VALUES=deploy/test_vault_values_tls.yaml
	$(MAKE) init-vault configure-vault
	$(KUBECTL) -n vault get secret vault-tls -o json \
		| jq -r '.data["ca.crt"]' \
		| base64 -d \
		| $(KUBECTL) create secret generic vault-tls-ca --from-file=ca.crt=/dev/stdin

create-cluster: ## Create a local k3d cluster
	k3d cluster create $(CLUSTER_NAME)

destroy-cluster: ## Destroy the local k3d cluster
	k3d cluster delete $(CLUSTER_NAME)

HELM    := helm --kube-context $(KUBECONTEXT)
KUBECTL := kubectl --context $(KUBECONTEXT)

VAULT_VALUES ?= deploy/test_vault_values.yaml
install-vault: ## Install vault into the local k3d cluster
	$(HELM) repo add hashicorp https://helm.releases.hashicorp.com
	$(HELM) repo update
	$(HELM) upgrade --install --wait \
		--create-namespace \
		--namespace vault \
		--values $(VAULT_VALUES) \
		vault hashicorp/vault
	$(KUBECTL) wait pod \
		--namespace vault \
		--for=condition=Ready \
		--timeout=300s \
		vault-0

install-certmanager: ## Install cert-manager into the local k3d cluster
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) upgrade --install --wait \
		--create-namespace \
		--namespace cert-manager \
		--set installCRDs=true \
		cert-manager jetstack/cert-manager
	$(KUBECTL) wait pod \
		--namespace cert-manager \
		--for=condition=Ready \
		--timeout=300s \
		--selector=app.kubernetes.io/name=cert-manager
	echo "$${VAULT_TLS}" | $(KUBECTL) apply -f -

define VAULT_TLS
---
apiVersion: v1
kind: Namespace
metadata:
  name: vault
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: vault-tls
  namespace: vault
spec:
  commonName: vault-tls
  secretName: vault-tls
  dnsNames:
  - vault
  - vault.vault
  - vault.vault.svc
  - vault.vault.svc.cluster.local
  ipAddresses:
  - 127.0.0.1
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
    group: cert-manager.io
endef
export VAULT_TLS

SVCACCT ?= vault-rbac-controller
VAULT   := $(KUBECTL) exec -it --namespace vault vault-0 -- vault

init-vault: ## Initialize vault
	$(VAULT) operator init -key-shares=1 -key-threshold=1 -format=json > keys.json
	$(VAULT) operator unseal "$$(jq -r ".unseal_keys_b64[]" keys.json)"
	$(VAULT) login $$(jq -r ".root_token" keys.json)
	$(VAULT) secrets enable -path=secret kv-v2
	$(VAULT) auth enable kubernetes

configure-vault: ## Configure vault
	cat deploy/vault_policy.hcl | $(VAULT) policy write $(SVCACCT) -
	$(KUBECTL) exec -it --namespace vault vault-0 -- \
		/bin/sh -c \
		'vault write auth/kubernetes/config \
			kubernetes_host="https://$${KUBERNETES_SERVICE_HOST}:$${KUBERNETES_SERVICE_PORT}"'
	$(VAULT) write auth/kubernetes/role/$(SVCACCT) \
		bound_service_account_names=$(SVCACCT) \
		bound_service_account_namespaces=default \
		policies=$(SVCACCT) \
		ttl=1h
	$(VAULT) kv put secret/example api_key=$(shell uuidgen)

load-image: ## Load the controller image into the local k3d cluster
	k3d image import $(IMG) --cluster $(CLUSTER_NAME)

RELEASE_NAME ?= rbac-controller
deploy-controller: load-image ## Deploy the controller into the local k3d cluster
	kubectl kustomize deploy/kustomize \
		| $(KUBECTL) apply -f -

undeploy-controller: ## Undeploy the controller from the local k3d cluster
	kubectl kustomize deploy/kustomize \
		| $(KUBECTL) delete -f -
