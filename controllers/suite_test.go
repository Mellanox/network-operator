/*
Copyright 2021 NVIDIA

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

package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	consts "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	mock_orchestrator "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/orchestrator/mock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	osconfigv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	mellanoxcomv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	timeout       = time.Second * 10
	interval      = time.Millisecond * 250
	namespaceName = "nvidia-network-operator"
)

var (
	ctx                context.Context
	k8sClient          client.Client
	k8sConfig          *rest.Config
	testEnv            *envtest.Environment
	k8sManager         manager.Manager
	k8sManagerCancelFn context.CancelFunc
)

type mockImageProvider struct {
}

func (d *mockImageProvider) TagExists(_ string) (bool, error) {
	return false, nil
}

func (d *mockImageProvider) SetImageSpec(*mellanoxcomv1alpha1.ImageSpec) {}

func setupDrainControllerWithManager(k8sManager manager.Manager,
	migrationCompletionChan chan struct{}) {
	t := GinkgoT()
	mockCtrl := gomock.NewController(t)
	orchestrator := mock_orchestrator.NewMockInterface(mockCtrl)
	orchestrator.EXPECT().Flavor().Return(consts.ClusterFlavorDefault).AnyTimes()
	orchestrator.EXPECT().ClusterType().Return(consts.ClusterTypeKubernetes).AnyTimes()
	orchestrator.EXPECT().BeforeDrainNode(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	orchestrator.EXPECT().AfterCompleteDrainNode(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

	drainKClient, err := client.New(k8sConfig, client.Options{
		Scheme: scheme.Scheme,
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&sriovnetworkv1.SriovNetworkNodeState{},
				&corev1.Node{},
				&mcfgv1.MachineConfigPool{},
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sConfig).ToNot(BeNil())
	drainController, err := NewDrainReconcileController(drainKClient,
		k8sConfig,
		k8sManager.GetScheme(),
		k8sManager.GetEventRecorderFor("operator"),
		orchestrator,
		migrationCompletionChan,
		k8sManager.GetLogger().WithValues("Function", "Drain"))
	Expect(err).ToNot(HaveOccurred())
	err = drainController.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	var err error
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	testSetupLog := logf.Log.WithName("test-log").WithName("setup")

	// Go to project root directory
	er := os.Chdir("..")
	Expect(er).NotTo(HaveOccurred())

	Expect(os.Setenv("DRAIN_CONTROLLER_REQUESTOR_NAMESPACE", drainRequestorNS)).NotTo(HaveOccurred())
	Expect(os.Setenv("DRAIN_CONTROLLER_REQUESTOR_ID", drainRequestorID)).NotTo(HaveOccurred())
	Expect(os.Setenv("DRAIN_CONTROLLER_NODE_MAINTENANCE_PREFIX", testNodeMaintenancePrefix)).NotTo(HaveOccurred())
	Expect(os.Setenv("DRAIN_CONTROLLER_SRIOV_NODE_STATE_NAMESPACE", namespaceName)).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("config", "crd", "bases"),
			filepath.Join("hack", "crds"),
			filepath.Join("deployment", "network-operator", "charts", "maintenance-operator-chart", "crds"),
			filepath.Join("deployment", "network-operator", "charts", "sriov-network-operator", "crds"),
		},
	}

	k8sConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sConfig).NotTo(BeNil())

	err = mellanoxcomv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = netattdefv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = osconfigv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = maintenancev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = sriovnetworkv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme
	k8sClient, err = client.New(k8sConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: namespaceName,
	}}
	err = k8sClient.Create(context.Background(), ns)
	Expect(err).NotTo(HaveOccurred())

	// Start controllers
	k8sManager, err = ctrl.NewManager(k8sConfig, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	migrationCompletionChan := make(chan struct{})
	close(migrationCompletionChan)

	err = (&HostDeviceNetworkReconciler{
		Client:      k8sManager.GetClient(),
		Scheme:      k8sManager.GetScheme(),
		MigrationCh: migrationCompletionChan,
	}).SetupWithManager(k8sManager, testSetupLog)
	Expect(err).ToNot(HaveOccurred())

	err = (&IPoIBNetworkReconciler{
		Client:      k8sManager.GetClient(),
		Scheme:      k8sManager.GetScheme(),
		MigrationCh: migrationCompletionChan,
	}).SetupWithManager(k8sManager, testSetupLog)
	Expect(err).ToNot(HaveOccurred())

	err = (&MacvlanNetworkReconciler{
		Client:      k8sManager.GetClient(),
		Scheme:      k8sManager.GetScheme(),
		MigrationCh: migrationCompletionChan,
	}).SetupWithManager(k8sManager, testSetupLog)
	Expect(err).ToNot(HaveOccurred())

	clusterTypeProvider, err := clustertype.NewProvider(context.Background(), k8sClient)
	Expect(err).NotTo(HaveOccurred())
	staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "/opt/cni/bin"})

	err = (&NicClusterPolicyReconciler{
		Client:                   k8sManager.GetClient(),
		Scheme:                   k8sManager.GetScheme(),
		ClusterTypeProvider:      clusterTypeProvider,
		StaticConfigProvider:     staticConfigProvider,
		MigrationCh:              migrationCompletionChan,
		DocaDriverImagesProvider: &mockImageProvider{},
	}).SetupWithManager(k8sManager, testSetupLog)
	Expect(err).ToNot(HaveOccurred())

	setupDrainControllerWithManager(k8sManager, migrationCompletionChan)

	go func() {
		defer GinkgoRecover()

		ctx, k8sManagerCancelFn = context.WithCancel(ctrl.SetupSignalHandler())
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if k8sManagerCancelFn != nil {
		k8sManagerCancelFn()
	}
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
