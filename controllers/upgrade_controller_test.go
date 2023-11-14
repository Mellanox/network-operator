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

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
)

var _ = Describe("Upgrade Controller", func() {
	var cr mellanoxv1alpha1.NicClusterPolicy
	BeforeEach(func() {
		cr = mellanoxv1alpha1.NicClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consts.NicClusterPolicyResourceName,
				Namespace: "",
			},
			Spec: mellanoxv1alpha1.NicClusterPolicySpec{
				OFEDDriver: nil,
			},
		}

		Expect(cr).NotTo(Equal(nil))
		err := k8sClient.Create(goctx.TODO(), &cr)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() {
		err := k8sClient.Delete(goctx.TODO(), &cr)
		Expect(err).NotTo(HaveOccurred())
	})
	Context("When NicClusterPolicy CR is created", func() {
		It("Upgrade policy is disabled", func() {
			migrationCompletionChan := make(chan struct{})
			close(migrationCompletionChan)
			upgradeReconciler := &UpgradeReconciler{
				Client:      k8sClient,
				Scheme:      k8sClient.Scheme(),
				MigrationCh: migrationCompletionChan,
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: consts.NicClusterPolicyResourceName}}
			ctx := goctx.TODO()

			_, err := upgradeReconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("removeNodeStateUpgradeLabels cleans up the node state upgrade labels", func() {
			upgrade.SetDriverName("ofed")

			nodes := createTestNodes(3)
			for _, node := range nodes {
				node.Labels[upgrade.GetUpgradeStateLabelKey()] = "test-state"
				err := k8sClient.Create(goctx.TODO(), node)
				Expect(err).NotTo(HaveOccurred())
			}
			migrationCompletionChan := make(chan struct{})
			close(migrationCompletionChan)

			upgradeReconciler := &UpgradeReconciler{
				Client:      k8sClient,
				Scheme:      k8sClient.Scheme(),
				MigrationCh: migrationCompletionChan,
			}
			// Call removeNodeUpgradeStateLabels function
			err := upgradeReconciler.removeNodeUpgradeStateLabels(goctx.TODO())
			Expect(err).NotTo(HaveOccurred())

			// Verify that upgrade state labels were removed
			nodeList := &corev1.NodeList{}
			err = k8sClient.List(goctx.TODO(), nodeList)
			Expect(err).NotTo(HaveOccurred())

			for _, node := range nodeList.Items {
				_, present := node.Labels[upgrade.GetUpgradeStateLabelKey()]
				Expect(present).To(Equal(false))
			}
		})
	})
})

func createTestNodes(count int) []*corev1.Node {
	nodes := make([]*corev1.Node, count)
	for i := 0; i < count; i++ {
		nodes[i] = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        fmt.Sprintf("node-%d", i),
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
		}
	}
	return nodes
}
