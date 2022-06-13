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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/upgrade"
)

var _ = Describe("DrainManager tests", func() {
	It("DrainManager should drain nodes", func() {
		ctx := context.TODO()

		node := createNode("node")

		drainManager := upgrade.NewDrainManager(k8sInterface, upgrade.NewNodeUpgradeStateProvider(k8sClient, log), log)
		drainSpec := &DrainSpec{
			Enable:         true,
			Force:          false,
			PodSelector:    "",
			TimeoutSecond:  1,
			DeleteEmptyDir: true,
		}
		nodeArray := []*corev1.Node{node}
		err := drainManager.ScheduleNodesDrain(ctx, &upgrade.DrainConfiguration{Nodes: nodeArray, Spec: drainSpec})
		Expect(err).To(Succeed())

		time.Sleep(time.Second)

		err = k8sClient.Get(ctx, types.NamespacedName{Name: node.Name}, node)
		Expect(err).To(Succeed())
		Expect(node.Spec.Unschedulable).To(BeTrue())
	})
	It("DrainManager should not fail on empty node list", func() {
		ctx := context.TODO()

		drainManager := upgrade.NewDrainManager(k8sInterface, upgrade.NewNodeUpgradeStateProvider(k8sClient, log), log)
		drainSpec := &DrainSpec{
			Enable:         true,
			Force:          false,
			PodSelector:    "",
			TimeoutSecond:  1,
			DeleteEmptyDir: true,
		}
		err := drainManager.ScheduleNodesDrain(ctx, &upgrade.DrainConfiguration{Nodes: nil, Spec: drainSpec})
		Expect(err).To(Succeed())

		time.Sleep(time.Second)
	})
	It("DrainManager should return error on nil drain spec", func() {
		ctx := context.TODO()

		node := createNode("node")

		drainManager := upgrade.NewDrainManager(k8sInterface, upgrade.NewNodeUpgradeStateProvider(k8sClient, log), log)

		nodeArray := []*corev1.Node{node}
		err := drainManager.ScheduleNodesDrain(ctx, &upgrade.DrainConfiguration{Nodes: nodeArray, Spec: nil})
		Expect(err).ToNot(Succeed())

		time.Sleep(time.Second)

		err = k8sClient.Get(ctx, types.NamespacedName{Name: node.Name}, node)
		Expect(err).To(Succeed())
		Expect(node.Spec.Unschedulable).To(BeFalse())
	})
	It("DrainManager should skip drain on empty drain spec", func() {
		ctx := context.TODO()

		node := createNode("node")

		drainManager := upgrade.NewDrainManager(k8sInterface, upgrade.NewNodeUpgradeStateProvider(k8sClient, log), log)

		nodeArray := []*corev1.Node{node}
		err := drainManager.ScheduleNodesDrain(ctx, &upgrade.DrainConfiguration{Nodes: nodeArray, Spec: &DrainSpec{}})
		Expect(err).To(Succeed())

		time.Sleep(time.Second)

		err = k8sClient.Get(ctx, types.NamespacedName{Name: node.Name}, node)
		Expect(err).To(Succeed())
		Expect(node.Spec.Unschedulable).To(BeFalse())
	})
	It("DrainManager should skip drain if drain is disabled in the spec", func() {
		ctx := context.TODO()

		node := createNode("node")

		drainManager := upgrade.NewDrainManager(k8sInterface, upgrade.NewNodeUpgradeStateProvider(k8sClient, log), log)

		nodeArray := []*corev1.Node{node}
		err := drainManager.ScheduleNodesDrain(
			ctx, &upgrade.DrainConfiguration{Nodes: nodeArray, Spec: &DrainSpec{Enable: false}})
		Expect(err).To(Succeed())

		time.Sleep(time.Second)

		err = k8sClient.Get(ctx, types.NamespacedName{Name: node.Name}, node)
		Expect(err).To(Succeed())
		Expect(node.Spec.Unschedulable).To(BeFalse())
	})
})
