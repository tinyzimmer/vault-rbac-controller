/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package util

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/edgeworx/vault-rbac-controller/internal/api"
)

func DefaultResourceFormat(namespace, name string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}

func IsIgnoredServiceAccount(svcacct *corev1.ServiceAccount) bool {
	return !HasAnnotation(svcacct, api.VaultRoleBindAnnotation)
}

func IsIgnoredRoleBinding(rolebinding *rbacv1.RoleBinding) bool {
	return !HasAnnotation(rolebinding, api.VaultRoleBindAnnotation)
}

func HasAnnotation(obj client.Object, toCheck string) bool {
	if annotations := obj.GetAnnotations(); annotations != nil {
		_, ok := annotations[toCheck]
		return ok
	}
	return false
}
