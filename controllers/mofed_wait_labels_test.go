/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
)

var _ = Describe("NicNodePolicy mofed.wait label management", func() {
	ctx := context.Background()

	Context("When NicNodePolicy with OFED is reconciled", func() {
		It("should set mofed.wait=false when OFED pod becomes ready on the target node", func() {
			By("Creating a node matching the NNP selector")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mofed-wait-test-node",
					Labels: map[string]string{
						"test-nnp":                      "mofed-wait",
						nodeinfo.NodeLabelMlnxNIC:       "true",
						nodeinfo.NodeLabelOSName:        "ubuntu",
						nodeinfo.NodeLabelCPUArch:       "amd64",
						nodeinfo.NodeLabelKernelVerFull: "5.15.0-100-generic",
						nodeinfo.NodeLabelOSVer:         "22.04",
					},
				},
			}
			Expect(k8sClient.Create(ctx, node)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, node) }()

			By("Creating a NicNodePolicy targeting that node")
			nnp := &mellanoxv1alpha1.NicNodePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mofed-wait-test-nnp",
				},
				Spec: mellanoxv1alpha1.NicNodePolicySpec{
					NodeSelector: map[string]string{"test-nnp": "mofed-wait"},
					OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{
						ImageSpec: mellanoxv1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "acme.buzz",
							Version:          "5.9-0.5.6.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nnp)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, nnp) }()

			By("Simulating a ready OFED pod owned by this NNP")
			dsOwner := mellanoxv1alpha1.NicNodePolicyCRDName + "-" + nnp.Name
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mofed-wait-test-pod",
					Namespace: namespaceName,
					Labels: map[string]string{
						consts.OfedDriverLabel: "",
						consts.DSOwnerLabel:    dsOwner,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: node.Name,
					Containers: []corev1.Container{
						{Name: "mofed-container", Image: "acme.buzz/mofed:5.9-0.5.6.0"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, pod) }()

			// Update pod status to ready
			pod.Status = corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "mofed-container", Ready: true},
				},
			}
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

			By("Verifying mofed.wait is eventually set to false on the node")
			Eventually(func() string {
				n := &corev1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: node.Name}, n)
				if err != nil {
					return ""
				}
				return n.Labels[nodeinfo.NodeLabelWaitOFED]
			}, timeout*3, interval).Should(Equal("false"))
		})
	})
})
