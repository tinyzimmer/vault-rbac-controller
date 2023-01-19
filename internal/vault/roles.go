/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vault

import (
	"context"
	"fmt"
	"path"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
)

type RoleManager interface {
	RoleName(client.Object) string
	WriteRole(ctx context.Context, obj client.Object, params map[string]any) error
	DeleteRole(ctx context.Context, obj client.Object) error
}

func NewRoleManager(authMount string) RoleManager {
	return &roleManager{authMount: authMount}
}

type roleManager struct {
	authMount string
}

func (r *roleManager) RoleName(obj client.Object) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		if role, ok := annotations[api.VaultRoleNameAnnotation]; ok {
			return role
		}
	}
	return util.DefaultResourceFormat(obj.GetNamespace(), obj.GetName())
}

func (r *roleManager) WriteRole(ctx context.Context, obj client.Object, params map[string]any) error {
	path := path.Join("auth", r.authMount, "role", r.RoleName(obj))
	cli, err := NewClient()
	if err != nil {
		return fmt.Errorf("failed to get vault client: %w", err)
	}
	_, err = cli.Logical().WriteWithContext(ctx, path, params)
	return err
}

func (r *roleManager) DeleteRole(ctx context.Context, obj client.Object) error {
	path := path.Join("auth", r.authMount, "role", r.RoleName(obj))
	cli, err := NewClient()
	if err != nil {
		return fmt.Errorf("failed to get vault client: %w", err)
	}
	_, err = cli.Logical().DeleteWithContext(ctx, path)
	return err
}
