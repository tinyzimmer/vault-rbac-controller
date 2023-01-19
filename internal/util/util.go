/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
)

func DefaultResourceFormat(namespace, name string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}

func IsIgnoredServiceAccount(svcacct *corev1.ServiceAccount) bool {
	return HasAnnotation(svcacct, api.VaultIgnoreAnnotation)
}

func IsIgnoredRole(role *rbacv1.Role) bool {
	return HasAnnotation(role, api.VaultIgnoreAnnotation)
}

func IsIgnoredRoleBinding(rolebinding *rbacv1.RoleBinding) bool {
	return HasAnnotation(rolebinding, api.VaultIgnoreAnnotation) ||
		!HasAnnotation(rolebinding, api.VaultRoleBindAnnotation)
}

func AddFinalizer(ctx context.Context, cli client.Client, obj client.Object) error {
	obj.SetFinalizers(append(obj.GetFinalizers(), api.ResourceFinalizer))
	return cli.Update(ctx, obj)
}

func RemoveFinalizer(ctx context.Context, cli client.Client, obj client.Object) error {
	obj.SetFinalizers(RemoveString(obj.GetFinalizers(), api.ResourceFinalizer))
	return cli.Update(ctx, obj)
}

func HasFinalizer(obj client.Object) bool {
	return ContainsString(obj.GetFinalizers(), api.ResourceFinalizer)
}

func ContainsString(slice []string, toCheck string) bool {
	for _, s := range slice {
		if s == toCheck {
			return true
		}
	}
	return false
}

func HasAnnotation(obj client.Object, toCheck string) bool {
	if annotations := obj.GetAnnotations(); annotations != nil {
		_, ok := annotations[toCheck]
		return ok
	}
	return false
}

func RemoveString(slice []string, toRemove string) []string {
	for i, s := range slice {
		if s == toRemove {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
