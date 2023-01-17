package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
	"github.com/tinyzimmer/vault-rbac-controller/internal/util"
)

type PolicyManager interface {
	HasACLs() bool
	PolicyName() string
	WritePolicy(ctx context.Context) error
	DeletePolicy(ctx context.Context) error
}

func NewPolicyManager(client client.Client, obj client.Object) PolicyManager {
	switch obj := obj.(type) {
	case *rbacv1.Role:
		return newRolePolicyManager(obj)
	case *corev1.ServiceAccount:
		return newServiceAccountPolicyManager(client, obj)
	default:
		panic("unknown object type")
	}
}

type serviceAccountPolicyManager struct {
	client  client.Client
	svcacct *corev1.ServiceAccount
}

func newServiceAccountPolicyManager(client client.Client, svcacct *corev1.ServiceAccount) *serviceAccountPolicyManager {
	return &serviceAccountPolicyManager{client: client, svcacct: svcacct}
}

func (s *serviceAccountPolicyManager) HasACLs() bool {
	return util.HasAnnotation(s.svcacct, api.VaultInlinePolicyAnnotation) ||
		util.HasAnnotation(s.svcacct, api.VaultConfigMapPolicyAnnotation)
}

func (s *serviceAccountPolicyManager) PolicyName() string {
	return policyName(s.svcacct)
}

func (s *serviceAccountPolicyManager) WritePolicy(ctx context.Context) error {
	policy, err := s.getPolicy(ctx)
	if err != nil {
		return err
	}
	cli, err := NewClient()
	if err != nil {
		return err
	}
	return cli.Sys().PutPolicyWithContext(ctx, s.PolicyName(), policy)
}

func (s *serviceAccountPolicyManager) getPolicy(ctx context.Context) (string, error) {
	if util.HasAnnotation(s.svcacct, api.VaultInlinePolicyAnnotation) {
		return s.svcacct.GetAnnotations()[api.VaultInlinePolicyAnnotation], nil
	}
	name := s.svcacct.GetAnnotations()[api.VaultConfigMapPolicyAnnotation]
	var cm corev1.ConfigMap
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: s.svcacct.GetNamespace(), Name: name}, &cm); err != nil {
		return "", fmt.Errorf("failed to get configmap: %w", err)
	}
	p, ok := cm.Data[api.VaultPolicyKey]
	if !ok {
		return "", errors.New("configmap does not have a policy key")
	}
	return p, nil
}

func (s *serviceAccountPolicyManager) DeletePolicy(ctx context.Context) error {
	cli, err := NewClient()
	if err != nil {
		return err
	}
	return cli.Sys().DeletePolicyWithContext(ctx, s.PolicyName())
}

type rolePolicyManager struct {
	role  *rbacv1.Role
	rules []rbacv1.PolicyRule
}

func newRolePolicyManager(role *rbacv1.Role) *rolePolicyManager {
	return &rolePolicyManager{role: role, rules: filterRules(role.Rules)}
}

func (r *rolePolicyManager) HasACLs() bool {
	return len(r.rules) > 0
}

func (r *rolePolicyManager) PolicyName() string {
	return policyName(r.role)
}

func (r *rolePolicyManager) WritePolicy(ctx context.Context) error {
	policy, err := toJSONPolicyString(r.rules)
	if err != nil {
		return err
	}
	cli, err := NewClient()
	if err != nil {
		return err
	}
	return cli.Sys().PutPolicyWithContext(ctx, r.PolicyName(), policy)
}

func (r *rolePolicyManager) DeletePolicy(ctx context.Context) error {
	cli, err := NewClient()
	if err != nil {
		return err
	}
	return cli.Sys().DeletePolicyWithContext(ctx, r.PolicyName())
}

type policyJSON struct {
	Path map[string]pathPolicy `json:"path"`
}

type pathPolicy struct {
	Capabilities []string `json:"capabilities"`
}

func toJSONPolicyString(rules []rbacv1.PolicyRule) (string, error) {
	pol := policyJSON{Path: make(map[string]pathPolicy)}
	for _, rule := range rules {
		for _, res := range rule.Resources {
			pol.Path[res] = pathPolicy{Capabilities: rule.Verbs}
		}
	}
	out, err := json.MarshalIndent(pol, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func policyName(obj client.Object) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		if name, ok := annotations[api.VaultPolicyNameAnnotation]; ok {
			return name
		}
	}
	return util.DefaultResourceFormat(obj.GetNamespace(), obj.GetName())
}

const vaultAPIGroup = "vault.hashicorp.com"

func filterRules(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	var out []rbacv1.PolicyRule
	for _, rule := range rules {
		if len(rule.APIGroups) != 0 && rule.APIGroups[0] == vaultAPIGroup {
			out = append(out, rule)
		}
	}
	return out
}
