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

	"github.com/edgeworx/vault-rbac-controller/internal/api"
	"github.com/edgeworx/vault-rbac-controller/internal/util"
	"github.com/edgeworx/vault-rbac-controller/internal/vault"
)

type RoleBindingReconciler struct {
	client.Client

	recorder      record.EventRecorder
	policies      vault.PolicyManager
	roles         vault.RoleManager
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
		r.recorder.Event(&rb, corev1.EventTypeNormal, api.EventReasonIgnored, "RoleBinding is ignored by the controller")
		return ctrl.Result{}, nil
	}

	if rb.GetDeletionTimestamp() != nil {
		if err := r.reconcileDelete(ctx, &rb); err != nil {
			r.recorder.Event(&rb, corev1.EventTypeWarning, api.EventReasonError, err.Error())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.reconcileCreateUpdate(ctx, &rb); err != nil {
		r.recorder.Event(&rb, corev1.EventTypeWarning, api.EventReasonError, err.Error())
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RoleBindingReconciler) reconcileCreateUpdate(ctx context.Context, rb *rbacv1.RoleBinding) error {
	// Retrieve the role to determine the policy name
	var role rbacv1.Role
	if err := r.Get(ctx, client.ObjectKey{Namespace: rb.Namespace, Name: rb.RoleRef.Name}, &role); err != nil {
		return fmt.Errorf("unable to fetch role: %w", err)
	}

	if !vault.HasACLs(&role) {
		ctrl.LoggerFrom(ctx).Info("rolebinding's role has no ACLs, skipping")
		r.recorder.Event(rb, corev1.EventTypeNormal, api.EventReasonIgnored, "RoleBinding's Role does not contain Vault ACLs")
		return nil
	}

	params, err := buildAuthRoleParameters(ctx, r.Client, rb, []string{r.policies.PolicyName(&role)})
	if err != nil {
		return fmt.Errorf("unable to build auth role parameters: %w", err)
	}

	// Write the role binding to vault
	if err := r.roles.WriteRole(ctx, rb, params); err != nil {
		return fmt.Errorf("unable to write role binding to vault: %w", err)
	}

	// Add finalizer if not present
	if r.useFinalizers && !controllerutil.ContainsFinalizer(rb, api.ResourceFinalizer) {
		if err := addFinalizer(ctx, r.Client, rb); err != nil {
			return fmt.Errorf("unable to update rolebinding with finalizer: %w", err)
		}
	}
	r.recorder.Event(rb, corev1.EventTypeNormal, api.EventReasonSynced, "RoleBinding synced to Vault")
	return nil
}

func (r *RoleBindingReconciler) reconcileDelete(ctx context.Context, rb *rbacv1.RoleBinding) error {
	if !controllerutil.ContainsFinalizer(rb, api.ResourceFinalizer) {
		return nil
	}
	// Delete the role binding from vault
	if err := r.roles.DeleteRole(ctx, rb); err != nil {
		return fmt.Errorf("unable to delete role binding from vault: %w", err)
	}
	if err := removeFinalizer(ctx, r.Client, rb); err != nil {
		return fmt.Errorf("unable to remove finalizer from rolebinding: %w", err)
	}
	return nil
}
