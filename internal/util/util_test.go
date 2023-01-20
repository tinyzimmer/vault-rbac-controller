/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package util

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tinyzimmer/vault-rbac-controller/internal/api"
)

func TestDefaultResourceFormat(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		{
			name:      "test",
			namespace: "test",
			want:      "test-test",
		},
		{
			name:      "hello",
			namespace: "world",
			want:      "world-hello",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultResourceFormat(tt.namespace, tt.name); got != tt.want {
				t.Errorf("DefaultResourceFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIgnoredServiceAccount(t *testing.T) {
	tt := []struct {
		object *corev1.ServiceAccount
		want   bool
	}{
		{
			object: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ignored",
				},
			},
			want: true,
		},
		{
			object: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-bound",
					Annotations: map[string]string{
						api.VaultRoleBindAnnotation: "true",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tt {
		t.Run(tt.object.GetName(), func(t *testing.T) {
			if got := IsIgnoredServiceAccount(tt.object); got != tt.want {
				t.Errorf("IsIgnoredServiceAccount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIgnoredRoleBinding(t *testing.T) {
	tt := []struct {
		object *rbacv1.RoleBinding
		want   bool
	}{
		{
			object: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ignored",
				},
			},
			want: true,
		},
		{
			object: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-bound",
					Annotations: map[string]string{
						api.VaultRoleBindAnnotation: "true",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tt {
		t.Run(tt.object.GetName(), func(t *testing.T) {
			if got := IsIgnoredRoleBinding(tt.object); got != tt.want {
				t.Errorf("IsIgnoredRoleBinding() = %v, want %v", got, tt.want)
			}
		})
	}
}
