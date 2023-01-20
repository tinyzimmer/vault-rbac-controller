package reconcilers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
)

var _ = Describe("Roles Reconciler", func() {

	var (
		role            *rbacv1.Role
		vaultPolicyName = "role-role"
	)

	BeforeEach(func() {
		role = &rbacv1.Role{}
		role.SetName("role")
		role.SetNamespace("role")
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"vault.hashicorp.com"},
				Resources: []string{"secret/*"},
				Verbs:     []string{"read"},
			},
		}
	})

	When("Reconciling", func() {

		// Create the role
		JustBeforeEach(func(ctx SpecContext) {
			Expect(k8sClient.Create(ctx, role)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(role), role)).To(Succeed())
		})

		// Delete the role
		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, role)).To(Succeed())
			Eventually(ObjectDeleted(ctx, role), timeout, interval).Should(BeTrue())
		})

		Context("a Role that does not have any Vault ACLs", func() {

			BeforeEach(func() {
				role.Rules = nil
			})

			It("should emit an Ignored event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, role), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, role)).To(Equal(api.EventReasonIgnored))
			})

			It("should not create a policy in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, role), timeout, interval).Should(BeTrue())
				Expect(VaultPolicy(ctx, vaultPolicyName)).To(BeEmpty())
			})

		})

		Context("a Role that has Vault ACLs", func() {

			It("should emit a Synced event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, role), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, role)).To(Equal(api.EventReasonSynced))
			})

			It("should create a policy in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, role), timeout, interval).Should(BeTrue())
				Expect(VaultPolicy(ctx, vaultPolicyName)).ToNot(BeEmpty())
			})

			It("should have the finalizer applied", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, role), timeout, interval).Should(BeTrue())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(role), role)).To(Succeed())
				Expect(role.GetFinalizers()).To(ContainElement(api.ResourceFinalizer))
			})

		})

	})

	When("cleaning up a Role", func() {

		BeforeEach(func(ctx SpecContext) {
			// Create the role
			Expect(k8sClient.Create(ctx, role)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(role), role)).To(Succeed())
			Eventually(EventOccurred(ctx, role), timeout, interval).Should(BeTrue())
			// Check that the policy was created
			Expect(VaultPolicy(ctx, vaultPolicyName)).ToNot(BeEmpty())
		})

		When("the Role is deleted", func() {

			JustBeforeEach(func(ctx SpecContext) {
				Expect(k8sClient.Delete(ctx, role)).To(Succeed())
				Eventually(ObjectDeleted(ctx, role), timeout, interval).Should(BeTrue())
			})

			It("should remove the policy from vault", func(ctx SpecContext) {
				Expect(VaultPolicy(ctx, vaultPolicyName)).To(BeEmpty())
			})

		})
	})
})
