/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package main

import (
	"flag"
	"net/http"
	"os"
	"strings"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/edgeworx/vault-rbac-controller/internal/reconcilers"
	"github.com/edgeworx/vault-rbac-controller/internal/vault"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	version string
	commit  string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr             string
		enableLeaderElection    bool
		probeAddr               string
		useFinalizers           bool
		authMount               string
		namespaces              string
		excludeNamespaces       string
		includeSystemNamespaces bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&useFinalizers, "use-finalizers", false,
		"Ensure finalizers on resources to attempt to clean up on deletion.")
	flag.StringVar(&authMount, "auth-mount", "kubernetes", "The auth mount for the kubernetes auth method.")
	flag.StringVar(&namespaces, "namespaces", "", "The namespaces to watch for roles. If empty, all namespaces are watched.")
	flag.StringVar(&excludeNamespaces, "exclude-namespaces", "", "The namespaces to exclude from watching. If empty, no namespaces are excluded.")
	flag.BoolVar(&includeSystemNamespaces, "include-system-namespaces", false, "Include system namespaces in the watched namespaces.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("starting vault-rbac-controller", "version", version, "commit", commit)

	ctrlNamespaces := strings.Split(namespaces, ",")
	if len(ctrlNamespaces) == 1 && ctrlNamespaces[0] == "" {
		ctrlNamespaces = nil
	}
	excludedNamespaces := strings.Split(excludeNamespaces, ",")
	if len(excludedNamespaces) == 1 && excludedNamespaces[0] == "" {
		excludedNamespaces = nil
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "87065891.rbac.vault.hashicorp.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err = reconcilers.SetupWithManager(mgr, &reconcilers.Options{
		AuthMount:               authMount,
		UseFinalizers:           useFinalizers,
		Namespaces:              ctrlNamespaces,
		ExcludeNamespaces:       excludedNamespaces,
		IncludeSystemNamespaces: includeSystemNamespaces,
	}); err != nil {
		setupLog.Error(err, "unable to create controllers")
		os.Exit(1)
	}

	// Add ping check for readyz and healthz
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Add health check for vault communication on healthz
	if err := mgr.AddHealthzCheck("vault", func(_ *http.Request) error {
		cli, err := vault.NewClient()
		if err != nil {
			return err
		}
		_, err = cli.Sys().Health()
		return err
	}); err != nil {
		setupLog.Error(err, "unable to set up vault health check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
