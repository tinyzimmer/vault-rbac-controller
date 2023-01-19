package vault

import (
	testing "github.com/mitchellh/go-testing-interface"

	k8sauth "github.com/hashicorp/vault-plugin-auth-kubernetes"
	"github.com/hashicorp/vault/api"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
)

func setupTestVault(t testing.T) *vault.TestCluster {
	t.Helper()

	cluster := vault.NewTestCluster(t, &vault.CoreConfig{
		CredentialBackends: map[string]logical.Factory{
			"kubernetes": k8sauth.Factory,
		},
	}, &vault.TestClusterOptions{
		HandlerFunc: vaulthttp.Handler,
	})
	cluster.Start()

	if err := cluster.Cores[0].Client.Sys().EnableAuthWithOptions("kubernetes", &api.EnableAuthOptions{
		Type: "kubernetes",
	}); err != nil {
		t.Fatal(err)
	}

	return cluster
}
