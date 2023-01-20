SHELL := /bin/bash

IMG ?= ghcr.io/tinyzimmer/vault-rbac-controller:latest
build: dist
	docker build --platform linux/amd64 -t $(IMG) .

.PHONY: dist
dist:
	go install github.com/mitchellh/gox@latest
	mkdir -p dist/
	CGO_ENABLED=0 gox \
		-tags netgo \
		-ldflags "-s -w" \
		-osarch="linux/amd64 linux/arm64" \
		-output="dist/vault-rbac-controller_{{.OS}}_{{.Arch}}" .
	upx --best --lzma dist/*

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

create-cluster:
	k3d cluster create $(CLUSTER_NAME)

destroy-cluster:
	k3d cluster delete $(CLUSTER_NAME)

HELM    := helm --kube-context $(KUBECONTEXT)
KUBECTL := kubectl --context $(KUBECONTEXT)
install-vault:
	$(HELM) repo add hashicorp https://helm.releases.hashicorp.com
	$(HELM) repo update
	$(HELM) upgrade --install --wait \
		--create-namespace \
		--namespace vault \
		--values deploy/test_vault_values.yaml \
		vault hashicorp/vault
	$(KUBECTL) wait pod \
		--namespace vault \
		--for=condition=Ready \
		--timeout=300s \
		vault-0

SVCACCT ?= vault-rbac-controller
VAULT   := $(KUBECTL) exec -it --namespace vault vault-0 -- vault

init-vault:
	$(VAULT) operator init -key-shares=1 -key-threshold=1 > keys.txt
	$(VAULT) operator unseal "$$(grep 'Unseal Key 1:' keys.txt | awk '{print $$4}')"
	$(VAULT) login $$(grep 'Initial Root Token:' keys.txt | awk '{print $$4}')
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
