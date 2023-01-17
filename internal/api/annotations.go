package api

// Annotations
const (
	// Common Annotations

	// VaultRoleBindAnnotation instructs the controller to emulate the rolebinding/serviceaccount
	// in vault.
	VaultRoleBindAnnotation = "vault.hashicorp.com/bind"
	// VaultIgnoreAnnotation instructs the controller to ignore the resource.
	VaultIgnoreAnnotation = "vault.hashicorp.com/ignore"
	// RoleName instructs the controller to bind the service account or rolebinding to a
	// Vault role with the given name. If this annotation is not set, the controller will
	// use the default format of "${namespace}-${resource_name}".
	VaultRoleNameAnnotation = "vault.hashicorp.com/role-name"
	// VaultRoleConfigMapAnnotation instructs the controller to use the given configmap for the
	// parameters of the connection role in Vault. Any annotations on the rolebinding or serviceaccount
	// will override those parameters found in the configmap.
	VaultRoleConfigMapAnnotation = "vault.hashicorp.com/configmap"
	// VaultPolicyNameAnnotation instructs the controller to create a Vault policy with
	// the name of the annotation value. This policy will be bound to the role or serviceaccount.
	// If left unset the controller will use the default format of "${namespace}-${resource_name}".
	VaultPolicyNameAnnotation = "vault.hashicorp.com/policy-name"

	// ServiceAccount Annotations

	// VaultInlinePolicyAnnotation instructs the controller to create a Vault policy with
	// the contents of the annotation value. This policy will be bound to the service
	// account. If both this and VaultConfigMapPolicyAnnotation are set, the controller
	// will use the value of this annotation.
	VaultInlinePolicyAnnotation = "vault.hashicorp.com/inline-policy"
	// VaultConfigMapPolicyAnnotation instructs the controller to create a Vault policy
	// with the contents of the configmap referenced by the annotation value. This policy
	// will be bound to the service account.
	VaultConfigMapPolicyAnnotation = "vault.hashicorp.com/configmap-policy"

	// Annotations that can be applied to rolebindings/serviceaccounts for configuring auth roles.
	// See the API documentation for details:
	// https://developer.hashicorp.com/vault/api-docs/auth/kubernetes#create-role

	VaultRoleAudienceAnnotation             = "vault.hashicorp.com/audience"
	VaultRoleAliasNameSourceAnnotation      = "vault.hashicorp.com/alias-name-source"
	VaultRoleTokenTTLAnnotation             = "vault.hashicorp.com/token-ttl"
	VaultRoleTokenMaxTTLAnnotation          = "vault.hashicorp.com/token-max-ttl"
	VaultRoleTokenBoundCIDRsAnnotation      = "vault.hashicorp.com/token-bound-cidrs"
	VaultRoleTokenExplicitMaxTTLAnnotation  = "vault.hashicorp.com/token-explicit-max-ttl"
	VaultRoleTokenNoDefaultPolicyAnnotation = "vault.hashicorp.com/token-no-default-policy"
	VaultRoleTokenNumUsesAnnotation         = "vault.hashicorp.com/token-num-uses"
	VaultRoleTokenPeriodAnnotation          = "vault.hashicorp.com/token-period"
	VaultRoleTokenTypeAnnotation            = "vault.hashicorp.com/token-type"
)

var RoleConfigAnnotations = map[string]string{
	VaultRoleAudienceAnnotation:             "audience",
	VaultRoleAliasNameSourceAnnotation:      "alias_name_source",
	VaultRoleTokenTTLAnnotation:             "token_ttl",
	VaultRoleTokenMaxTTLAnnotation:          "token_max_ttl",
	VaultRoleTokenBoundCIDRsAnnotation:      "token_bound_cidrs",
	VaultRoleTokenExplicitMaxTTLAnnotation:  "token_explicit_max_ttl",
	VaultRoleTokenNoDefaultPolicyAnnotation: "token_no_default_policy",
	VaultRoleTokenNumUsesAnnotation:         "token_num_uses",
	VaultRoleTokenPeriodAnnotation:          "token_period",
	VaultRoleTokenTypeAnnotation:            "token_type",
}
