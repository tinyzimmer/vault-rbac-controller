package vault

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
)

type RoleManager interface {
	WriteRole(ctx context.Context, policies []string) error
	DeleteRole(ctx context.Context) error
}

func NewRoleManager(client client.Client, obj client.Object, authMount string) RoleManager {
	return &roleManager{client: client, obj: obj, authMount: authMount}
}

type roleManager struct {
	client    client.Client
	obj       client.Object
	authMount string
}

func (r *roleManager) WriteRole(ctx context.Context, policies []string) error {
	params, err := r.buildAuthRoleParameters(ctx, r.obj, policies)
	if err != nil {
		return fmt.Errorf("failed to build auth role parameters: %w", err)
	}
	cli, err := NewClient()
	if err != nil {
		return fmt.Errorf("failed to get vault client: %w", err)
	}
	path := path.Join("auth", r.authMount, "role", authRoleName(r.obj))
	_, err = cli.Logical().WriteWithContext(ctx, path, params)
	return err
}

func (r *roleManager) DeleteRole(ctx context.Context) error {
	path := path.Join("auth", r.authMount, "role", authRoleName(r.obj))
	cli, err := NewClient()
	if err != nil {
		return fmt.Errorf("failed to get vault client: %w", err)
	}
	_, err = cli.Logical().DeleteWithContext(ctx, path)
	return err
}

func (r *roleManager) buildAuthRoleParameters(ctx context.Context, obj client.Object, policies []string) (map[string]interface{}, error) {
	var saNames []string
	switch obj := obj.(type) {
	case *rbacv1.RoleBinding:
		for _, sub := range obj.Subjects {
			if sub.Kind == "ServiceAccount" {
				saNames = append(saNames, sub.Name)
			}
		}
	case *corev1.ServiceAccount:
		saNames = []string{obj.GetName()}
	default:
		return nil, errors.New("unknown object type")
	}
	params := map[string]interface{}{
		"bound_service_account_names":      saNames,
		"bound_service_account_namespaces": []string{obj.GetNamespace()},
		"policies":                         policies,
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return params, nil
	}
	// Check if a configmap is specified
	if configmap, ok := annotations[api.VaultRoleConfigMapAnnotation]; ok {
		var cm corev1.ConfigMap
		if err := r.client.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: configmap}, &cm); err != nil {
			return nil, fmt.Errorf("unable to fetch configmap: %w", err)
		}
		for k, v := range cm.Data {
			param := strings.Replace(k, "-", "_", -1)
			params[param] = v
		}
	}
	// Check other annotations
	for toCheck, param := range api.RoleConfigAnnotations {
		if val, ok := annotations[toCheck]; ok {
			params[param] = val
		}
	}
	return params, nil
}

func authRoleName(obj client.Object) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		if role, ok := annotations[api.VaultRoleNameAnnotation]; ok {
			return role
		}
	}
	return util.DefaultResourceFormat(obj.GetNamespace(), obj.GetName())
}
