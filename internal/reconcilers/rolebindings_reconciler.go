package reconcilers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

type RoleBindingReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	AuthMount     string
	UseFinalizers bool
}

func (r *RoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("reconciling rolebinding")

	var rb rbacv1.RoleBinding
	if err := r.Get(ctx, req.NamespacedName, &rb); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch rolebinding")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if util.IsIgnoredRoleBinding(&rb) {
		log.Info("rolebinding is ignored, skipping")
		return ctrl.Result{}, nil
	}

	if rb.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.reconcileDelete(ctx, &rb)
	}

	return ctrl.Result{}, r.reconcileCreateUpdate(ctx, &rb)
}

func (r *RoleBindingReconciler) reconcileCreateUpdate(ctx context.Context, rb *rbacv1.RoleBinding) error {
	// Retrieve the service account
	var sa corev1.ServiceAccount
	if err := r.Get(ctx, client.ObjectKey{Namespace: rb.Namespace, Name: rb.Subjects[0].Name}, &sa); err != nil {
		return fmt.Errorf("unable to fetch service account: %w", err)
	}

	if util.IsIgnoredServiceAccount(&sa) {
		ctrl.LoggerFrom(ctx).Info("service account is ignored, skipping")
		return nil
	}

	// Collect all current policies for the role
	policies, err := r.getRolePolicies(ctx, rb)
	if err != nil {
		return fmt.Errorf("unable to get role policies: %w", err)
	}

	// Write the role binding to vault
	if err := vault.NewRoleManager(r.Client, rb, r.AuthMount).WriteRole(ctx, policies); err != nil {
		return fmt.Errorf("unable to write role binding to vault: %w", err)
	}

	// Add finalizer if not present
	if r.UseFinalizers && !util.HasFinalizer(rb) {
		if err := util.AddFinalizer(ctx, r.Client, rb); err != nil {
			return fmt.Errorf("unable to update rolebinding with finalizer: %w", err)
		}
	}

	return nil
}

func (r *RoleBindingReconciler) reconcileDelete(ctx context.Context, rb *rbacv1.RoleBinding) error {
	if !util.HasFinalizer(rb) {
		return nil
	}
	manager := vault.NewRoleManager(r.Client, rb, r.AuthMount)
	// Collect all current policies for the role
	policies, err := r.getRolePolicies(ctx, rb)
	if err != nil {
		return fmt.Errorf("unable to get role policies: %w", err)
	}
	// If no policies are left, delete the rolebinding
	if len(policies) == 0 {
		// Delete the role binding from vault
		if err := manager.DeleteRole(ctx); err != nil {
			return fmt.Errorf("unable to delete role binding from vault: %w", err)
		}
		return util.RemoveFinalizer(ctx, r.Client, rb)
	}
	if err := manager.WriteRole(ctx, policies); err != nil {
		return fmt.Errorf("unable to update role binding in vault: %w", err)
	}
	return util.RemoveFinalizer(ctx, r.Client, rb)
}

func (r *RoleBindingReconciler) getRolePolicies(ctx context.Context, current *rbacv1.RoleBinding) ([]string, error) {
	var rbs rbacv1.RoleBindingList
	if err := r.List(ctx, &rbs, client.InNamespace(current.GetNamespace())); err != nil {
		return nil, err
	}
	var policies []string
	for _, rb := range rbs.Items {
		if rb.RoleRef.Name == current.RoleRef.Name {
			var role rbacv1.Role
			if err := r.Get(ctx, client.ObjectKey{Namespace: rb.Namespace, Name: rb.RoleRef.Name}, &role); err != nil {
				return nil, err
			}
			manager := vault.NewPolicyManager(r.Client, &role)
			if !util.IsIgnoredRole(&role) && manager.HasACLs() {
				policies = append(policies, manager.PolicyName())
			}
		}
	}
	return policies, nil
}
