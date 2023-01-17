package reconcilers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

type ServiceAccountReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	AuthMount     string
	UseFinalizers bool
}

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("reconciling serviceaccount")

	var sa corev1.ServiceAccount
	if err := r.Get(ctx, req.NamespacedName, &sa); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch serviceaccount")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if util.IsIgnoredServiceAccount(&sa) {
		log.Info("serviceaccount is ignored, skipping")
		return ctrl.Result{}, nil
	}

	if sa.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.reconcileDelete(ctx, &sa)
	}

	return ctrl.Result{}, r.reconcileCreateUpdate(ctx, &sa)
}

func (r *ServiceAccountReconciler) reconcileCreateUpdate(ctx context.Context, sa *corev1.ServiceAccount) error {
	// Make sure we have the bind annotation
	if !util.HasAnnotation(sa, api.VaultRoleBindAnnotation) {
		ctrl.LoggerFrom(ctx).Info("service account is not bound or managed by a rolebinding, skipping")
		return nil
	}
	// Create the policy in vault
	policy := vault.NewPolicyManager(r.Client, sa)
	if !policy.HasACLs() {
		ctrl.LoggerFrom(ctx).Info("no vault rules found in serviceaccount, skipping")
		return nil
	}
	if err := policy.WritePolicy(ctx); err != nil {
		return fmt.Errorf("unable to put policy in vault: %w", err)
	}
	// Create an auth role in vault
	role := vault.NewRoleManager(r.Client, sa, r.AuthMount)
	if err := role.WriteRole(ctx, []string{policy.PolicyName()}); err != nil {
		return fmt.Errorf("unable to put auth role in vault: %w", err)
	}
	// Add finalizer if not present
	if r.UseFinalizers && !util.HasFinalizer(sa) {
		if err := util.AddFinalizer(ctx, r.Client, sa); err != nil {
			return fmt.Errorf("unable to update serviceaccount with finalizer: %w", err)
		}
	}
	return nil
}

func (r *ServiceAccountReconciler) reconcileDelete(ctx context.Context, sa *corev1.ServiceAccount) error {
	if !util.HasFinalizer(sa) {
		// Nothing to do
		return nil
	}
	// Ensure the policy is deleted in vault
	if err := vault.NewPolicyManager(r.Client, sa).DeletePolicy(ctx); err != nil {
		return fmt.Errorf("unable to delete policy in vault: %w", err)
	}
	// Ensure the auth role is deleted in vault
	if err := vault.NewRoleManager(r.Client, sa, r.AuthMount).DeleteRole(ctx); err != nil {
		return fmt.Errorf("unable to delete auth role in vault: %w", err)
	}
	// Remove the finalizer
	return util.RemoveFinalizer(ctx, r.Client, sa)
}
