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

package migrate

import (
	goctx "context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/network-operator/pkg/consts"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
)

//nolint:dupl
var _ = Describe("Migrate", func() {
	AfterEach(func() {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: nvIPAMcmName}}
		_ = k8sClient.Delete(goctx.Background(), cm)
		_ = k8sClient.DeleteAllOf(goctx.Background(), &corev1.Node{})
		_ = k8sClient.DeleteAllOf(goctx.Background(), &corev1.Pod{})
	})
	It("should delete MOFED DS", func() {
		upgrade.SetDriverName("ofed")
		createNCP()
		createMofedDS()
		createNodes()
		createPods()
		By("Verify Single DS is deleted")
		err := Migrate(goctx.Background(), testLog, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			ds := &appsv1.DaemonSet{}
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "test-ds"}, ds)
			return errors.IsNotFound(err)
		})
		By("Verify Nodes have upgrade-requested annotation")
		Eventually(func() bool {
			node1 := &corev1.Node{}
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "test-node1"}, node1)
			Expect(err).NotTo(HaveOccurred())
			node2 := &corev1.Node{}
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "test-node2"}, node2)
			Expect(err).NotTo(HaveOccurred())
			return node1.Annotations[upgrade.GetUpgradeRequestedAnnotationKey()] == "true" &&
				node2.Annotations[upgrade.GetUpgradeRequestedAnnotationKey()] == "true"
		})
	})
})

func createMofedDS() {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-ds",
			Labels:    map[string]string{"nvidia.com/ofed-driver": ""},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mofed-ubuntu22.04"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "mofed-ubuntu22.04"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mofed-container",
							Image: "github/mofed",
						},
					},
				},
			},
		},
	}
	err := k8sClient.Create(goctx.Background(), ds)
	Expect(err).NotTo(HaveOccurred())
}

func createNodes() {
	By("Create Nodes")
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-node1",
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}
	err := k8sClient.Create(goctx.TODO(), node)
	Expect(err).NotTo(HaveOccurred())
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-node2",
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}
	err = k8sClient.Create(goctx.TODO(), node2)
	Expect(err).NotTo(HaveOccurred())
}

func createPods() {
	By("Create Pods")
	gracePeriodSeconds := int64(0)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod1",
			Namespace: namespaceName,
			Labels:    map[string]string{"app": "mofed-ubuntu22.04"},
		},
		Spec: corev1.PodSpec{
			NodeName:                      "test-node1",
			TerminationGracePeriodSeconds: &gracePeriodSeconds,
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}
	err := k8sClient.Create(goctx.TODO(), pod)
	Expect(err).NotTo(HaveOccurred())
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod2",
			Namespace: namespaceName,
			Labels:    map[string]string{"app": "mofed-ubuntu22.04"},
		},
		Spec: corev1.PodSpec{
			NodeName:                      "test-node2",
			TerminationGracePeriodSeconds: &gracePeriodSeconds,
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}
	err = k8sClient.Create(goctx.TODO(), pod2)
	Expect(err).NotTo(HaveOccurred())
}

func createNCP() {
	ncp := &mellanoxv1alpha1.NicClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: consts.NicClusterPolicyResourceName}}
	ncp.Spec.OFEDDriver = &mellanoxv1alpha1.OFEDDriverSpec{
		ImageSpec: mellanoxv1alpha1.ImageSpec{
			Image:            "mofed",
			Repository:       "nvcr.io/nvidia/mellanox",
			Version:          "5.9-0.5.6.0",
			ImagePullSecrets: []string{},
		},
	}
	err := k8sClient.Create(goctx.Background(), ncp)
	Expect(err).NotTo(HaveOccurred())
}
