/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package reconcilers

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

type RoleReconciler struct {
	client.Client

	policies      vault.PolicyManager
	useFinalizers bool
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
	if !vault.HasACLs(role) {
		ctrl.LoggerFrom(ctx).Info("no vault rules found in role, skipping")
		return nil
	}
	policy := vault.ToJSONPolicyString(vault.FilterACLs(role.Rules))
	if err := r.policies.WritePolicy(ctx, role, policy); err != nil {
		return fmt.Errorf("unable to put policy in vault: %w", err)
	}
	if r.useFinalizers && !util.HasFinalizer(role) {
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
	if err := r.policies.DeletePolicy(ctx, role); err != nil {
		return fmt.Errorf("unable to delete policy in vault: %w", err)
	}
	// Remove the finalizer
	if err := util.RemoveFinalizer(ctx, r.Client, role); err != nil {
		return fmt.Errorf("unable to remove finalizer from role: %w", err)
	}
	return nil
}
