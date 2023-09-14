/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vault

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/edgeworx/vault-rbac-controller/internal/api"
	"github.com/edgeworx/vault-rbac-controller/internal/util"
)

type PolicyManager interface {
	PolicyName(client.Object) string
	WritePolicy(context.Context, client.Object, string) error
	DeletePolicy(context.Context, client.Object) error
}

func NewPolicyManager() PolicyManager {
	return &policyManager{}
}

type policyManager struct{}

func (p *policyManager) PolicyName(object client.Object) string {
	if annotations := object.GetAnnotations(); annotations != nil {
		if name, ok := annotations[api.VaultPolicyNameAnnotation]; ok {
			return name
		}
	}
	return util.DefaultResourceFormat(object.GetNamespace(), object.GetName())
}

func (p *policyManager) WritePolicy(ctx context.Context, object client.Object, policy string) error {
	policyName := p.PolicyName(object)
	cli, err := NewClient()
	if err != nil {
		return fmt.Errorf("failed to get vault client: %w", err)
	}
	if err := cli.Sys().PutPolicyWithContext(ctx, policyName, policy); err != nil {
		return fmt.Errorf("failed to write policy to vault: %w", err)
	}
	return nil
}

func (p *policyManager) DeletePolicy(ctx context.Context, object client.Object) error {
	cli, err := NewClient()
	if err != nil {
		return fmt.Errorf("failed to get vault client: %w", err)
	}
	if err := cli.Sys().DeletePolicyWithContext(ctx, p.PolicyName(object)); err != nil {
		return fmt.Errorf("failed to delete policy from vault: %w", err)
	}
	return nil
}
