/*
Copyright 2022 NVIDIA CORPORATION & AFFILIATES

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

package upgrade_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/Mellanox/network-operator/pkg/upgrade"
	"github.com/Mellanox/network-operator/pkg/upgrade/mocks"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var k8sInterface kubernetes.Interface
var testEnv *envtest.Environment
var log logr.Logger
var nodeUpgradeStateProvider mocks.NodeUpgradeStateProvider
var drainManager mocks.DrainManager
var podDeleteManager mocks.PodDeleteManager
var uncordonManager mocks.UncordonManager

var createdObjects []client.Object

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sInterface, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sInterface).NotTo(BeNil())

	log = ctrl.Log.WithName("upgradeSuitTest")

	nodeUpgradeStateProvider = mocks.NodeUpgradeStateProvider{}
	nodeUpgradeStateProvider.
		On("ChangeNodeUpgradeState", mock.Anything, mock.Anything, mock.Anything).
		Return(func(ctx context.Context, node *corev1.Node, newNodeState string) error {
			node.Annotations[upgrade.UpgradeStateAnnotation] = newNodeState
			return nil
		})

	drainManager = mocks.DrainManager{}
	drainManager.
		On("ScheduleNodesDrain", mock.Anything, mock.Anything).
		Return(nil)
	podDeleteManager = mocks.PodDeleteManager{}
	podDeleteManager.
		On("SchedulePodsRestart", mock.Anything, mock.Anything).
		Return(nil)
	uncordonManager = mocks.UncordonManager{}
	uncordonManager.
		On("CordonOrUncordonNode", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	createdObjects = nil
})

var _ = AfterEach(func() {
	for i := range createdObjects {
		r := createdObjects[i]
		key := client.ObjectKeyFromObject(r)
		err := k8sClient.Get(context.TODO(), key, r)
		if err == nil {
			Expect(k8sClient.Delete(context.TODO(), r)).To(Succeed())
		}

		_, isNamespace := r.(*corev1.Namespace)
		if !isNamespace {
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), key, r)
			}).Should(HaveOccurred())
		}
	}
})

func createNamespace(name string) *corev1.Namespace {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := k8sClient.Create(context.TODO(), namespace)
	Expect(err).NotTo(HaveOccurred())
	createdObjects = append(createdObjects, namespace)
	return namespace
}

func createPod(name, namespace string) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "test-container", Image: "test-image"}}}}
	err := k8sClient.Create(context.TODO(), pod)
	Expect(err).NotTo(HaveOccurred())
	createdObjects = append(createdObjects, pod)
	return pod
}

func createNode(name string) *corev1.Node {
	node := &corev1.Node{}
	node.Name = name
	err := k8sClient.Create(context.TODO(), node)
	Expect(err).NotTo(HaveOccurred())
	createdObjects = append(createdObjects, node)
	return node
}

func deleteObj(obj client.Object) {
	Expect(k8sClient.Delete(context.TODO(), obj)).To(BeNil())
}

func getNodeUpgradeState(node *corev1.Node) string {
	return node.Annotations[upgrade.UpgradeStateAnnotation]
}
