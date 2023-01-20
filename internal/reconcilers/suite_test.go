/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package reconcilers

import (
	"context"
	"errors"
	"testing"
	"time"

	testingi "github.com/mitchellh/go-testing-interface"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/hashicorp/go-hclog"
	k8sauth "github.com/hashicorp/vault-plugin-auth-kubernetes"
	vaultapi "github.com/hashicorp/vault/api"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	hashivault "github.com/hashicorp/vault/vault"

	"github.com/tinyzimmer/vault-rbac-controller/internal/vault"
)

var (
	timeout      = time.Second * 10
	interval     = time.Millisecond * 250
	env          *envtest.Environment
	cfg          *rest.Config
	k8sClient    client.Client
	mgr          ctrl.Manager
	vaultCluster *hashivault.TestCluster
	vaultClient  *vaultapi.Client
	envctx       context.Context
	cancel       context.CancelFunc
)

func TestReconcilers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconcilers Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	envctx, cancel = context.WithCancel(context.Background())
	var err error

	By("bootstrapping test environment")

	// Start test environment
	env = &envtest.Environment{}
	cfg, err = env.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	// Start test vault
	vaultCluster = setupTestVault(GinkgoT())
	vault.NewClient = func() (*vaultapi.Client, error) {
		return vaultCluster.Cores[0].Client, nil
	}
	vaultClient = vaultCluster.Cores[0].Client

	// Setup reconcilers
	mgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(mgr).ToNot(BeNil())
	Expect(SetupWithManager(mgr, &Options{
		AuthMount:     "kubernetes",
		UseFinalizers: true,
	})).To(Succeed())
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(envctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	// Setup k8s client
	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).ToNot(HaveOccurred())

	// Create namespaces for each suite
	Expect(k8sClient.Create(envctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "serviceaccount"},
	})).To(Succeed())
	Expect(k8sClient.Create(envctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "rolebinding"},
	})).To(Succeed())
	Expect(k8sClient.Create(envctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "role"},
	})).To(Succeed())

})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := env.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func VaultRole(ctx context.Context, path string) (*vaultapi.Secret, error) {
	path = "auth/kubernetes/role/" + path
	return vaultClient.Logical().ReadWithContext(ctx, path)
}

func VaultPolicy(ctx context.Context, name string) (string, error) {
	return vaultClient.Sys().GetPolicyWithContext(ctx, name)
}

func ObjectDeleted(ctx context.Context, obj client.Object) func() bool {
	return func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		return err != nil && client.IgnoreNotFound(err) == nil
	}
}

func EventOccurred(ctx context.Context, obj client.Object) func() bool {
	return func() bool {
		_, err := GetMostRecentEvent(ctx, obj)
		return err == nil
	}
}

func MostRecentEventReason(ctx context.Context, obj client.Object) (string, error) {
	event, err := GetMostRecentEvent(ctx, obj)
	if err != nil {
		return "", err
	}
	return event.Reason, nil
}

func GetMostRecentEvent(ctx context.Context, obj client.Object) (*eventsv1.Event, error) {
	log := GinkgoLogr.WithName("GetMostRecentEvent")
	log.Info("getting most recent event",
		"namespace", obj.GetNamespace(), "name", obj.GetName(), "uid", obj.GetUID())
	var eventList eventsv1.EventList
	err := k8sClient.List(ctx, &eventList, client.InNamespace(obj.GetNamespace()))
	if err != nil {
		return nil, err
	}
	var mostRecent *eventsv1.Event
	for _, e := range eventList.Items {
		if string(e.Regarding.UID) == string(obj.GetUID()) {
			log.Info("found event for object", "event", e.Name, "time", e.EventTime.Time)
			if mostRecent == nil || e.EventTime.Time.After(mostRecent.EventTime.Time) {
				mostRecent = &e
				continue
			}
		}
	}
	if mostRecent == nil {
		return nil, errors.New("no events found for object")
	}
	return mostRecent, nil
}

func setupTestVault(t testingi.T) *hashivault.TestCluster {
	t.Helper()

	cluster := hashivault.NewTestCluster(t, &hashivault.CoreConfig{
		CredentialBackends: map[string]logical.Factory{
			"kubernetes": k8sauth.Factory,
		},
	}, &hashivault.TestClusterOptions{
		NumCores:    1,
		HandlerFunc: vaulthttp.Handler,
		Logger: hclog.New(&hclog.LoggerOptions{
			Output: GinkgoWriter,
			Level:  hclog.Error,
		}),
	})
	cluster.Start()

	if err := cluster.Cores[0].Client.Sys().EnableAuthWithOptions("kubernetes", &vaultapi.EnableAuthOptions{
		Type: "kubernetes",
	}); err != nil {
		t.Fatal(err)
	}

	return cluster
}
