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

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	osconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

var k8sClient client.Client
var testEnv *envtest.Environment
var k8sManagerCancelFn context.CancelFunc

type mockImageProvider struct {
}

func (d *mockImageProvider) TagExists(_ string) bool {
	return false
}

func (d *mockImageProvider) SetImageSpec(*mellanoxcomv1alpha1.ImageSpec) {}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	testSetupLog := logf.Log.WithName("test-log").WithName("setup")

	// Go to project root directory
	er := os.Chdir("..")
	Expect(er).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("config", "crd", "bases"), filepath.Join("hack", "crds")},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = mellanoxcomv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = netattdefv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = osconfigv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: namespaceName,
	}}
	err = k8sClient.Create(context.Background(), ns)
	Expect(err).NotTo(HaveOccurred())

	// Start controllers
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
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

	go func() {
		defer GinkgoRecover()
		var ctx context.Context
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
