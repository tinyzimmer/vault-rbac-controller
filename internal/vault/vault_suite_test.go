package vault

import (
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
	NewClient = func() (*api.Client, error) {
		return cluster.Cores[0].Client, nil
	}
})

var _ = AfterSuite(func() {
	cluster.Cleanup()
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
