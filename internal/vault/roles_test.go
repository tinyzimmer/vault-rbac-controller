package vault

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
)

var _ = Describe("Vault Roles", func() {
	var roles RoleManager
	var object client.Object

	BeforeEach(func() {
		object = &corev1.ServiceAccount{}
		object.SetName("serviceaccount")
		object.SetNamespace("default")
		roles = NewRoleManager("kubernetes")
		Expect(roles).ToNot(BeNil())
	})

	Describe("generating connection role names", func() {

		When("the object has the role name annotation", func() {
			BeforeEach(func() {
				object.SetAnnotations(map[string]string{
					api.VaultRoleNameAnnotation: "test-role",
				})
			})
			It("should return the annotation value", func() {
				Expect(roles.RoleName(object)).To(Equal("test-role"))
			})
		})

		When("the object does not have an annotation", func() {
			It("should return the default resource format", func() {
				Expect(roles.RoleName(object)).To(Equal("default-serviceaccount"))
			})
		})

	})

	Describe("writing connection roles", func() {

		When("the role is valid", func() {
			var err error
			BeforeEach(func() {
				err = roles.WriteRole(context.Background(), object, map[string]any{
					"bound_service_account_names":      []string{"default"},
					"bound_service_account_namespaces": []string{"serviceaccount"},
					"policies":                         []string{"test-policy"},
				})
			})
			It("should not error", func() {
				Expect(err).To(BeNil())
			})
			It("should write the role to vault", func() {
				cli, err := NewClient()
				Expect(err).To(BeNil())
				role, err := cli.Logical().Read("auth/kubernetes/role/default-serviceaccount")
				Expect(err).To(BeNil())
				Expect(role).ToNot(BeNil())
				Expect(role.Data["bound_service_account_names"]).To(Equal([]interface{}{"default"}))
				Expect(role.Data["bound_service_account_namespaces"]).To(Equal([]interface{}{"serviceaccount"}))
				Expect(role.Data["policies"]).To(Equal([]interface{}{"test-policy"}))
			})
		})

		When("the role is invalid", func() {
			var err error
			BeforeEach(func() {
				object.SetName("")
				object.SetNamespace("")
				err = roles.WriteRole(context.Background(), object, nil)
			})
			It("should error", func() {
				Expect(err).ToNot(BeNil())
			})
		})

	})

	Describe("deleting connection roles", func() {

		When("the role exists", func() {
			var err error
			BeforeEach(func() {
				err = roles.WriteRole(context.Background(), object, map[string]any{
					"bound_service_account_names":      []string{"default"},
					"bound_service_account_namespaces": []string{"serviceaccount"},
					"policies":                         []string{"test-policy"},
				})
				Expect(err).To(BeNil())
				err = roles.DeleteRole(context.Background(), object)
			})
			It("should not error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("the role does not exists", func() {
			var err error
			BeforeEach(func() {
				err = roles.DeleteRole(context.Background(), object)
			})
			It("should not error", func() {
				Expect(err).To(BeNil())
			})
		})

	})
})
