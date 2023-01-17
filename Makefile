CLUSTER_NAME ?= rbac-controller
KUBECONTEXT  ?= k3d-$(CLUSTER_NAME)

IMG ?= rbac-controller:latest
build:
	docker build -t $(IMG) .

test-cluster: create-cluster install-vault init-vault configure-vault

create-cluster:
	k3d cluster create $(CLUSTER_NAME)

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

SVCACCT ?= controller
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
	$(HELM) upgrade --install \
		--set image.pullPolicy=Never \
		--set serviceAccount.name=$(SVCACCT) \
		--set vault.tlsSkipVerify=true \
		--set controller.excludedNamespaces=vault \
		--set controller.useFinalizers=true \
		$(RELEASE_NAME) deploy/chart

undeploy-controller:
	$(HELM) uninstall $(RELEASE_NAME)

destroy:
	k3d cluster delete $(CLUSTER_NAME)