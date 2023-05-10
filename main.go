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

package main

import (
	"flag"
	"os"

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	osconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	mellanoxcomv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/controllers"
	"github.com/Mellanox/network-operator/pkg/migrate"
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
	// +kubebuilder:scaffold:scheme
}

func setupCRDControllers(mgr ctrl.Manager) error {
	ctrLog := setupLog.WithName("controller")
	if err := (&controllers.NicClusterPolicyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, ctrLog.WithName("NicClusterPolicy")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NicClusterPolicy")
		return err
	}
	if err := (&controllers.MacvlanNetworkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, ctrLog.WithName("MacVlanNetwork")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MacvlanNetwork")
		return err
	}
	if err := (&controllers.HostDeviceNetworkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, ctrLog.WithName("HostDeviceNetwork")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HostDeviceNetwork")
		return err
	}
	if err := (&controllers.IPoIBNetworkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
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
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
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

	// run migration logic before controllers start
	if err := migrate.Migrate(stopCtx, setupLog.WithName("migrate"), directClient); err != nil {
		setupLog.Error(err, "failed to run migration logic")
		os.Exit(1)
	}

	err = setupCRDControllers(mgr)
	if err != nil {
		os.Exit(1)
	}

	upgrade.SetDriverName("ofed")

	upgradeLogger := ctrl.Log.WithName("controllers").WithName("Upgrade")

	nodeUpgradeStateProvider := upgrade.NewNodeUpgradeStateProvider(
		mgr.GetClient(), upgradeLogger.WithName("nodeUpgradeStateProvider"), nil)

	clusterUpdateStateManager, err := upgrade.NewClusterUpgradeStateManager(
		upgradeLogger.WithName("clusterUpgradeManager"), config.GetConfigOrDie(), nil)

	if err != nil {
		setupLog.Error(err, "unable to create new ClusterUpdateStateManager", "controller", "Upgrade")
		os.Exit(1)
	}

	if err = (&controllers.UpgradeReconciler{
		Client:                   mgr.GetClient(),
		Scheme:                   mgr.GetScheme(),
		StateManager:             clusterUpdateStateManager,
		NodeUpgradeStateProvider: nodeUpgradeStateProvider,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Upgrade")
		os.Exit(1)
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

	setupLog.Info("starting manager")
	if err := mgr.Start(stopCtx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
