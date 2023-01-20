/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package reconcilers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
)

var _ = Describe("ServiceAccounts Reconciler", func() {

	var (
		sa          *corev1.ServiceAccount
		vaultSaName = "serviceaccount-serviceaccount"
	)

	BeforeEach(func() {
		sa = &corev1.ServiceAccount{}
		sa.SetName("serviceaccount")
		sa.SetNamespace("serviceaccount")
	})

	When("Reconciling", func() {

		JustBeforeEach(func(ctx SpecContext) {
			Expect(k8sClient.Create(ctx, sa)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(k8sClient.Delete(ctx, sa)).To(Succeed())
			Eventually(ObjectDeleted(ctx, sa), timeout, interval).Should(BeTrue())
		})

		Context("a ServiceAccount that does not have the bind annotation", func() {

			It("should emit an Ignored event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, sa)).To(Equal(api.EventReasonIgnored))
			})

			It("should not create a policy in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultPolicy(ctx, vaultSaName)).To(BeEmpty())
			})

			It("should not create a role in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultRole(ctx, vaultSaName)).To(BeNil())
			})
		})

		Context("a ServiceAccount that has the bind annotation but no ACLs", func() {

			BeforeEach(func() {
				sa.Annotations = map[string]string{
					api.VaultRoleBindAnnotation: "true",
				}
			})

			It("should emit an Ignored event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, sa)).To(Equal(api.EventReasonIgnored))
			})

			It("should not create a policy in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultPolicy(ctx, vaultSaName)).To(BeEmpty())
			})

			It("should not create a role in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultRole(ctx, vaultSaName)).To(BeNil())
			})
		})

		Context("a ServiceAccount that has an inline policy", func() {

			var policy = `path "secret/data/*" { capabilities = ["read"] }`

			BeforeEach(func() {
				sa.Annotations = map[string]string{
					api.VaultRoleBindAnnotation:     "true",
					api.VaultInlinePolicyAnnotation: policy,
				}
			})

			It("should emit a Synced event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, sa)).To(Equal(api.EventReasonSynced))
			})

			It("should create a policy in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultPolicy(ctx, vaultSaName)).To(Equal(policy))
			})

			It("should create a role in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultRole(ctx, vaultSaName)).ToNot(BeNil())
			})

			It("should have the finalizer applied", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
				Expect(sa.GetFinalizers()).To(ContainElement(api.ResourceFinalizer))
			})
		})

		Context("a ServiceAccount that has a configmap policy", func() {

			var policy = `path "secret/data/*" { capabilities = ["create"] }`

			BeforeEach(func(ctx SpecContext) {
				sa.Annotations = map[string]string{
					api.VaultRoleBindAnnotation:        "true",
					api.VaultConfigMapPolicyAnnotation: "configmap-policy",
				}
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap-policy",
						Namespace: "serviceaccount",
					},
					Data: map[string]string{
						api.VaultPolicyKey: policy,
					},
				}
				Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(k8sClient.Delete(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap-policy",
						Namespace: "serviceaccount",
					},
				})).To(Succeed())
			})

			It("should emit a Synced event", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(MostRecentEventReason(ctx, sa)).To(Equal(api.EventReasonSynced))
			})

			It("should create a policy in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultPolicy(ctx, vaultSaName)).To(Equal(policy))
			})

			It("should create a role in vault", func(ctx SpecContext) {
				Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
				Expect(VaultRole(ctx, vaultSaName)).ToNot(BeNil())
			})
		})

	})

	When("cleaning up a ServiceAccount", func() {

		BeforeEach(func(ctx SpecContext) {
			// Create the ServiceAccount
			sa.SetAnnotations(map[string]string{
				api.VaultRoleBindAnnotation:     "true",
				api.VaultInlinePolicyAnnotation: `path "secret/data/*" { capabilities = ["read"] }`,
			})
			Expect(k8sClient.Create(ctx, sa)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
			Eventually(EventOccurred(ctx, sa), timeout, interval).Should(BeTrue())
			// Ensure resources are created
			Expect(VaultPolicy(ctx, vaultSaName)).ToNot(BeEmpty())
			Expect(VaultRole(ctx, vaultSaName)).ToNot(BeNil())
		})

		When("the ServiceAccount is deleted", func() {

			JustBeforeEach(func(ctx SpecContext) {
				Expect(k8sClient.Delete(ctx, sa)).To(Succeed())
				Eventually(ObjectDeleted(ctx, sa), timeout, interval).Should(BeTrue())
			})

			It("should remove the policy from vault", func(ctx SpecContext) {
				Expect(VaultPolicy(ctx, vaultSaName)).To(BeEmpty())
			})

			It("should remove the role from vault", func(ctx SpecContext) {
				Expect(VaultRole(ctx, vaultSaName)).To(BeNil())
			})
		})
	})
})
