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

	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
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
	roleReconciler := &RoleReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		UseFinalizers: opts.UseFinalizers,
	}
	rbReconciler := &RoleBindingReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		AuthMount:     opts.AuthMount,
		UseFinalizers: opts.UseFinalizers,
	}
	saReconciler := &ServiceAccountReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		AuthMount:     opts.AuthMount,
		UseFinalizers: opts.UseFinalizers,
	}
	eventFilter := checkNamespacesPredicate(opts.Namespaces, opts.ExcludeNamespaces, opts.IncludeSystemNamespaces)
	for reconciler, builder := range map[reconcile.Reconciler]*builder.Builder{
		roleReconciler: ctrl.NewControllerManagedBy(mgr).For(&rbacv1.Role{}).WithEventFilter(eventFilter),
		rbReconciler:   ctrl.NewControllerManagedBy(mgr).For(&rbacv1.RoleBinding{}).WithEventFilter(eventFilter),
		saReconciler:   ctrl.NewControllerManagedBy(mgr).For(&corev1.ServiceAccount{}).WithEventFilter(eventFilter),
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
	return (len(namespaces) == 0 || util.ContainsString(namespaces, ns)) &&
		!util.ContainsString(excludeNamespaces, ns)
}

func isSystemNamespace(ns string) bool {
	return ns == "kube-system" || ns == "kube-public" || ns == "kube-node-lease"
}
