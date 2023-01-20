/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vault

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
)

type policyJSON struct {
	Path map[string]pathPolicy `json:"path"`
}

type pathPolicy struct {
	Capabilities []string `json:"capabilities"`
}

func ToJSONPolicyString(rules []rbacv1.PolicyRule) string {
	pol := policyJSON{Path: make(map[string]pathPolicy)}
	for _, rule := range rules {
		for _, res := range rule.Resources {
			pol.Path[res] = pathPolicy{Capabilities: rule.Verbs}
		}
	}
	// Will never error since we are marshaling strings
	out, _ := json.MarshalIndent(pol, "", "  ")
	return string(out)
}

const vaultAPIGroup = "vault.hashicorp.com"

func FilterACLs(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	var out []rbacv1.PolicyRule
	for _, rule := range rules {
		if len(rule.APIGroups) != 0 && rule.APIGroups[0] == vaultAPIGroup {
			out = append(out, rule)
		}
	}
	return out
}

func HasACLs(object client.Object) bool {
	switch object := object.(type) {
	case *rbacv1.Role:
		return roleHasACLs(object)
	case *corev1.ServiceAccount:
		return serviceAccountHasACLs(object)
	default:
		return false
	}
}

func serviceAccountHasACLs(svcacct *corev1.ServiceAccount) bool {
	return util.HasAnnotation(svcacct, api.VaultInlinePolicyAnnotation) ||
		util.HasAnnotation(svcacct, api.VaultConfigMapPolicyAnnotation)
}

func roleHasACLs(role *rbacv1.Role) bool {
	return len(FilterACLs(role.Rules)) > 0
}
