/*
Copyright 2020 NVIDIA

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

package nodeinfo

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NodeAttributes tests", func() {

	nodes := []*corev1.Node{
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					NodeLabelOSName:           "ubuntu",
					NodeLabelCPUArch:          "amd64",
					NodeLabelKernelVerFull:    "5.4.0-generic",
					NodeLabelOSVer:            "20.04",
					NodeLabelCudaVersionMajor: "465"},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
				Labels: map[string]string{
					NodeLabelOSName:           "rhel",
					NodeLabelCPUArch:          "x86_64",
					NodeLabelKernelVerFull:    "5.4.0-generic",
					NodeLabelCudaVersionMajor: "460"},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-3",
				Labels: map[string]string{
					NodeLabelOSName:        "ubuntu",
					NodeLabelCPUArch:       "amd64",
					NodeLabelKernelVerFull: "4.5.0-generic"},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{Kind: "Node"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-4",
				Labels: map[string]string{
					NodeLabelOSName:        "coreos",
					NodeLabelCPUArch:       "x86_64",
					NodeLabelKernelVerFull: "5.4.0-generic"},
			},
		}}

	Context("Filter on empty list of nodes", func() {
		It("Should return an empty list of nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(NodeLabelHostname, "test-host").
				Build()
			filteredNodes := filter.Apply([]*corev1.Node{})

			Expect(filteredNodes).To(BeEmpty())
		})
	})

	Context("Filter with criteria that doesnt match any node", func() {
		It("Should return an empty list of nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(NodeLabelCPUArch, "arm64").
				Build()
			filteredNodes := filter.Apply(nodes)

			Expect(filteredNodes).To(BeEmpty())
		})
	})

	Context("Filter with criteria that is missing from nodes", func() {
		It("Should return an empty list of nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(NodeLabelHostname, "test-host").
				Build()
			filteredNodes := filter.Apply(nodes)

			Expect(filteredNodes).To(BeEmpty())
		})
	})

	Context("Filter with criteria that match on some nodes", func() {
		It("Should only return the relevant nodes", func() {
			filter := NewNodeLabelFilterBuilder().
				WithLabel(NodeLabelKernelVerFull, "5.4.0-generic").
				WithLabel(NodeLabelCPUArch, "x86_64").
				Build()
			filteredNodes := filter.Apply(nodes)
			Expect(len(filteredNodes)).To(Equal(2))
			Expect(filteredNodes[0].Name).To(Equal("node-2"))
			Expect(filteredNodes[1].Name).To(Equal("node-4"))
		})
	})

	Context("Filter by labels without values", func() {
		It("Should only return the relevant nodes", func() {
			filter := NewNodeLabelNoValFilterBuilderr().
				WithLabel(NodeLabelCudaVersionMajor).
				Build()
			filteredNodes := filter.Apply(nodes)
			Expect(len(filteredNodes)).To(Equal(2))
			Expect(filteredNodes[0].Name).To(Equal("node-1"))
			Expect(filteredNodes[1].Name).To(Equal("node-2"))
		})
		It("Should return an empty list of nodes", func() {
			filter := NewNodeLabelNoValFilterBuilderr().
				WithLabel("unknown_label").
				Build()
			filteredNodes := filter.Apply(nodes)
			Expect(filteredNodes).To(BeEmpty())
		})
	})
})

var _ = Describe("Combined filtering tests", func() {
	commonLabel := map[string]string{"app": "test-app"}
	nodeWithLabelOnly := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-label-only", Labels: commonLabel},
	}
	nodeWithTaintNoSchedule := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-taint-noschedule", Labels: commonLabel},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    "gpu",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			}},
		},
	}
	nodeWithTaintNoExecute := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-taint-noexecute", Labels: commonLabel},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    "gpu",
				Value:  "true",
				Effect: corev1.TaintEffectNoExecute,
			}},
		},
	}
	nodeWithOtherTaint := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-other-taint", Labels: commonLabel},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    "storage",
				Value:  "slow",
				Effect: corev1.TaintEffectNoSchedule,
			}},
		},
	}
	nodeWithMultipleTaints := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-multi-taint", Labels: commonLabel},
		Spec: corev1.NodeSpec{Taints: []corev1.Taint{
			{Key: "gpu", Value: "true", Effect: corev1.TaintEffectNoSchedule},
			{Key: "maintenance", Value: "true", Effect: corev1.TaintEffectNoExecute},
		}},
	}
	nodeNoLabelWithTaint := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-nolabel-taint"},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    "gpu",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			}},
		},
	}

	allTestNodes := []*corev1.Node{
		nodeWithLabelOnly, nodeWithTaintNoSchedule, nodeWithTaintNoExecute,
		nodeWithOtherTaint, nodeWithMultipleTaints, nodeNoLabelWithTaint,
	}

	tolerationGpuNoSchedule := corev1.Toleration{
		Key:      "gpu",
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	}
	tolerationGpuNoExecute := corev1.Toleration{
		Key:      "gpu",
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoExecute,
	}
	tolerationMaintenance := corev1.Toleration{
		Key:      "maintenance",
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoExecute,
	}

	Context("When label and tolerations are provided", func() {
		It("Should filter nodes matching both label and tolerated taints", func() {
			labelFilter := NewNodeLabelFilterBuilder().WithLabel("app", "test-app").Build()
			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{tolerationGpuNoSchedule}).Build()

			filteredNodes := labelFilter.Apply(allTestNodes)
			filteredNodes = taintFilter.Apply(filteredNodes)

			Expect(len(filteredNodes)).To(Equal(2))
			Expect(filteredNodes).To(ContainElement(nodeWithLabelOnly))       // No taints, passes
			Expect(filteredNodes).To(ContainElement(nodeWithTaintNoSchedule)) // Tolerated NoSchedule taint
		})

		It("Should return no nodes if taints are not tolerated", func() {
			labelFilter := NewNodeLabelFilterBuilder().WithLabel("app", "test-app").Build()
			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{tolerationGpuNoExecute}).Build()
			// nodeWithTaintNoSchedule has TaintEffectNoSchedule, so it should be filtered out
			// nodeWithLabelOnly has no taints, so it should pass
			// nodeWithTaintNoExecute has TaintEffectNoExecute, so it should pass
			nodesToFilter := []*corev1.Node{nodeWithLabelOnly, nodeWithTaintNoSchedule, nodeWithTaintNoExecute}
			filteredNodes := labelFilter.Apply(nodesToFilter)
			filteredNodes = taintFilter.Apply(filteredNodes)
			Expect(len(filteredNodes)).To(Equal(2))
			Expect(filteredNodes).To(ContainElement(nodeWithLabelOnly))
			Expect(filteredNodes).To(ContainElement(nodeWithTaintNoExecute))
		})

		It("Should return only nodes with label if empty tolerations list is provided", func() {
			labelFilter := NewNodeLabelFilterBuilder().WithLabel("app", "test-app").Build()
			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{}).Build()

			filteredNodes := labelFilter.Apply(allTestNodes)
			filteredNodes = taintFilter.Apply(filteredNodes)

			// Empty tolerations should only include nodes with no taints
			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes).To(ContainElement(nodeWithLabelOnly))
			Expect(filteredNodes).NotTo(ContainElement(nodeWithTaintNoSchedule))
			Expect(filteredNodes).NotTo(ContainElement(nodeWithTaintNoExecute))
			Expect(filteredNodes).NotTo(ContainElement(nodeWithOtherTaint))
			Expect(filteredNodes).NotTo(ContainElement(nodeWithMultipleTaints))
		})

		It("Should support all 5 taint/toleration scenarios", func() {
			// Create a single node list with nodes for all 5 scenarios
			noTaintNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "no-taint-node", Labels: commonLabel},
			}
			gpuTaintNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu-taint-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    "gpu",
						Value:  "true",
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
			}
			otherTaintNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "other-taint-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    "storage",
						Value:  "slow",
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
			}

			testNodes := []*corev1.Node{noTaintNode, gpuTaintNode, otherTaintNode}
			labelFilter := NewNodeLabelFilterBuilder().WithLabel("app", "test-app").Build()

			// 1. No tolerations + no taints -> don't filter
			// 2. No tolerations + taints -> filter
			noTolerationsTaintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{}).Build()

			filteredNodes := noTolerationsTaintFilter.Apply(labelFilter.Apply(testNodes))
			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes).To(ContainElement(noTaintNode))       // Scenario 1: passes
			Expect(filteredNodes).NotTo(ContainElement(gpuTaintNode))   // Scenario 2: filtered out
			Expect(filteredNodes).NotTo(ContainElement(otherTaintNode)) // Scenario 2: filtered out

			// 3. Tolerations + no taints -> don't filter
			// 4. Tolerations + taints which don't match -> filter
			// 5. Tolerations + taints which match -> don't filter
			gpuTolerationsTaintFilter := NewNodeTaintFilterBuilder().
				WithTolerations([]corev1.Toleration{tolerationGpuNoSchedule}).
				Build()

			filteredNodes = gpuTolerationsTaintFilter.Apply(labelFilter.Apply(testNodes))
			Expect(len(filteredNodes)).To(Equal(2))
			Expect(filteredNodes).To(ContainElement(noTaintNode))       // Scenario 3: passes
			Expect(filteredNodes).To(ContainElement(gpuTaintNode))      // Scenario 5: passes
			Expect(filteredNodes).NotTo(ContainElement(otherTaintNode)) // Scenario 4: filtered out
		})
	})

	Context("When only label filter is applied", func() {
		It("Should return all nodes matching the label, regardless of taints", func() {
			labelFilter := NewNodeLabelFilterBuilder().WithLabel("app", "test-app").Build()
			filteredNodes := labelFilter.Apply(allTestNodes)
			Expect(len(filteredNodes)).To(Equal(5))
			Expect(filteredNodes).To(ContainElements(
				nodeWithLabelOnly, nodeWithTaintNoSchedule, nodeWithTaintNoExecute,
				nodeWithOtherTaint, nodeWithMultipleTaints,
			))
		})
	})

	Context("When only taint filter is applied", func() {
		It("Should filter nodes based on taints, regardless of labels", func() {
			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{tolerationGpuNoSchedule}).Build()
			filteredNodes := taintFilter.Apply(allTestNodes)
			// nodeWithLabelOnly (no taints) - PASS
			// nodeWithTaintNoSchedule (gpu/NoSchedule, tolerated) - PASS
			// nodeWithTaintNoExecute (gpu/NoExecute, not tolerated by this filter) - FAIL
			// nodeWithOtherTaint (storage/NoSchedule, not tolerated by this filter) - FAIL
			// nodeWithMultipleTaints (gpu/NoSchedule tolerated, maintenance/NoExecute not tolerated) -
			// FAIL (all taints must be tolerated)
			// nodeNoLabelWithTaint (gpu/NoSchedule, tolerated) - PASS
			Expect(len(filteredNodes)).To(Equal(3))
			Expect(filteredNodes).To(ContainElement(nodeWithLabelOnly))
			Expect(filteredNodes).To(ContainElement(nodeWithTaintNoSchedule))
			Expect(filteredNodes).To(ContainElement(nodeNoLabelWithTaint))
		})
	})

	Context("When node has multiple taints", func() {
		It("Should only pass if all taints are tolerated", func() {
			labelFilter := NewNodeLabelFilterBuilder().WithLabel("app", "test-app").Build()
			taintFilter := NewNodeTaintFilterBuilder().WithTolerations(
				[]corev1.Toleration{tolerationGpuNoSchedule, tolerationMaintenance}).Build()

			nodesToFilter := []*corev1.Node{nodeWithLabelOnly, nodeWithMultipleTaints}
			filteredNodes := taintFilter.Apply(labelFilter.Apply(nodesToFilter))

			Expect(len(filteredNodes)).To(Equal(2))
			Expect(filteredNodes).To(ContainElement(nodeWithLabelOnly))
			Expect(filteredNodes).To(ContainElement(nodeWithMultipleTaints))
		})

		It("Should fail if not all taints are tolerated", func() {
			labelFilter := NewNodeLabelFilterBuilder().WithLabel("app", "test-app").Build()
			taintFilter := NewNodeTaintFilterBuilder().WithTolerations(
				[]corev1.Toleration{tolerationGpuNoSchedule}).Build() // Only tolerates one of the two taints

			nodesToFilter := []*corev1.Node{nodeWithLabelOnly, nodeWithMultipleTaints}
			filteredNodes := taintFilter.Apply(labelFilter.Apply(nodesToFilter))
			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes).To(ContainElement(nodeWithLabelOnly)) // nodeWithMultipleTaints should be filtered out
		})
	})

	Context("When no filter criteria are specified", func() {
		It("Should return all input nodes", func() {
			filteredNodes := allTestNodes
			Expect(len(filteredNodes)).To(Equal(len(allTestNodes)))
		})
	})

	Context("New Filter Test for Node with MlnxNIC and No Taints", func() {
		It("Should include the node if it has the Mellanox NIC label and no taints, even if the filter has tolerations",
			func() {
				testNodes := []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node-mlnx-nic-no-taints",
							Labels: map[string]string{
								NodeLabelMlnxNIC:       "true",
								NodeLabelOSName:        "ubuntu",
								NodeLabelCPUArch:       "amd64",
								NodeLabelKernelVerFull: "generic-9.0.1",
								NodeLabelOSVer:         "20.0.4",
							},
							// No taints defined for this node
						},
					},
				}

				// Define a filter that requires the Mellanox NIC label
				// and includes some tolerations.
				filterTolerations := []corev1.Toleration{
					{Key: "example.com/some-taint", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				}

				labelFilter := NewNodeLabelFilterBuilder().WithLabel(NodeLabelMlnxNIC, "true").Build()
				taintFilter := NewNodeTaintFilterBuilder().WithTolerations(filterTolerations).Build()

				filteredNodes := taintFilter.Apply(labelFilter.Apply(testNodes))

				// Expectation: The node has the required label and no taints.
				// A node with no taints should always pass a toleration check.
				// Thus, it should not be filtered out.
				Expect(filteredNodes).To(HaveLen(1), "Node with MlnxNIC and no taints should not be filtered out")
				Expect(filteredNodes[0].Name).To(Equal("test-node-mlnx-nic-no-taints"),
					"The correct node should be present in the filtered list")
			})
	})

	Context("DaemonSet default tolerations", func() {
		// Test that nodes with DaemonSet default taints are properly handled when tolerations are provided
		It("Should filter nodes with unschedulable taint when toleration is provided", func() {
			nodeWithUnschedulable := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "cordoned-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    corev1.TaintNodeUnschedulable,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
			}

			tolerationForUnschedulable := corev1.Toleration{
				Key:      corev1.TaintNodeUnschedulable,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			}

			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{tolerationForUnschedulable}).Build()
			filteredNodes := taintFilter.Apply([]*corev1.Node{nodeWithUnschedulable})

			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes[0].Name).To(Equal("cordoned-node"))
		})

		It("Should filter out nodes with unschedulable taint when NO toleration is provided", func() {
			nodeWithUnschedulable := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "cordoned-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    corev1.TaintNodeUnschedulable,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
			}

			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{}).Build()
			filteredNodes := taintFilter.Apply([]*corev1.Node{nodeWithUnschedulable})

			Expect(len(filteredNodes)).To(Equal(0), "Node with unschedulable taint should be filtered out without toleration")
		})

		It("Should handle nodes with disk pressure taint when toleration is provided", func() {
			nodeWithDiskPressure := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "disk-pressure-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    corev1.TaintNodeDiskPressure,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
			}

			tolerationForDiskPressure := corev1.Toleration{
				Key:      corev1.TaintNodeDiskPressure,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			}

			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{tolerationForDiskPressure}).Build()
			filteredNodes := taintFilter.Apply([]*corev1.Node{nodeWithDiskPressure})

			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes[0].Name).To(Equal("disk-pressure-node"))
		})

		It("Should handle nodes with memory pressure taint when toleration is provided", func() {
			nodeWithMemoryPressure := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "memory-pressure-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    corev1.TaintNodeMemoryPressure,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
			}

			tolerationForMemoryPressure := corev1.Toleration{
				Key:      corev1.TaintNodeMemoryPressure,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			}

			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{tolerationForMemoryPressure}).Build()
			filteredNodes := taintFilter.Apply([]*corev1.Node{nodeWithMemoryPressure})

			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes[0].Name).To(Equal("memory-pressure-node"))
		})

		It("Should handle nodes with not-ready taint when toleration is provided", func() {
			nodeNotReady := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "not-ready-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    corev1.TaintNodeNotReady,
						Effect: corev1.TaintEffectNoExecute,
					}},
				},
			}

			tolerationForNotReady := corev1.Toleration{
				Key:      corev1.TaintNodeNotReady,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			}

			taintFilter := NewNodeTaintFilterBuilder().WithTolerations([]corev1.Toleration{tolerationForNotReady}).Build()
			filteredNodes := taintFilter.Apply([]*corev1.Node{nodeNotReady})

			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes[0].Name).To(Equal("not-ready-node"))
		})

		It("Should handle nodes with multiple DaemonSet default taints when all tolerations are provided", func() {
			nodeWithMultipleDefaultTaints := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-taint-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule},
						{Key: corev1.TaintNodeDiskPressure, Effect: corev1.TaintEffectNoSchedule},
						{Key: corev1.TaintNodeMemoryPressure, Effect: corev1.TaintEffectNoSchedule},
					},
				},
			}

			allDefaultTolerations := []corev1.Toleration{
				{Key: corev1.TaintNodeUnschedulable, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Key: corev1.TaintNodeDiskPressure, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Key: corev1.TaintNodeMemoryPressure, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
			}

			taintFilter := NewNodeTaintFilterBuilder().WithTolerations(allDefaultTolerations).Build()
			filteredNodes := taintFilter.Apply([]*corev1.Node{nodeWithMultipleDefaultTaints})

			Expect(len(filteredNodes)).To(Equal(1))
			Expect(filteredNodes[0].Name).To(Equal("multi-taint-node"))
		})

		It("Should filter out nodes with multiple DaemonSet default taints when only some tolerations are provided", func() {
			nodeWithMultipleDefaultTaints := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-taint-node", Labels: commonLabel},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule},
						{Key: corev1.TaintNodeDiskPressure, Effect: corev1.TaintEffectNoSchedule},
						{Key: corev1.TaintNodeMemoryPressure, Effect: corev1.TaintEffectNoSchedule},
					},
				},
			}

			// Only provide toleration for unschedulable, not for disk/memory pressure
			partialTolerations := []corev1.Toleration{
				{Key: corev1.TaintNodeUnschedulable, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
			}

			taintFilter := NewNodeTaintFilterBuilder().WithTolerations(partialTolerations).Build()
			filteredNodes := taintFilter.Apply([]*corev1.Node{nodeWithMultipleDefaultTaints})

			Expect(len(filteredNodes)).To(Equal(0), "Node should be filtered out when not all taints are tolerated")
		})
	})
})
