/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package main performs setup and runs the controller manager.
package main

import (
	"context"
	"flag"
	"os"

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	osconfigv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	mellanoxcomv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/api/v1alpha1/validator"
	"github.com/Mellanox/network-operator/controllers"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/migrate"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
	"github.com/Mellanox/network-operator/version"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(mellanoxcomv1alpha1.AddToScheme(scheme))
	utilruntime.Must(netattdefv1.AddToScheme(scheme))
	utilruntime.Must(osconfigv1.AddToScheme(scheme))
	utilruntime.Must(imagev1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func setupWebhookControllers(mgr ctrl.Manager) error {
	if os.Getenv("SKIP_VALIDATIONS") == "true" {
		setupLog.Info("disabling admission controller validations")
		validator.DisableValidations()
	}
	if err := validator.SetupHostDeviceNetworkWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HostDeviceNetwork")
		return err
	}
	if err := validator.SetupNicClusterPolicyWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "NicClusterPolicy")

		return err
	}
	return nil
}

func setupCRDControllers(ctx context.Context, c client.Client, mgr ctrl.Manager, migrationChan chan struct{}) error {
	ctrLog := setupLog.WithName("controller")
	clusterTypeProvider, err := clustertype.NewProvider(ctx, c)
	if err != nil {
		setupLog.Error(err, "unable to create cluster type provider")
		return err
	}

	cniBinDir := os.Getenv("CNI_BIN_DIR")
	staticInfoProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: cniBinDir})

	docaImagesProvider := docadriverimages.NewProvider(ctx, c)

	if err := (&controllers.NicClusterPolicyReconciler{
		Client:                   mgr.GetClient(),
		Scheme:                   mgr.GetScheme(),
		ClusterTypeProvider:      clusterTypeProvider, // we want to cache information about the cluster type
		StaticConfigProvider:     staticInfoProvider,
		MigrationCh:              migrationChan,
		DocaDriverImagesProvider: docaImagesProvider,
	}).SetupWithManager(mgr, ctrLog.WithName("NicClusterPolicy")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NicClusterPolicy")
		return err
	}
	if err := (&controllers.MacvlanNetworkReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		MigrationCh: migrationChan,
	}).SetupWithManager(mgr, ctrLog.WithName("MacVlanNetwork")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MacvlanNetwork")
		return err
	}
	if err := (&controllers.HostDeviceNetworkReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		MigrationCh: migrationChan,
	}).SetupWithManager(mgr, ctrLog.WithName("HostDeviceNetwork")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HostDeviceNetwork")
		return err
	}
	if err := (&controllers.IPoIBNetworkReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		MigrationCh: migrationChan,
	}).SetupWithManager(mgr, ctrLog.WithName("IPoIBNetwork")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IPoIBNetwork")
		return err
	}
	return nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	stopCtx := ctrl.SetupSignalHandler()

	clientConf := ctrl.GetConfigOrDie()

	mgr, err := ctrl.NewManager(clientConf, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "12620820.mellanox.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	directClient, err := client.New(clientConf,
		client.Options{Scheme: mgr.GetScheme(), Mapper: mgr.GetRESTMapper()})
	if err != nil {
		setupLog.Error(err, "failed to create direct client")
		os.Exit(1)
	}

	migrationCompletionChan := make(chan struct{})
	m := migrate.Migrator{
		K8sClient:      directClient,
		MigrationCh:    migrationCompletionChan,
		LeaderElection: enableLeaderElection,
		Logger:         ctrl.Log.WithName("Migrator"),
	}
	err = mgr.Add(&m)
	if err != nil {
		setupLog.Error(err, "failed to add Migrator to the Manager")
		os.Exit(1)
	}

	err = setupCRDControllers(stopCtx, directClient, mgr, migrationCompletionChan)
	if err != nil {
		os.Exit(1)
	}

	err = setupUpgradeController(mgr, migrationCompletionChan)
	if err != nil {
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") == "true" {
		if err := setupWebhookControllers(mgr); err != nil {
			os.Exit(1)
		}
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "version", version.Version, "commit", version.Commit, "buildDate", version.Date)
	if err := mgr.Start(stopCtx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupUpgradeController(mgr ctrl.Manager, migrationChan chan struct{}) error {
	upgrade.SetDriverName("ofed")

	upgradeLogger := ctrl.Log.WithName("controllers").WithName("Upgrade")

	clusterUpdateStateManager, err := upgrade.NewClusterUpgradeStateManager(
		upgradeLogger.WithName("clusterUpgradeManager"), config.GetConfigOrDie(), nil)

	if err != nil {
		setupLog.Error(err, "unable to create new ClusterUpdateStateManager", "controller", "Upgrade")
		return err
	}

	if err = (&controllers.UpgradeReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		StateManager: clusterUpdateStateManager,
		MigrationCh:  migrationChan,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Upgrade")
		return err
	}
	return nil
}
