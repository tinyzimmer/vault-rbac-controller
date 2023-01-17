package reconcilers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

type RoleReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	UseFinalizers bool
}

func (r *RoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("reconciling role")

	var role rbacv1.Role
	if err := r.Get(ctx, req.NamespacedName, &role); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch role")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if util.IsIgnoredRole(&role) {
		log.Info("role is ignored, skipping")
		return ctrl.Result{}, nil
	}

	if role.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.reconcileDelete(ctx, &role)
	}

	return ctrl.Result{}, r.reconcileCreateUpdate(ctx, &role)
}

func (r *RoleReconciler) reconcileCreateUpdate(ctx context.Context, role *rbacv1.Role) error {
	manager := vault.NewPolicyManager(r.Client, role)
	if !manager.HasACLs() {
		ctrl.LoggerFrom(ctx).Info("no vault rules found in role, skipping")
		return nil
	}
	if err := manager.WritePolicy(ctx); err != nil {
		return fmt.Errorf("unable to put policy in vault: %w", err)
	}
	if r.UseFinalizers && !util.HasFinalizer(role) {
		if err := util.AddFinalizer(ctx, r.Client, role); err != nil {
			return fmt.Errorf("unable to update rolebinding with finalizer: %w", err)
		}
	}
	return nil
}

func (r *RoleReconciler) reconcileDelete(ctx context.Context, role *rbacv1.Role) error {
	if !util.HasFinalizer(role) {
		return nil
	}
	// Ensure the policy is deleted in vault
	if err := vault.NewPolicyManager(r.Client, role).DeletePolicy(ctx); err != nil {
		return fmt.Errorf("unable to delete policy in vault: %w", err)
	}
	// Remove the finalizer
	if err := util.RemoveFinalizer(ctx, r.Client, role); err != nil {
		return fmt.Errorf("unable to remove finalizer from role: %w", err)
	}
	return nil
}
