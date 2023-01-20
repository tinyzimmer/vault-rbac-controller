# vault-rbac-controller

A controller for managing application access to Vault secrets via the Kubernetes RBAC system.

## Quickstart

### Installing and Configuring Vault

The controller is intended to be used with the Kubernetes auth method and the [Agent Sidecar Injector](https://developer.hashicorp.com/vault/docs/platform/k8s/injector) that
is included in the [Vault helm chart](https://developer.hashicorp.com/vault/docs/platform/k8s/helm) by default.
These components must be deployed to the cluster before beginning.

For the sake of the quickstart, we'll deploy a standalone Vault server without TLS and a single key.
We'll also disable the readiness/liveness probes so we can unseal the Vault. 
These are **not** the recommended practices for a production installation.

```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update
helm install \
    --create-namespace \
    --namespace vault \
    --set server.extraEnvironmentVars.VAULT_CLI_NO_COLOR="1" \
    --set server.readinessProbe.enabled=false \
    --set server.livenessProbe.enabled=false \
    vault hashicorp/vault

# Wait for the vault pod to start
kubectl wait pod --namespace vault --for=condition=Ready --timeout=300s vault-0
```

Next we need to initialize the vault, enable a secret engine, and enable the Kubernetes auth method.

```bash
# Initialize and unseal
kubectl exec -it --namespace vault vault-0 -- vault operator init -key-shares=1 -key-threshold=1 > keys.txt
kubectl exec -it --namespace vault vault-0 -- vault operator unseal "$(grep 'Unseal Key 1:' keys.txt | awk '{print $4}')"
# Login on the pod
kubectl exec -it --namespace vault vault-0 -- vault login $(grep 'Initial Root Token:' keys.txt | awk '{print $4}')
# Enable a secret engine
kubectl exec -it --namespace vault vault-0 -- vault secrets enable -path=secret kv-v2
# Enable the kubernetes auth method
kubectl exec -it --namespace vault vault-0 -- vault auth enable kubernetes
# Configure the auth method to use the internal API server
kubectl exec -it --namespace vault vault-0 -- \
    /bin/sh -c \
    'vault write auth/kubernetes/config \
        kubernetes_host="https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}"'
```

The deployment manifets for the controller assume the agent injector using Kubernetes auth.
An example minimum policy is included in the repository for the controller.
To create a Kubernetes auth role for the policy, execute the following commands:

```bash
# Write the policy included in this repository to the server
cat deploy/vault_policy.hcl | kubectl exec -it --namespace vault vault-0 -- vault policy write vault-rbac-controller -
# Bind the controller's (soon to be) service account to the policy
kubectl exec -it --namespace vault vault-0 -- vault write auth/kubernetes/role/vault-rbac-controller \
		bound_service_account_names=vault-rbac-controller \
		bound_service_account_namespaces=default \
		policies=vault-rbac-controller
```

### Installing the Controller

_To complete after repo setup_

The manifests in [deploy/samples](deploy/samples) contain various ways to use the controller.
They all depened on a `secret/example`. We can generate one with:

```bash
kubectl exec -it --namespace vault vault-0 -- vault kv put secret/example api_key=$(uuidgen)
```