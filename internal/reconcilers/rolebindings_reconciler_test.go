/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package reconcilers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/edgeworx/vault-rbac-controller/internal/api"
)

var _ = Describe("RoleBindings Reconciler", func() {

	var (
		rb                   *rbacv1.RoleBinding
		rbRole               *rbacv1.Role
		vaultRoleBindingName = "rolebinding-rolebinding"
	)

	// Set up boilerplate roles/rolebindings
	BeforeEach(func() {
		rbRole = &rbacv1.Role{}
		rb = &rbacv1.RoleBinding{}
		rb.SetName("rolebinding")
		rb.SetNamespace("rolebinding")
		rbRole.SetName("rolebinding-role")
		rbRole.SetNamespace("rolebinding")
		rbRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"vault.hashicorp.com"},
				Resources: []string{"secret/*"},
				Verbs:     []string{"read"},
			},
		}
		rb.RoleRef = rbacv1.RoleRef{
			Kind: "Role",
			Name: rbRole.GetName(),
		}
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: "rolebinding",
			},
		}
	})

	When("Reconciling", func() {

		// Create the roles/rolebindings
		JustBeforeEach(func(ctx SpecContext) {
			Expect(k8sClient.Create(ctx, rbRole)).To(Succeed())
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb), rb)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rbRole), rbRole)).To(Succeed())
		})

		// Delete the roles/rolebindings
		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, rbRole)).To(Succeed())
			Expect(k8sClient.Delete(ctx, rb)).To(Succeed())
			Eventually(ObjectDeleted(ctx, rbRole), timeout, interval).Should(BeTrue())
			Eventually(ObjectDeleted(ctx, rb), timeout, interval).Should(BeTrue())
		})

		Context("a RoleBinding that does not have the bind annotation", func() {

			It("should emit an Ignored event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, rb), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, rb)).To(Equal(api.EventReasonIgnored))
			})

			It("should not create a role in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, rb), timeout, interval).Should(BeTrue())
				Expect(VaultRole(ctx, vaultRoleBindingName)).To(BeNil())
			})
		})

		Context("a RoleBinding that has the correct annotations", func() {

			BeforeEach(func() {
				rb.SetAnnotations(map[string]string{
					api.VaultRoleBindAnnotation: "true",
				})
			})

			It("should emit a Synced event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, rb), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, rb)).To(Equal(api.EventReasonSynced))
			})

			It("should create a role in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, rb), timeout, interval).Should(BeTrue())
				Expect(VaultRole(ctx, vaultRoleBindingName)).ToNot(BeNil())
			})

			It("should have the finalizer applied", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, rb), timeout, interval).Should(BeTrue())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb), rb)).To(Succeed())
				Expect(rb.GetFinalizers()).To(ContainElement(api.ResourceFinalizer))
			})
		})

	})

	When("cleaning up rolebindings", func() {

		BeforeEach(func(ctx SpecContext) {
			// Create the roles/rolebindings
			rb.SetAnnotations(map[string]string{
				api.VaultRoleBindAnnotation: "true",
			})
			Expect(k8sClient.Create(ctx, rbRole)).To(Succeed())
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())
			Eventually(EventOccurred(ctx, rb), timeout, interval).Should(BeTrue())
			// Ensure the role is created in vault
			Expect(VaultRole(ctx, vaultRoleBindingName)).ToNot(BeNil())
		})

		When("the rolebinding is deleted", func() {

			JustBeforeEach(func(ctx SpecContext) {
				Expect(k8sClient.Delete(ctx, rb)).To(Succeed())
				Eventually(ObjectDeleted(ctx, rb), timeout, interval).Should(BeTrue())
			})

			It("should remove the role from vault", func(ctx SpecContext) {
				Expect(VaultRole(ctx, vaultRoleBindingName)).To(BeNil())
			})
		})

	})
})
