package api

const (
	// VaultRBACControllerFinalizer is the finalizer added to resources managed by the controller.
	ResourceFinalizer = "vault-rbac-controller/finalizer"
	// VaultPolicyKey is the key in configmaps that contains the Vault policy.
	VaultPolicyKey = "policy.hcl"
)
