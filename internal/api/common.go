/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package api

const (
	// VaultRBACControllerFinalizer is the finalizer added to resources managed by the controller.
	ResourceFinalizer = "vault-rbac-controller/finalizer"
	// VaultPolicyKey is the key in configmaps that contains the Vault policy.
	VaultPolicyKey = "policy.hcl"

	EventReasonIgnored = "Ignored"
	EventReasonSynced  = "Synced"
	EventReasonError   = "Error"
)
