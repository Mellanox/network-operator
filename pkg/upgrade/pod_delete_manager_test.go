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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Mellanox/network-operator/pkg/upgrade"
)

var _ = Describe("PodDeleteManager tests", func() {
	It("PodDeleteManager should delete only given pods", func() {
		ctx := context.TODO()

		namespace := createNamespace("test-namespace")
		noRestartPod := createPod("no-restart-pod", namespace.Name)

		restartPods := []*corev1.Pod{
			createPod("restart-pod1", namespace.Name),
			createPod("restart-pod2", namespace.Name),
			createPod("restart-pod3", namespace.Name),
		}

		podList := &corev1.PodList{}
		err := k8sClient.List(ctx, podList)
		Expect(err).To(Succeed())
		Expect(podList.Items).To(HaveLen(4))

		manager := upgrade.NewPodDeleteManager(k8sClient, log)
		err = manager.SchedulePodsRestart(ctx, restartPods)
		Expect(err).To(Succeed())

		podList = &corev1.PodList{}
		err = k8sClient.List(ctx, podList)
		Expect(err).To(Succeed())
		Expect(podList.Items).To(HaveLen(1))

		// Check that pod not scheduled for restart is not deleted
		err = k8sClient.Get(ctx, types.NamespacedName{Name: noRestartPod.Name, Namespace: namespace.Name}, noRestartPod)
		Expect(err).To(Succeed())
	})
	It("PodDeleteManager should report an error on invalid input", func() {
		ctx := context.TODO()

		namespace := createNamespace("test")
		deletedPod := createPod("deleted-pod", namespace.Name)
		deleteObj(deletedPod)

		podList := &corev1.PodList{}
		err := k8sClient.List(ctx, podList)
		Expect(err).To(Succeed())
		Expect(podList.Items).To(HaveLen(0))

		manager := upgrade.NewPodDeleteManager(k8sClient, log)
		err = manager.SchedulePodsRestart(ctx, []*corev1.Pod{deletedPod})
		Expect(err).To(HaveOccurred())
	})
	It("PodDeleteManager should not fail on empty input", func() {
		ctx := context.TODO()

		podList := &corev1.PodList{}
		err := k8sClient.List(ctx, podList)
		Expect(err).To(Succeed())
		Expect(podList.Items).To(HaveLen(0))

		manager := upgrade.NewPodDeleteManager(k8sClient, log)
		err = manager.SchedulePodsRestart(ctx, []*corev1.Pod{})
		Expect(err).To(Succeed())
	})
})
