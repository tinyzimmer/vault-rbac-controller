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
build:
	go mod download -x
	CGO_ENABLED=0 GOARCH=$(ARCH) GOOS=$(OS) \
		go build -o dist/vault-rbac-controller_$(OS)_$(ARCH) .
	docker buildx build --load --platform $(OS)/$(ARCH) -t $(IMG) .

.PHONY: dist
PLATFORMS ?= linux/amd64 linux/arm64 linux/arm darwin/amd64 darwin/arm64
dist:
	rm -rf dist/
	go install github.com/mitchellh/gox@latest
	mkdir -p dist/
	CGO_ENABLED=0 gox \
		-rebuild \
		-tags netgo \
		-ldflags "-s -w" \
		-osarch="$(PLATFORMS)" \
		-output="dist/vault-rbac-controller_{{.OS}}_{{.Arch}}" .
	upx --best --lzma dist/*
	# Rename the windows binary
	mv dist/vault-rbac-controller_windows_amd64.exe dist/vault-rbac-controller_windows_amd64

test: setup-envtest
	go install github.com/onsi/ginkgo/v2/ginkgo@latest
	$(shell setup-envtest use -p env) ; \
		ginkgo run -r -v \
			--race \
			--output-dir ./ \
			--junit-report junit_report.xml \
			--cover --covermode atomic \
			--coverprofile cover.out
	go tool cover -func cover.out

GOLANGCI_LINT_VERSION = v1.50.1
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	golangci-lint run -v --timeout 300s

setup-envtest:
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	setup-envtest use

## Local development helpers

CLUSTER_NAME ?= vault-rbac-controller
KUBECONTEXT  ?= k3d-$(CLUSTER_NAME)

test-cluster: create-cluster install-vault init-vault configure-vault

test-cluster-tls: create-cluster install-certmanager
	$(MAKE) install-vault VAULT_VALUES=deploy/test_vault_values_tls.yaml
	$(MAKE) init-vault configure-vault
	$(KUBECTL) -n vault get secret vault-tls -o json \
		| jq -r '.data["ca.crt"]' \
		| base64 -d \
		| $(KUBECTL) create secret generic vault-tls-ca --from-file=ca.crt=/dev/stdin

create-cluster:
	k3d cluster create $(CLUSTER_NAME)

destroy-cluster:
	k3d cluster delete $(CLUSTER_NAME)

HELM    := helm --kube-context $(KUBECONTEXT)
KUBECTL := kubectl --context $(KUBECONTEXT)

VAULT_VALUES ?= deploy/test_vault_values.yaml
install-vault:
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

install-certmanager:
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

init-vault:
	$(VAULT) operator init -key-shares=1 -key-threshold=1 -format=json > keys.json
	$(VAULT) operator unseal "$$(jq -r ".unseal_keys_b64[]" keys.json)"
	$(VAULT) login $$(jq -r ".root_token" keys.json)
	$(VAULT) secrets enable -path=secret kv-v2
	$(VAULT) auth enable kubernetes

configure-vault:
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

load-image:
	k3d image import $(IMG) --cluster $(CLUSTER_NAME)

RELEASE_NAME ?= rbac-controller
deploy-controller: load-image
	kubectl kustomize deploy/kustomize \
		| $(KUBECTL) apply -f -

undeploy-controller:
	kubectl kustomize deploy/kustomize \
		| $(KUBECTL) delete -f -
