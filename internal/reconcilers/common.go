package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
)

func addFinalizer(ctx context.Context, cli client.Client, obj client.Object) error {
	controllerutil.AddFinalizer(obj, api.ResourceFinalizer)
	return cli.Update(ctx, obj)
}

func removeFinalizer(ctx context.Context, cli client.Client, obj client.Object) error {
	controllerutil.RemoveFinalizer(obj, api.ResourceFinalizer)
	return cli.Update(ctx, obj)
}

func buildAuthRoleParameters(ctx context.Context, cli client.Client, obj client.Object, policies []string) (map[string]interface{}, error) {
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
		if err := cli.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: configmap}, &cm); err != nil {
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
