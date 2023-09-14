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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/edgeworx/vault-rbac-controller/internal/api"
	"github.com/edgeworx/vault-rbac-controller/internal/util"
	"github.com/edgeworx/vault-rbac-controller/internal/vault"
)

type ServiceAccountReconciler struct {
	client.Client

	recorder      record.EventRecorder
	policies      vault.PolicyManager
	roles         vault.RoleManager
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
		r.recorder.Event(&sa, corev1.EventTypeNormal, api.EventReasonIgnored, "ServiceAccount is ignored by the controller")
		return ctrl.Result{}, nil
	}

	if !vault.HasACLs(&sa) {
		log.Info("no vault rules found in serviceaccount, skipping")
		r.recorder.Event(&sa, corev1.EventTypeNormal, api.EventReasonIgnored, "ServiceAccount does not define any Vault ACLs")
		return ctrl.Result{}, nil
	}

	if sa.GetDeletionTimestamp() != nil {
		if err := r.reconcileDelete(ctx, &sa); err != nil {
			r.recorder.Event(&sa, corev1.EventTypeWarning, api.EventReasonError, err.Error())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.reconcileCreateUpdate(ctx, &sa); err != nil {
		r.recorder.Event(&sa, corev1.EventTypeWarning, api.EventReasonError, err.Error())
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ServiceAccountReconciler) reconcileCreateUpdate(ctx context.Context, sa *corev1.ServiceAccount) error {
	// Create the policy in vault
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
	if r.useFinalizers && !controllerutil.ContainsFinalizer(sa, api.ResourceFinalizer) {
		if err := addFinalizer(ctx, r.Client, sa); err != nil {
			return fmt.Errorf("unable to update serviceaccount with finalizer: %w", err)
		}
	}
	r.recorder.Event(sa, corev1.EventTypeNormal, api.EventReasonSynced, "ServiceAccount synced to Vault")
	return nil
}

func (r *ServiceAccountReconciler) reconcileDelete(ctx context.Context, sa *corev1.ServiceAccount) error {
	if !controllerutil.ContainsFinalizer(sa, api.ResourceFinalizer) {
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
	if err := removeFinalizer(ctx, r.Client, sa); err != nil {
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
