/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package reconcilers

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

type ServiceAccountReconciler struct {
	client.Client

	policies      vault.PolicyManager
	roles         vault.RoleManager
	authMount     string
	useFinalizers bool
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
	if !vault.HasACLs(sa) {
		ctrl.LoggerFrom(ctx).Info("no vault rules found in serviceaccount, skipping")
		return nil
	}
	policy, err := r.getServiceAccountPolicy(ctx, sa)
	if err != nil {
		return fmt.Errorf("unable to get serviceaccount policy: %w", err)
	}
	if err := r.policies.WritePolicy(ctx, sa, policy); err != nil {
		return fmt.Errorf("unable to put policy in vault: %w", err)
	}
	params, err := buildAuthRoleParameters(ctx, r.Client, sa, []string{r.policies.PolicyName(sa)})
	if err != nil {
		return fmt.Errorf("unable to build auth role parameters: %w", err)
	}
	// Create an auth role in vault
	if err := r.roles.WriteRole(ctx, sa, params); err != nil {
		return fmt.Errorf("unable to put auth role in vault: %w", err)
	}
	// Add finalizer if not present
	if r.useFinalizers && !util.HasFinalizer(sa) {
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
	if err := r.policies.DeletePolicy(ctx, sa); err != nil {
		return fmt.Errorf("unable to delete policy in vault: %w", err)
	}
	// Ensure the auth role is deleted in vault
	if err := r.roles.DeleteRole(ctx, sa); err != nil {
		return fmt.Errorf("unable to delete auth role in vault: %w", err)
	}
	// Remove the finalizer
	if err := util.RemoveFinalizer(ctx, r.Client, sa); err != nil {
		return fmt.Errorf("unable to remove finalizer from serviceaccount: %w", err)
	}
	return nil
}

func (r *ServiceAccountReconciler) getServiceAccountPolicy(ctx context.Context, sa *corev1.ServiceAccount) (string, error) {
	if util.HasAnnotation(sa, api.VaultInlinePolicyAnnotation) {
		return sa.GetAnnotations()[api.VaultInlinePolicyAnnotation], nil
	}
	name := sa.GetAnnotations()[api.VaultConfigMapPolicyAnnotation]
	var cm corev1.ConfigMap
	if err := r.Get(ctx, client.ObjectKey{Namespace: sa.GetNamespace(), Name: name}, &cm); err != nil {
		return "", fmt.Errorf("failed to get configmap: %w", err)
	}
	policy, ok := cm.Data[api.VaultPolicyKey]
	if !ok {
		return "", errors.New("configmap does not have a policy key")
	}
	return policy, nil
}
