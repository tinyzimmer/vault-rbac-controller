/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vault

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/edgeworx/vault-rbac-controller/internal/api"
)

func TestToJSONPolicyString(t *testing.T) {
	tt := []struct {
		rules []rbacv1.PolicyRule
		json  string
	}{
		{
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"foo"},
					Verbs:     []string{"read"},
				},
			},
			json: `
{
  "path": {
    "foo": {
      "capabilities": [
        "read"
      ]
    }
  }
}`,
		},
		{
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"bar"},
					Verbs:     []string{"update", "create", "read"},
				},
			},
			json: `
{
  "path": {
    "bar": {
      "capabilities": [
        "update",
        "create",
        "read"
      ]
    }
  }
}`,
		},
		{
			rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"foo"},
					Verbs:     []string{"read"},
				},
				{
					Resources: []string{"bar"},
					Verbs:     []string{"read"},
				},
			},
			json: `
{
  "path": {
    "bar": {
      "capabilities": [
        "read"
      ]
    },
    "foo": {
      "capabilities": [
        "read"
      ]
    }
  }
}`,
		},
	}
	for _, tc := range tt {
		out := ToJSONPolicyString(tc.rules)
		if out != strings.TrimSpace(tc.json) {
			t.Errorf("Expected %s, got %s", tc.json, out)
		}
	}
}

func TestHasACLs(t *testing.T) {
	tt := []struct {
		object  client.Object
		hasACLs bool
	}{
		{object: &corev1.Pod{}, hasACLs: false},
		{object: &corev1.ServiceAccount{}, hasACLs: false},
		{object: &rbacv1.Role{}, hasACLs: false},
		{object: &rbacv1.RoleBinding{}, hasACLs: false},
		{object: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					api.VaultInlinePolicyAnnotation: "",
				},
			},
		}, hasACLs: true},
		{object: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					api.VaultConfigMapPolicyAnnotation: "",
				},
			},
		}, hasACLs: true},
		{object: &rbacv1.Role{
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{vaultAPIGroup},
				},
			},
		}, hasACLs: true},
		{object: &rbacv1.Role{
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
				},
			},
		}, hasACLs: false},
	}
	for _, tc := range tt {
		if tc.hasACLs != HasACLs(tc.object) {
			t.Errorf("Expected %+v has ACLs = %t, got %t", tc.object, tc.hasACLs, HasACLs(tc.object))
		}
	}
}
