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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

type RoleReconciler struct {
	client.Client

	recorder      record.EventRecorder
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

	if role.GetDeletionTimestamp() != nil {
		if err := r.reconcileDelete(ctx, &role); err != nil {
			r.recorder.Event(&role, corev1.EventTypeWarning, api.EventReasonError, err.Error())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.reconcileCreateUpdate(ctx, &role); err != nil {
		r.recorder.Event(&role, corev1.EventTypeWarning, api.EventReasonError, err.Error())
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RoleReconciler) reconcileCreateUpdate(ctx context.Context, role *rbacv1.Role) error {
	if !vault.HasACLs(role) {
		ctrl.LoggerFrom(ctx).Info("no vault rules found in role, skipping")
		r.recorder.Event(role, corev1.EventTypeNormal, api.EventReasonIgnored, "Role does not contain any Vault ACLs")
		return nil
	}
	policy := vault.ToJSONPolicyString(vault.FilterACLs(role.Rules))
	if err := r.policies.WritePolicy(ctx, role, policy); err != nil {
		return fmt.Errorf("unable to put policy in vault: %w", err)
	}
	if r.useFinalizers && !controllerutil.ContainsFinalizer(role, api.ResourceFinalizer) {
		if err := addFinalizer(ctx, r.Client, role); err != nil {
			return fmt.Errorf("unable to update rolebinding with finalizer: %w", err)
		}
	}
	r.recorder.Event(role, corev1.EventTypeNormal, api.EventReasonSynced, "Role policy synced to Vault")
	return nil
}

func (r *RoleReconciler) reconcileDelete(ctx context.Context, role *rbacv1.Role) error {
	if !controllerutil.ContainsFinalizer(role, api.ResourceFinalizer) {
		return nil
	}
	// Ensure the policy is deleted in vault
	if err := r.policies.DeletePolicy(ctx, role); err != nil {
		return fmt.Errorf("unable to delete policy in vault: %w", err)
	}
	// Remove the finalizer
	if err := removeFinalizer(ctx, r.Client, role); err != nil {
		return fmt.Errorf("unable to remove finalizer from role: %w", err)
	}
	return nil
}
