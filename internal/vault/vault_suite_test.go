/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vault

import (
	"os"
	"testing"

	testingi "github.com/mitchellh/go-testing-interface"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hashicorp/go-hclog"
	k8sauth "github.com/hashicorp/vault-plugin-auth-kubernetes"
	"github.com/hashicorp/vault/api"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
)

var (
	cluster *vault.TestCluster
)

func TestVault(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vault Suite")
}

var _ = BeforeSuite(func() {
	cluster = setupTestVault(GinkgoT())
	os.Setenv("VAULT_ADDR", cluster.Cores[0].Client.Address())
	os.Setenv("VAULT_TOKEN", cluster.RootToken)
	os.Setenv("VAULT_SKIP_VERIFY", "true")
})

var _ = AfterSuite(func() {
	cluster.Cleanup()
	os.Unsetenv("VAULT_ADDR")
	os.Unsetenv("VAULT_TOKEN")
	os.Unsetenv("VAULT_SKIP_VERIFY")
})

func setupTestVault(t testingi.T) *vault.TestCluster {
	t.Helper()

	cluster := vault.NewTestCluster(t, &vault.CoreConfig{
		CredentialBackends: map[string]logical.Factory{
			"kubernetes": k8sauth.Factory,
		},
	}, &vault.TestClusterOptions{
		NumCores:    1,
		HandlerFunc: vaulthttp.Handler,
		Logger: hclog.New(&hclog.LoggerOptions{
			Output: GinkgoWriter,
			Level:  hclog.Error,
		}),
	})
	cluster.Start()

	if err := cluster.Cores[0].Client.Sys().EnableAuthWithOptions("kubernetes", &api.EnableAuthOptions{
		Type: "kubernetes",
	}); err != nil {
		t.Fatal(err)
	}

	return cluster
}
