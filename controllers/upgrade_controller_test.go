/*
2023 NVIDIA CORPORATION & AFFILIATES

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
	goctx "context"
	"fmt"
	"os"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
)

var _ = Describe("Upgrade Controller", func() {
	var (
		cr                        mellanoxv1alpha1.NicClusterPolicy
		clusterUpdateStateManager upgrade.ClusterUpgradeStateManager
		upgradeReconciler         *UpgradeReconciler
		nodes                     []*corev1.Node
		testCtx                   goctx.Context
	)
	BeforeEach(func() {
		testCtx = goctx.TODO()
		cr = mellanoxv1alpha1.NicClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consts.NicClusterPolicyResourceName,
				Namespace: "",
			},
			Spec: mellanoxv1alpha1.NicClusterPolicySpec{
				OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image:      "doca-ofed",
						Repository: "nvcr.io/nvidia",
					},
					OfedUpgradePolicy: &mellanoxv1alpha1.DriverUpgradePolicySpec{
						AutoUpgrade: true,
					},
				},
			},
		}

		Expect(cr).NotTo(Equal(nil))
		err := k8sClient.Create(testCtx, &cr)
		Expect(err).NotTo(HaveOccurred())

		upgrade.SetDriverName("ofed")
		nodes = createTestNodes(3)
		for _, node := range nodes {
			node.Labels[upgrade.GetUpgradeStateLabelKey()] = "test-state"
			err := k8sClient.Create(testCtx, node)
			Expect(err).NotTo(HaveOccurred())
		}

		// Initialize clusterUpdateStateManager
		requestorOpts := upgrade.GetRequestorOptsFromEnvs()
		clusterUpdateStateManager, err = upgrade.NewClusterUpgradeStateManager(
			log.FromContext(testCtx),
			k8sConfig,
			nil,
			upgrade.StateOptions{Requestor: requestorOpts})
		Expect(err).NotTo(HaveOccurred())

		migrationCompletionChan := make(chan struct{})
		close(migrationCompletionChan)
		upgradeReconciler = &UpgradeReconciler{
			Client:       k8sClient,
			Scheme:       k8sClient.Scheme(),
			StateManager: clusterUpdateStateManager,
			MigrationCh:  migrationCompletionChan,
		}

	})
	AfterEach(func() {
		err := k8sClient.Delete(testCtx, &cr)
		Expect(err).NotTo(HaveOccurred())
		// delete nodes once test is done
		for _, node := range nodes {
			err := k8sClient.Delete(testCtx, node)
			Expect(err).NotTo(HaveOccurred())
		}
	})
	Context("When NicClusterPolicy CR is created", func() {
		It("Upgrade policy is disabled", func() {
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}}

			curCr := &mellanoxv1alpha1.NicClusterPolicy{}
			err := k8sClient.Get(testCtx, req.NamespacedName, curCr)
			Expect(err).NotTo(HaveOccurred())
			// remove OFEDDriver from NicClusterPolicy CR
			cr.Spec.OFEDDriver = nil
			err = k8sClient.Update(testCtx, curCr)
			Expect(err).NotTo(HaveOccurred())

			_, err = upgradeReconciler.Reconcile(testCtx, req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("removeNodeStateUpgradeLabels cleans up the node state upgrade labels", func() {
			// Call removeNodeUpgradeStateLabels function
			err := upgradeReconciler.removeNodeUpgradeStateLabels(testCtx)
			Expect(err).NotTo(HaveOccurred())

			// Verify that upgrade state labels were removed
			nodeList := &corev1.NodeList{}
			err = k8sClient.List(testCtx, nodeList)
			Expect(err).NotTo(HaveOccurred())

			for _, node := range nodeList.Items {
				_, present := node.Labels[upgrade.GetUpgradeStateLabelKey()]
				Expect(present).To(Equal(false))
			}
		})
	})

	Context("When upgrade policy is updated in NicClusterPolicy CR", func() {
		const (
			testRequestorID = "network-operator.nvidia.com"
			testNamespace   = "nvidia-network-operator"
		)
		It("skip node maintenance objects cleanup when upgrade policy is set", func() {
			setOrUnsetRequestorEnvVars(true)
			defer setOrUnsetRequestorEnvVars(false)

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: consts.NicClusterPolicyResourceName}}
			_, err := upgradeReconciler.Reconcile(testCtx, req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should cleanup node maintenance objects when upgrade policy is disabled", func() {
			setOrUnsetRequestorEnvVars(true)
			defer setOrUnsetRequestorEnvVars(false)

			for _, node := range nodes {
				// Create test NodeMaintenance object that should be cleaned up
				nodeMaintenance := &maintenancev1alpha1.NodeMaintenance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      upgrade.DefaultNodeMaintenanceNamePrefix + node.Name,
						Namespace: testNamespace,
					},
					Spec: maintenancev1alpha1.NodeMaintenanceSpec{
						NodeName:    node.Name,
						RequestorID: testRequestorID,
					},
				}
				err := k8sClient.Create(testCtx, nodeMaintenance)
				Expect(err).NotTo(HaveOccurred())
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}}
			curCr := &mellanoxv1alpha1.NicClusterPolicy{}
			err := k8sClient.Get(testCtx, req.NamespacedName, curCr)
			Expect(err).NotTo(HaveOccurred())
			// remove OFEDDriver from NicClusterPolicy CR
			curCr.Spec.OFEDDriver = nil
			err = k8sClient.Update(testCtx, curCr)
			Expect(err).NotTo(HaveOccurred())

			_, err = upgradeReconciler.Reconcile(testCtx, req)
			Expect(err).NotTo(HaveOccurred())

			for _, node := range nodes {
				// Verify that NodeMaintenance objects were deleted
				var deleted maintenancev1alpha1.NodeMaintenance
				err = k8sClient.Get(testCtx, types.NamespacedName{Name: upgrade.DefaultNodeMaintenanceNamePrefix + node.Name,
					Namespace: testNamespace}, &deleted)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		})
	})

	Context("cleanupNodeMaintenanceObjects", func() {
		var upgradeReconciler *UpgradeReconciler
		var testCtx goctx.Context
		const (
			testRequestorID  = "network-operator.nvidia.com"
			testNamespace    = "nvidia-network-operator"
			otherRequestorID = "other-operator.test.com"
		)

		BeforeEach(func() {
			testCtx = goctx.TODO()
			migrationCompletionChan := make(chan struct{})
			close(migrationCompletionChan)
			upgradeReconciler = &UpgradeReconciler{
				Client:      k8sClient,
				Scheme:      k8sClient.Scheme(),
				MigrationCh: migrationCompletionChan,
			}
		})

		It("should skip cleanup when maintenance operator is not enabled", func() {
			// Set environment variables to disable maintenance operator
			setOrUnsetRequestorEnvVars(false)
			defer setOrUnsetRequestorEnvVars(false)

			err := upgradeReconciler.cleanupNodeMaintenanceObjects(testCtx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should skip cleanup requestor is not owning the node maintenance objects", func() {
			setOrUnsetRequestorEnvVars(true)
			defer setOrUnsetRequestorEnvVars(false)

			for _, node := range nodes {
				// Create test NodeMaintenance object that should be cleaned up
				nodeMaintenance := &maintenancev1alpha1.NodeMaintenance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      upgrade.DefaultNodeMaintenanceNamePrefix + node.Name,
						Namespace: testNamespace,
					},
					Spec: maintenancev1alpha1.NodeMaintenanceSpec{
						NodeName:    node.Name,
						RequestorID: otherRequestorID,
					},
				}
				err := k8sClient.Create(testCtx, nodeMaintenance)
				Expect(err).NotTo(HaveOccurred())
			}

			err := upgradeReconciler.cleanupNodeMaintenanceObjects(testCtx)
			Expect(err).NotTo(HaveOccurred())
			// check node maintenance objects still exist
			for _, node := range nodes {
				var nodeMaintenance maintenancev1alpha1.NodeMaintenance
				err = k8sClient.Get(testCtx, types.NamespacedName{Name: upgrade.DefaultNodeMaintenanceNamePrefix + node.Name,
					Namespace: testNamespace}, &nodeMaintenance)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})

func createTestNodes(count int) []*corev1.Node {
	nodes := make([]*corev1.Node, count)
	for i := 0; i < count; i++ {
		nodes[i] = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        fmt.Sprintf("node-%d", i+1),
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
		}
	}
	return nodes
}

func setOrUnsetRequestorEnvVars(set bool) {
	if set {
		Expect(os.Setenv("MAINTENANCE_OPERATOR_ENABLED", "true")).NotTo(HaveOccurred())
		Expect(os.Setenv("MAINTENANCE_OPERATOR_REQUESTOR_ID", "network-operator.nvidia.com")).NotTo(HaveOccurred())
		Expect(os.Setenv("MAINTENANCE_OPERATOR_REQUESTOR_NAMESPACE", "nvidia-network-operator")).NotTo(HaveOccurred())
	} else {
		Expect(os.Unsetenv("MAINTENANCE_OPERATOR_ENABLED")).NotTo(HaveOccurred())
		Expect(os.Unsetenv("MAINTENANCE_OPERATOR_REQUESTOR_ID")).NotTo(HaveOccurred())
		Expect(os.Unsetenv("MAINTENANCE_OPERATOR_REQUESTOR_NAMESPACE")).NotTo(HaveOccurred())
	}
}
