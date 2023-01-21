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
kubectl exec -it --namespace vault vault-0 -- vault operator init -key-shares=1 -key-threshold=1 -format=json > keys.json
kubectl exec -it --namespace vault vault-0 -- vault operator unseal "$(jq -r ".unseal_keys_b64[]" keys.json)"
# Login on the pod
kubectl exec -it --namespace vault vault-0 -- vault login $(jq -r ".root_token" keys.json)
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

You can either use the `helm` chart or the `kustomizization` to deploy the controller.
Both can be found in the `deploy/` directory.
The chart is not published anywhere yet, so you'll have to clone the repository first to use it.
The defaults in the `kustomization` assume the names and insecure settings used in this quickstart.
Otherwise, edit the `config_patch.yaml` to suit your needs or create a wrapping kustomization.
To install using `kustomize`:

```bash
kubectl kustomize https://github.com/tinyzimmer/vault-rbac-controller/deploy/kustomize \
    | kubectl apply -f -
```

You can now experiment with the controller.

The manifests in [deploy/samples](deploy/samples) contain various ways to use the controller.
They all depend on a `secret/example`. You can generate one with:

```bash
kubectl exec -it --namespace vault vault-0 -- vault kv put secret/example api_key=$(uuidgen)
```

## Usage

Vault ACLs for a ServiceAccount can be configured in one of three ways:

 - Inline policy in an annotation on the ServiceAccount
 - Inline policy in a ConfigMap referenced by an annotation on the ServiceAccount
 - Roles containing rules with the `apiGroup` "vault.hashicorp.com" and their associated RoleBindings.

Complete examples can be found in the [deploy/samples](deploy/samples) directory.
For a full list of the annotations used with their descriptions, see the [annotations.go](internal/api/annotations.go) file.

Below are the command-line options for the controller:

```
-auth-mount string
    The auth mount for the kubernetes auth method. (default "kubernetes")
-exclude-namespaces string
    The namespaces to exclude from watching. If empty, no namespaces are excluded.
-health-probe-bind-address string
    The address the probe endpoint binds to. (default ":8081")
-include-system-namespaces
    Include system namespaces in the watched namespaces.
-kubeconfig string
    Paths to a kubeconfig. Only required if out-of-cluster.
-leader-elect
    Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
-metrics-bind-address string
    The address the metric endpoint binds to. (default ":8080")
-namespaces string
    The namespaces to watch for roles. If empty, all namespaces are watched.
-use-finalizers
    Ensure finalizers on resources to attempt to clean up on deletion.
-zap-devel
    Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
-zap-encoder value
    Zap log encoding (one of 'json' or 'console')
-zap-log-level value
    Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
-zap-stacktrace-level value
    Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
-zap-time-encoding value
    Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
```