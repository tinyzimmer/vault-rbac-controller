/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package reconcilers

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

// Options are the options for configuring the reconcilers.
type Options struct {
	AuthMount               string
	Namespaces              []string
	ExcludeNamespaces       []string
	IncludeSystemNamespaces bool
	UseFinalizers           bool
}

// SetupWithManager sets up all reconcilers with the given manager.
func SetupWithManager(mgr ctrl.Manager, opts *Options) error {
	policies := vault.NewPolicyManager()
	roles := vault.NewRoleManager(opts.AuthMount)
	recorder := mgr.GetEventRecorderFor("vault-rbac-controller")
	roleReconciler := &RoleReconciler{
		Client:        mgr.GetClient(),
		recorder:      recorder,
		policies:      policies,
		useFinalizers: opts.UseFinalizers,
	}
	rbReconciler := &RoleBindingReconciler{
		Client:        mgr.GetClient(),
		recorder:      recorder,
		policies:      policies,
		roles:         roles,
		useFinalizers: opts.UseFinalizers,
	}
	saReconciler := &ServiceAccountReconciler{
		Client:        mgr.GetClient(),
		recorder:      recorder,
		policies:      policies,
		roles:         roles,
		useFinalizers: opts.UseFinalizers,
	}
	eventFilter := checkNamespacesPredicate(opts.Namespaces, opts.ExcludeNamespaces, opts.IncludeSystemNamespaces)
	for reconciler, builder := range map[reconcile.Reconciler]*builder.Builder{
		roleReconciler: ctrl.NewControllerManagedBy(mgr).
			For(&rbacv1.Role{}).
			WithEventFilter(eventFilter),
		rbReconciler: ctrl.NewControllerManagedBy(mgr).
			For(&rbacv1.RoleBinding{}).
			WithEventFilter(eventFilter),
		saReconciler: ctrl.NewControllerManagedBy(mgr).
			For(&corev1.ServiceAccount{}).
			WithEventFilter(eventFilter),
	} {
		if err := builder.Complete(reconciler); err != nil {
			return err
		}
	}
	return nil
}

func checkNamespacesPredicate(namespaces []string, excludeNamespaces []string, includeKubeSystem bool) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return checkObject(e.Object, namespaces, excludeNamespaces, includeKubeSystem)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return checkObject(e.Object, namespaces, excludeNamespaces, includeKubeSystem)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return checkObject(e.ObjectNew, namespaces, excludeNamespaces, includeKubeSystem)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return checkObject(e.Object, namespaces, excludeNamespaces, includeKubeSystem)
		},
	}
}

func checkObject(obj client.Object, namespaces []string, excludeNamespaces []string, includeKubeSystem bool) bool {
	ns := obj.GetNamespace()
	if !includeKubeSystem && isSystemNamespace(ns) {
		return false
	}
	return (len(namespaces) == 0 || containsString(namespaces, ns)) &&
		!containsString(excludeNamespaces, ns)
}

func isSystemNamespace(ns string) bool {
	return ns == "kube-system" || ns == "kube-public" || ns == "kube-node-lease"
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
