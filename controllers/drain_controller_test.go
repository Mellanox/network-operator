/*
2025 NVIDIA CORPORATION & AFFILIATES

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

package controllers //nolint:dupl

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	constants "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"

	"github.com/Mellanox/network-operator/pkg/drain"
)

var (
	testRequestorID           = "secondary.requestor.com"
	drainRequestorID          = "primary-drain.requestor.com"
	drainRequestorNS          = "default"
	testNodeMaintenancePrefix = drain.DefaultNodeMaintenanceNamePrefix
	maxParallelOperations     = atomic.Int32{}
)

var _ = Describe("Drain Controller", Ordered, func() {

	BeforeAll(func() {
		By("Setup maintennace controller mock")
		mockNodeMaintenanceController(ctx, k8sClient, metav1.Condition{
			Type:               maintenancev1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             maintenancev1alpha1.ConditionReasonReady,
			Message:            "Maintenance completed successfully",
			LastTransitionTime: metav1.NewTime(time.Now()),
		})

		err := k8sClient.Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name: "default", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		Expect(k8sClient.DeleteAllOf(context.Background(), &corev1.Node{},
			&client.DeleteAllOfOptions{
				DeleteOptions: client.DeleteOptions{GracePeriodSeconds: ptr.To[int64](0)}})).ToNot(HaveOccurred())
		Expect(k8sClient.DeleteAllOf(context.Background(),
			&sriovnetworkv1.SriovNetworkNodeState{}, client.InNamespace(namespaceName),
			&client.DeleteAllOfOptions{
				DeleteOptions: client.DeleteOptions{GracePeriodSeconds: ptr.To[int64](0)}})).ToNot(HaveOccurred())
		Expect(k8sClient.DeleteAllOf(context.Background(), &corev1.Pod{},
			client.InNamespace(namespaceName),
			&client.DeleteAllOfOptions{
				DeleteOptions: client.DeleteOptions{GracePeriodSeconds: ptr.To[int64](0)}})).ToNot(HaveOccurred())

		poolConfig := &sriovnetworkv1.SriovNetworkPoolConfig{}
		poolConfig.SetNamespace(namespaceName)
		poolConfig.SetName("test-workers")
		err := k8sClient.Delete(context.Background(), poolConfig)
		if err != nil {
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		}

		podList := &corev1.PodList{}
		err = k8sClient.List(context.Background(), podList, &client.ListOptions{Namespace: "default"})
		Expect(err).ToNot(HaveOccurred())
		for _, podObj := range podList.Items {
			err = k8sClient.Delete(context.Background(), &podObj,
				&client.DeleteOptions{GracePeriodSeconds: ptr.To[int64](0)})
			Expect(err).ToNot(HaveOccurred())
		}

	})

	Context("when there is only one node", func() {
		It("should not drain node on drain require while use-external-drainer annotation is not set",
			func(ctx context.Context) {
				node, nodeState := createNode(ctx, "node1", false)

				simulateDaemonSetAnnotation(node, constants.DrainRequired)

				expectNodeStateAnnotation(nodeState, constants.DrainIdle)
				expectNodeIsSchedulable(node)

			})

		It("should drain single node on drain require", func(ctx context.Context) {
			node, nodeState := createNode(ctx, "node1", true)

			simulateDaemonSetAnnotation(node, constants.DrainRequired)

			expectNodeStateAnnotation(nodeState, constants.DrainComplete)
			expectNodeIsNotSchedulable(node)

			simulateDaemonSetAnnotation(node, constants.DrainIdle)

			expectNodeStateAnnotation(nodeState, constants.DrainIdle)
			expectNodeIsSchedulable(node)

		})

		It("should not drain on reboot for single node", func(ctx context.Context) {
			node, nodeState := createNode(ctx, "node1", true)

			simulateDaemonSetAnnotation(node, constants.RebootRequired)

			expectNodeStateAnnotation(nodeState, constants.DrainComplete)
			expectNodeIsSchedulable(node)

			simulateDaemonSetAnnotation(node, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState, constants.DrainIdle)
			expectNodeIsSchedulable(node)
		})

		It("should drain on reboot for multiple node", func(ctx context.Context) {
			node, nodeState := createNode(ctx, "node1", true)
			createNode(ctx, "node2", true)

			simulateDaemonSetAnnotation(node, constants.RebootRequired)

			expectNodeStateAnnotation(nodeState, constants.DrainComplete)
			expectNodeIsNotSchedulable(node)

			simulateDaemonSetAnnotation(node, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState, constants.DrainIdle)
			expectNodeIsSchedulable(node)
		})

		It("should drain single node on drain require, with additional requestor", func(ctx context.Context) {
			node, nodeState := createNode(ctx, "node1", true)
			nmName := fmt.Sprintf("%s-%s", testNodeMaintenancePrefix, node.Name)
			_ = newNodeMaintenance(ctx, k8sClient, node.Name, drainRequestorNS)

			simulateDaemonSetAnnotation(node, constants.DrainRequired)
			expectNodeStateAnnotation(nodeState, constants.DrainComplete)
			expectNodeIsNotSchedulable(node)

			nm := maintenancev1alpha1.NodeMaintenance{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nmName, Namespace: drainRequestorNS},
				&nm)).ToNot(HaveOccurred())
			expectNodeMaintenanceUpdate(nmName, []string{drainRequestorID})

			expectNodeStateAnnotation(nodeState, constants.DrainComplete)
			simulateDaemonSetAnnotation(node, constants.DrainIdle)
			expectNodeMaintenanceUpdate(nmName, nil)
			// expect node to be unschedulable
			expectNodeIsNotSchedulable(node)

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nmName, Namespace: drainRequestorNS},
				&nm)).ToNot(HaveOccurred())
			Eventually(k8sClient.Delete(ctx, &nm)).ToNot(HaveOccurred())
			expectNodeStateAnnotation(nodeState, constants.DrainIdle)
			expectNodeIsSchedulable(node)
		})
	})

	Context("when there are multiple nodes", func() {

		It("should drain nodes serially", func(ctx context.Context) {
			node1, nodeState1 := createNode(ctx, "node1", true)
			node2, nodeState2 := createNode(ctx, "node2", true)
			node3, nodeState3 := createNode(ctx, "node3", true)

			// Two nodes require to drain at the same time
			maxParallelOperations.Store(1)
			simulateDaemonSetAnnotation(node1, constants.DrainRequired)
			simulateDaemonSetAnnotation(node2, constants.DrainRequired)

			// Only the first node drains
			expectNodeStateAnnotation(nodeState1, constants.DrainComplete)
			expectNodeStateAnnotation(nodeState2, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState3, constants.DrainIdle)
			expectNodeIsNotSchedulable(node1)
			expectNodeIsSchedulable(node2)
			expectNodeIsSchedulable(node3)

			simulateDaemonSetAnnotation(node1, constants.DrainIdle)

			expectNodeStateAnnotation(nodeState1, constants.DrainIdle)
			expectNodeIsSchedulable(node1)

			// Second node starts draining
			expectNodeStateAnnotation(nodeState1, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState2, constants.DrainComplete)
			expectNodeStateAnnotation(nodeState3, constants.DrainIdle)
			expectNodeIsSchedulable(node1)
			expectNodeIsNotSchedulable(node2)
			expectNodeIsSchedulable(node3)

			simulateDaemonSetAnnotation(node2, constants.DrainIdle)

			expectNodeStateAnnotation(nodeState1, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState2, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState3, constants.DrainIdle)
			expectNodeIsSchedulable(node1)
			expectNodeIsSchedulable(node2)
			expectNodeIsSchedulable(node3)
		})

		It("should drain nodes in parallel", func(ctx context.Context) {
			node1, nodeState1 := createNode(ctx, "node1", true)
			node2, nodeState2 := createNode(ctx, "node2", true)
			node3, nodeState3 := createNode(ctx, "node3", true)

			// two nodes require to drain at the same time
			maxParallelOperations.Store(2)
			simulateDaemonSetAnnotation(node1, constants.DrainRequired)
			simulateDaemonSetAnnotation(node2, constants.DrainRequired)

			// drain two nodes in parallel
			expectNodeStateAnnotation(nodeState1, constants.DrainComplete)
			expectNodeStateAnnotation(nodeState2, constants.DrainComplete)
			expectNodeStateAnnotation(nodeState3, constants.DrainIdle)
			expectNodeIsNotSchedulable(node1)
			expectNodeIsNotSchedulable(node2)
			expectNodeIsSchedulable(node3)

			simulateDaemonSetAnnotation(node1, constants.DrainIdle)
			simulateDaemonSetAnnotation(node2, constants.DrainIdle)

			expectNodeStateAnnotation(nodeState1, constants.DrainIdle)
			expectNodeIsSchedulable(node1)
			expectNodeStateAnnotation(nodeState2, constants.DrainIdle)
			expectNodeIsSchedulable(node2)

			// all nodes shoud be in idle state
			expectNodeStateAnnotation(nodeState1, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState2, constants.DrainIdle)
			expectNodeStateAnnotation(nodeState3, constants.DrainIdle)
			expectNodeIsSchedulable(node1)
			expectNodeIsSchedulable(node2)
			expectNodeIsSchedulable(node3)
		})
	})
})

func expectNodeStateAnnotation(nodeState *sriovnetworkv1.SriovNetworkNodeState, expectedAnnotationValue string) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: nodeState.Namespace, Name: nodeState.Name}, nodeState)).
			ToNot(HaveOccurred())

		g.Expect(utils.ObjectHasAnnotation(nodeState, constants.NodeStateDrainAnnotationCurrent,
			expectedAnnotationValue)).
			To(BeTrue(),
				"Node[%s] annotation[%s] == '%s'. Expected '%s'", nodeState.Name,
				constants.NodeDrainAnnotation, nodeState.GetAnnotations()[constants.NodeStateDrainAnnotationCurrent],
				expectedAnnotationValue)
	}, "20s", "1s").Should(Succeed())
}

func ExpectDrainCompleteNodesHaveIsNotSchedule(nodesState ...*sriovnetworkv1.SriovNetworkNodeState) {
	for _, nodeState := range nodesState {
		if utils.ObjectHasAnnotation(nodeState, constants.NodeStateDrainAnnotationCurrent, constants.DrainComplete) {
			node := &corev1.Node{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: nodeState.Name}, node)).
				ToNot(HaveOccurred())
			expectNodeIsNotSchedulable(node)
		}
	}
}

func expectNodeIsNotSchedulable(node *corev1.Node) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: node.Name}, node)).
			ToNot(HaveOccurred())

		g.Expect(node.Spec.Unschedulable).To(BeTrue())
	}, "20s", "1s").Should(Succeed())
}

func expectNodeIsSchedulable(node *corev1.Node) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: node.Name}, node)).
			ToNot(HaveOccurred())

		g.Expect(node.Spec.Unschedulable).To(BeFalse())
	}, "20s", "1s").Should(Succeed())
}

func expectNodeMaintenanceUpdate(name string, expectedAnnotationValue []string) {
	EventuallyWithOffset(1, func(g Gomega) {
		nm := &maintenancev1alpha1.NodeMaintenance{}
		g.Expect(k8sClient.Get(context.Background(),
			types.NamespacedName{Name: name, Namespace: drainRequestorNS}, nm)).
			ToNot(HaveOccurred())

		g.Expect(nm.Spec.AdditionalRequestors).To(Equal(expectedAnnotationValue))
	}, "20s", "1s").Should(Succeed())
}

func simulateDaemonSetAnnotation(node *corev1.Node, drainAnnotationValue string) {
	ExpectWithOffset(1,
		utils.AnnotateObject(context.Background(), node,
			constants.NodeDrainAnnotation, drainAnnotationValue, k8sClient)).
		ToNot(HaveOccurred())
}

func createNode(ctx context.Context, nodeName string, useExternalDrainer bool) (*corev1.Node,
	*sriovnetworkv1.SriovNetworkNodeState) {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Annotations: map[string]string{
				constants.NodeDrainAnnotation:                     constants.DrainIdle,
				"machineconfiguration.openshift.io/desiredConfig": "worker-1",
			},
			Labels: map[string]string{
				"test": "",
			},
		},
	}

	nodeState := sriovnetworkv1.SriovNetworkNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: namespaceName,
			Annotations: map[string]string{
				constants.NodeStateDrainAnnotationCurrent: constants.DrainIdle,
			},
		},
	}
	if useExternalDrainer {
		// TODO: change annotation name to constants.NodeExternalDrainerAnnotation once SRIOV PR is merged
		nodeState.Annotations["sriovnetwork.openshift.io/use-external-drainer"] = "true"
	}

	Expect(k8sClient.Create(ctx, &node)).ToNot(HaveOccurred())
	Expect(k8sClient.Create(ctx, &nodeState)).ToNot(HaveOccurred())

	return &node, &nodeState
}

func mockNodeMaintenanceController(
	ctx context.Context,
	c client.Client,
	//nolint:gocritic
	desiredCondition metav1.Condition,
) {
	go func() {
		defer GinkgoRecover()

		// Create a rate-limited work queue
		queue := workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.NewTypedItemExponentialFailureRateLimiter[string](100*time.Millisecond, 5*time.Second),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "nodeMaintenance"},
		)
		defer queue.ShutDown()

		// Add initial work item to start polling
		queue.Add("poll")

		for {
			select {
			case <-ctx.Done():
				return
			default:
				item, shutdown := queue.Get()
				if shutdown {
					return
				}

				// Process work item directly (no second goroutine)
				func() {
					defer queue.Done(item)

					// Process the polling work
					nms := &maintenancev1alpha1.NodeMaintenanceList{}
					err := c.List(ctx, nms, client.InNamespace(drainRequestorNS))
					if err != nil {
						return
					}

					processItems(ctx, c, &maxParallelOperations, nms, desiredCondition)
					// Re-queue for next poll (success case)
					queue.AddAfter("poll", 100*time.Millisecond)
				}()
			}
		}
	}()
}

//nolint:gocritic
func processItems(ctx context.Context, c client.Client, maxParallel *atomic.Int32,
	nms *maintenancev1alpha1.NodeMaintenanceList, desiredCondition metav1.Condition) {
	// Process each NodeMaintenance object
	for i, nm := range nms.Items {
		if i != 0 && int32(i) >= maxParallel.Load() {
			break
		}

		if nm.Spec.RequestorID != drainRequestorID && nm.Spec.RequestorID != testRequestorID {
			continue
		}

		// Handle deletion case
		if !nm.DeletionTimestamp.IsZero() {
			By("maintenance operator: remove node-maintenance finalizer")
			if controllerutil.ContainsFinalizer(&nm, maintenancev1alpha1.MaintenanceFinalizerName) {
				original := nm.DeepCopy()
				nm.SetFinalizers([]string{})
				patch := client.MergeFrom(original)
				Expect(c.Patch(ctx, &nm, patch)).To(Succeed())
			}

			// uncordon node
			By("maintenance operator: uncordon node")
			node := &corev1.Node{}
			Expect(c.Get(ctx, types.NamespacedName{Name: nm.Spec.NodeName}, node)).ToNot(HaveOccurred())
			node.Spec.Unschedulable = false
			Expect(c.Update(ctx, node)).To(Succeed())
			continue
		}

		// Handle creation/update case
		if !controllerutil.ContainsFinalizer(&nm, maintenancev1alpha1.MaintenanceFinalizerName) {
			By("maintenance operator: add node-maintenance finalizer")
			nm.Finalizers = append(nm.Finalizers, maintenancev1alpha1.MaintenanceFinalizerName)
			Expect(c.Update(ctx, &nm)).To(Succeed())
			continue
		}

		// Update status conditions
		if nm.Status.Conditions == nil {
			By("maintenance operator: add node-maintenance conditions")
			Expect(nm.Finalizers).To(ContainElement(maintenancev1alpha1.MaintenanceFinalizerName))
			// Update status conditions
			meta.SetStatusCondition(&nm.Status.Conditions, desiredCondition)
			Expect(c.Status().Update(ctx, &nm)).To(Succeed())
			Expect(nm.Status.Conditions).To(HaveLen(1))

			// cordon node
			By("maintenance operator: cordon node")
			node := &corev1.Node{}
			Expect(c.Get(ctx, types.NamespacedName{Name: nm.Spec.NodeName}, node)).ToNot(HaveOccurred())
			node.Spec.Unschedulable = true
			Expect(c.Update(ctx, node)).To(Succeed())
		}
	}
}

func newNodeMaintenance(ctx context.Context, c client.Client,
	name, namespace string) *maintenancev1alpha1.NodeMaintenance {
	nm := &maintenancev1alpha1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", testNodeMaintenancePrefix, name),
			Namespace: namespace,
		},
		Spec: maintenancev1alpha1.NodeMaintenanceSpec{
			NodeName:    name,
			RequestorID: testRequestorID,
		},
	}

	nm.SetFinalizers([]string{maintenancev1alpha1.MaintenanceFinalizerName})
	Expect(c.Create(ctx, nm)).To(Succeed())

	return nm
}
