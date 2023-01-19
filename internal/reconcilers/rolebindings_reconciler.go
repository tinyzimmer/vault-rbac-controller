/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package reconcilers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

type RoleBindingReconciler struct {
	client.Client

	policies      vault.PolicyManager
	roles         vault.RoleManager
	authMount     string
	useFinalizers bool
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

	params, err := buildAuthRoleParameters(ctx, r.Client, rb, policies)
	if err != nil {
		return fmt.Errorf("unable to build auth role parameters: %w", err)
	}

	// Write the role binding to vault
	if err := r.roles.WriteRole(ctx, rb, params); err != nil {
		return fmt.Errorf("unable to write role binding to vault: %w", err)
	}

	// Add finalizer if not present
	if r.useFinalizers && !util.HasFinalizer(rb) {
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
	// Collect all current policies for the role
	policies, err := r.getRolePolicies(ctx, rb)
	if err != nil {
		return fmt.Errorf("unable to get role policies: %w", err)
	}
	// If no policies are left, delete the rolebinding
	if len(policies) == 0 {
		// Delete the role binding from vault
		if err := r.roles.DeleteRole(ctx, rb); err != nil {
			return fmt.Errorf("unable to delete role binding from vault: %w", err)
		}
		if err := util.RemoveFinalizer(ctx, r.Client, rb); err != nil {
			return fmt.Errorf("unable to remove finalizer from rolebinding: %w", err)
		}
		return nil
	}
	params, err := buildAuthRoleParameters(ctx, r.Client, rb, policies)
	if err != nil {
		return fmt.Errorf("unable to build auth role parameters: %w", err)
	}
	if err := r.roles.WriteRole(ctx, rb, params); err != nil {
		return fmt.Errorf("unable to update role binding in vault: %w", err)
	}
	if err := util.RemoveFinalizer(ctx, r.Client, rb); err != nil {
		return fmt.Errorf("unable to remove finalizer from rolebinding: %w", err)
	}
	return nil
}

func (r *RoleBindingReconciler) getRolePolicies(ctx context.Context, current *rbacv1.RoleBinding) ([]string, error) {
	var rbs rbacv1.RoleBindingList
	if err := r.List(ctx, &rbs, client.InNamespace(current.GetNamespace())); err != nil {
		return nil, fmt.Errorf("unable to list rolebindings: %w", err)
	}
	var policies []string
	for _, rb := range rbs.Items {
		if rb.DeletionTimestamp != nil || util.IsIgnoredRoleBinding(&rb) {
			continue
		}
		if rb.RoleRef.Name == current.RoleRef.Name {
			var role rbacv1.Role
			if err := r.Get(ctx, client.ObjectKey{Namespace: rb.Namespace, Name: rb.RoleRef.Name}, &role); err != nil {
				return nil, fmt.Errorf("unable to fetch role: %w", err)
			}
			if !util.IsIgnoredRole(&role) && vault.HasACLs(&role) {
				policies = append(policies, r.policies.PolicyName(&role))
			}
		}
	}
	return policies, nil
}
