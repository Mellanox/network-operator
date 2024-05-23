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

const (
	testArch       = "amd64"
	testOsUbuntu   = "ubuntu"
	testOsRhcos    = "rhcos"
	testOsVer      = "22.04"
	testKernelFull = "5.15.0-78-generic"
)

// A Filter applies a filter on a list of Nodes
type dummyFilter struct {
	called   bool
	filtered []*corev1.Node
}

func (df *dummyFilter) Apply(_ []*corev1.Node) []*corev1.Node {
	df.called = true
	return df.filtered
}

var _ = Describe("nodeinfo Provider tests", func() {
	Context("GetNodePools with filter", func() {
		It("Should return an empty list of pools", func() {
			filter := &dummyFilter{filtered: []*corev1.Node{}}
			provider := NewProvider([]*corev1.Node{
				getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-2", testOsUbuntu, testOsVer, testKernelFull, testArch),
			})
			pools := provider.GetNodePools(filter)
			Expect(len(pools)).To(Equal(0))
		})
		It("Should return an empty list of pools, filter by label", func() {
			node := getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch)
			delete(node.Labels, NodeLabelMlnxNIC)
			provider := NewProvider([]*corev1.Node{node})
			pools := provider.GetNodePools(NewNodeLabelFilterBuilder().WithLabel(NodeLabelMlnxNIC, "true").Build())
			Expect(len(pools)).To(Equal(0))
		})
	})

	Context("GetNodePools without filter", func() {
		It("Should return pool", func() {
			provider := NewProvider([]*corev1.Node{
				getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-2", testOsUbuntu, testOsVer, testKernelFull, testArch),
			})
			pools := provider.GetNodePools()
			Expect(len(pools)).To(Equal(1))
			Expect(pools[0].Arch).To(Equal(testArch))
			Expect(pools[0].OsName).To(Equal(testOsUbuntu))
			Expect(pools[0].OsVersion).To(Equal(testOsVer))
			Expect(pools[0].Kernel).To(Equal(testKernelFull))
		})
		DescribeTable("GetNodePools",
			func(nodeList []*corev1.Node, expectedPools int) {
				provider := NewProvider(nodeList)
				pools := provider.GetNodePools()
				Expect(len(pools)).To(Equal(expectedPools))
			},
			Entry("single pool, multiple nodes same NFD labels", []*corev1.Node{
				getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-2", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-3", testOsUbuntu, testOsVer, testKernelFull, testArch),
			}, 1),
			Entry("2 pools, multiple nodes different OS NFD labels", []*corev1.Node{
				getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-2", testOsRhcos, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-3", testOsRhcos, testOsVer, testKernelFull, testArch),
			}, 2),
			Entry("2 pools, multiple nodes different OSVer NFD labels", []*corev1.Node{
				getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-2", testOsUbuntu, "20.04", testKernelFull, testArch),
			}, 2),
			Entry("2 pools, multiple nodes different KernelFull NFD labels", []*corev1.Node{
				getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-2", testOsUbuntu, testOsVer, "6", testArch),
			}, 2),
			Entry("1 pool, multiple nodes different arch NFD labels", []*corev1.Node{
				getNodeWithNfdLabels("Node-1", testOsUbuntu, testOsVer, testKernelFull, testArch),
				getNodeWithNfdLabels("Node-2", testOsUbuntu, testOsVer, testKernelFull, "arm"),
			}, 1),
			Entry("no pool, node without NFD labels", []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "Node-1"},
				},
			}, 0),
			Entry("no pool, node with missing osName NFD label", []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Node-1",
						Labels: map[string]string{
							NodeLabelMlnxNIC:       "true",
							NodeLabelOSVer:         testOsVer,
							NodeLabelKernelVerFull: testKernelFull,
							NodeLabelCPUArch:       testArch,
						}},
				},
			}, 0),
			Entry("no pool, node with missing osVer NFD label", []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Node-1",
						Labels: map[string]string{
							NodeLabelMlnxNIC:       "true",
							NodeLabelOSName:        testOsUbuntu,
							NodeLabelKernelVerFull: testKernelFull,
							NodeLabelCPUArch:       testArch,
						}},
				},
			}, 0),
			Entry("no pool, node with missing arch NFD label", []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Node-1",
						Labels: map[string]string{
							NodeLabelMlnxNIC:       "true",
							NodeLabelOSName:        testOsUbuntu,
							NodeLabelOSVer:         testOsVer,
							NodeLabelKernelVerFull: testKernelFull,
						}},
				},
			}, 0),
			Entry("no pool, node with missing Kernel full NFD label", []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Node-1",
						Labels: map[string]string{
							NodeLabelMlnxNIC: "true",
							NodeLabelOSName:  testOsUbuntu,
							NodeLabelOSVer:   testOsVer,
							NodeLabelCPUArch: testArch,
						}},
				},
			}, 0),
		)
	})
})

func getNodeWithNfdLabels(name, osName, osVer, kernelFull, arch string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				NodeLabelMlnxNIC:       "true",
				NodeLabelOSName:        osName,
				NodeLabelOSVer:         osVer,
				NodeLabelKernelVerFull: kernelFull,
				NodeLabelCPUArch:       arch,
			},
		},
	}
}
