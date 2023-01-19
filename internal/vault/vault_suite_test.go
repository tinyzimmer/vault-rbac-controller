package vault

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hashicorp/vault/api"
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
