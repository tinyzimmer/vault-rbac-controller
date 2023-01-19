package vault

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
)

var _ = Describe("Vault Policies", func() {
	var policies PolicyManager
	var object client.Object

	BeforeEach(func() {
		object = &corev1.ServiceAccount{}
		object.SetName("serviceaccount")
		object.SetNamespace("default")
		policies = NewPolicyManager()
		Expect(policies).ToNot(BeNil())
	})

	Describe("generating policy names", func() {

		When("the object has the policy name annotation", func() {
			BeforeEach(func() {
				object.SetAnnotations(map[string]string{
					api.VaultPolicyNameAnnotation: "test-policy",
				})
			})
			It("should return the annotation value", func() {
				Expect(policies.PolicyName(object)).To(Equal("test-policy"))
			})
		})

		When("the object does not have an annotation", func() {
			It("should return the default resource format", func() {
				Expect(policies.PolicyName(object)).To(Equal("default-serviceaccount"))
			})
		})

	})

	Describe("writing policies", func() {

		When("the policy is valid", func() {
			var err error
			BeforeEach(func() {
				err = policies.WritePolicy(context.Background(), object, "path \"secret/*\" { capabilities = [\"read\"] }")
			})
			It("should not error", func() {
				Expect(err).To(BeNil())
			})
			It("should write the policy to vault", func() {
				cli, err := NewClient()
				Expect(err).To(BeNil())
				policy, err := cli.Sys().GetPolicy("default-serviceaccount")
				Expect(err).To(BeNil())
				Expect(policy).To(Equal("path \"secret/*\" { capabilities = [\"read\"] }"))
			})
		})

		When("the policy is invalid", func() {
			It("should return an error", func() {
				Expect(policies.WritePolicy(context.Background(), object, "invalid")).ToNot(Succeed())
			})
		})

	})

	Describe("deleting policies", func() {

		When("the policy exists", func() {
			BeforeEach(func() {
				Expect(policies.WritePolicy(context.Background(), object, "path \"secret/*\" { capabilities = [\"read\"] }")).To(Succeed())
			})
			It("should delete the policy from vault", func() {
				Expect(policies.DeletePolicy(context.Background(), object)).To(Succeed())
			})
		})

		When("the policy does not exist", func() {
			It("should still succeed", func() {
				Expect(policies.DeletePolicy(context.Background(), object)).To(Succeed())
			})
		})

	})
})
