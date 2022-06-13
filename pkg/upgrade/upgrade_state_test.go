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
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/upgrade"
	"github.com/Mellanox/network-operator/pkg/upgrade/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

var _ = Describe("UpgradeStateManager tests", func() {
	It("UpgradeStateManager should fail on nil currentState", func() {
		ctx := context.TODO()

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, nil, &v1alpha1.OfedUpgradePolicySpec{})).ToNot(Succeed())
	})
	It("UpgradeStateManager should not fail on nil upgradePolicy", func() {
		ctx := context.TODO()

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &upgrade.ClusterUpgradeState{}, nil)).To(Succeed())
	})
	It("UpgradeStateManager should move up-to-date nodes to Done and outdated nodes to UpgradeRequired states", func() {
		ctx := context.TODO()

		daemonSet := &appsv1.DaemonSet{ObjectMeta: v1.ObjectMeta{Generation: 2}}
		upToDatePod := &corev1.Pod{
			ObjectMeta: v1.ObjectMeta{Labels: map[string]string{utils.PodTemplateGenerationLabel: "2"}}}
		outdatedPod := &corev1.Pod{
			ObjectMeta: v1.ObjectMeta{Labels: map[string]string{utils.PodTemplateGenerationLabel: "1"}}}

		UnknownToDoneNode := nodeWithUpgradeState("")
		UnknownToUpgradeRequiredNode := nodeWithUpgradeState("")
		DoneToDoneNode := nodeWithUpgradeState(upgrade.UpgradeStateDone)
		DoneToUpgradeRequiredNode := nodeWithUpgradeState(upgrade.UpgradeStateDone)

		clusterState := upgrade.NewClusterUpgradeState()
		unknownNodes := []*upgrade.NodeUpgradeState{
			{Node: UnknownToDoneNode, DriverPod: upToDatePod, DriverDaemonSet: daemonSet},
			{Node: UnknownToUpgradeRequiredNode, DriverPod: outdatedPod, DriverDaemonSet: daemonSet},
		}
		doneNodes := []*upgrade.NodeUpgradeState{
			{Node: DoneToDoneNode, DriverPod: upToDatePod, DriverDaemonSet: daemonSet},
			{Node: DoneToUpgradeRequiredNode, DriverPod: outdatedPod, DriverDaemonSet: daemonSet},
		}
		clusterState.NodeStates[""] = unknownNodes
		clusterState.NodeStates[upgrade.UpgradeStateDone] = doneNodes

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, &v1alpha1.OfedUpgradePolicySpec{AutoUpgrade: true})).To(Succeed())
		Expect(getNodeUpgradeState(UnknownToDoneNode)).To(Equal(upgrade.UpgradeStateDone))
		Expect(getNodeUpgradeState(UnknownToUpgradeRequiredNode)).To(Equal(upgrade.UpgradeStateUpgradeRequired))
		Expect(getNodeUpgradeState(DoneToDoneNode)).To(Equal(upgrade.UpgradeStateDone))
		Expect(getNodeUpgradeState(DoneToUpgradeRequiredNode)).To(Equal(upgrade.UpgradeStateUpgradeRequired))
	})
	It("UpgradeStateManager should schedule upgrade on all nodes if maxParallel upgrades is set to 0", func() {
		ctx := context.TODO()

		clusterState := upgrade.NewClusterUpgradeState()
		nodeStates := []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
		}

		clusterState.NodeStates[upgrade.UpgradeStateUpgradeRequired] = nodeStates

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
			// Unlimited upgrades
			MaxParallelUpgrades: 0,
		}

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).To(Succeed())
		stateCount := make(map[string]int)
		for i := range nodeStates {
			state := getNodeUpgradeState(clusterState.NodeStates[upgrade.UpgradeStateUpgradeRequired][i].Node)
			stateCount[state]++
		}
		Expect(stateCount[upgrade.UpgradeStateUpgradeRequired]).To(Equal(0))
		Expect(stateCount[upgrade.UpgradeStateDrain]).To(Equal(len(nodeStates)))
	})
	It("UpgradeStateManager should start upgrade on limited amount of nodes "+
		"if maxParallel upgrades is less than node count", func() {
		ctx := context.TODO()

		const maxParallelUpgrades = 3

		clusterState := upgrade.NewClusterUpgradeState()
		nodeStates := []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
		}

		clusterState.NodeStates[upgrade.UpgradeStateUpgradeRequired] = nodeStates

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade:         true,
			MaxParallelUpgrades: maxParallelUpgrades,
		}

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).To(Succeed())
		stateCount := make(map[string]int)
		for i := range nodeStates {
			state := getNodeUpgradeState(nodeStates[i].Node)
			stateCount[state]++
		}
		Expect(stateCount[upgrade.UpgradeStateUpgradeRequired]).To(Equal(2))
		Expect(stateCount[upgrade.UpgradeStateDrain]).To(Equal(maxParallelUpgrades))
	})
	It("UpgradeStateManager should start additional upgrades if maxParallelUpgrades limit is not reached", func() {
		ctx := context.TODO()

		const maxParallelUpgrades = 4

		upgradeRequiredNodes := []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateUpgradeRequired)},
		}
		drainNodes := []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
		}
		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStateUpgradeRequired] = upgradeRequiredNodes
		clusterState.NodeStates[upgrade.UpgradeStateDrain] = drainNodes

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade:         true,
			MaxParallelUpgrades: maxParallelUpgrades,
			DrainSpec: &v1alpha1.DrainSpec{
				Enable: true,
			},
		}

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).To(Succeed())
		stateCount := make(map[string]int)
		for _, state := range append(upgradeRequiredNodes, drainNodes...) {
			state := getNodeUpgradeState(state.Node)
			stateCount[state]++
		}
		Expect(stateCount[upgrade.UpgradeStateUpgradeRequired]).To(Equal(1))
		Expect(stateCount[upgrade.UpgradeStateDrain]).To(Equal(maxParallelUpgrades))
	})
	It("UpgradeStateManager should skip drain if it's disabled by policy", func() {
		ctx := context.TODO()

		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStateDrain] = []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
		}

		policyWithNoDrainSpec := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
		}

		policyWithDisabledDrain := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
			DrainSpec: &v1alpha1.DrainSpec{
				Enable: false,
			},
		}

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policyWithNoDrainSpec)).To(Succeed())
		for _, state := range clusterState.NodeStates[upgrade.UpgradeStateDrain] {
			Expect(getNodeUpgradeState(state.Node)).To(Equal(upgrade.UpgradeStatePodRestart))
		}

		clusterState.NodeStates[upgrade.UpgradeStateDrain] = []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
		}
		Expect(stateManager.ApplyState(ctx, &clusterState, policyWithDisabledDrain)).To(Succeed())
		for _, state := range clusterState.NodeStates[upgrade.UpgradeStateDrain] {
			Expect(getNodeUpgradeState(state.Node)).To(Equal(upgrade.UpgradeStatePodRestart))
		}
	})
	It("UpgradeStateManager should schedule drain for UpgradeStateDrain nodes and pass drain config", func() {
		ctx := context.TODO()

		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStateDrain] = []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
		}

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
			DrainSpec: &v1alpha1.DrainSpec{
				Enable: true,
			},
		}

		drainManagerMock := mocks.DrainManager{}
		drainManagerMock.
			On("ScheduleNodesDrain", mock.Anything, mock.Anything).
			Return(func(ctx context.Context, config *upgrade.DrainConfiguration) error {
				Expect(config.Spec).To(Equal(policy.DrainSpec))
				Expect(config.Nodes).To(HaveLen(3))
				return nil
			})
		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManagerMock, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).To(Succeed())
	})
	It("UpgradeStateManager should fail if drain manager returns an error", func() {
		ctx := context.TODO()

		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStateDrain] = []*upgrade.NodeUpgradeState{
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
			{Node: nodeWithUpgradeState(upgrade.UpgradeStateDrain)},
		}

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
			DrainSpec: &v1alpha1.DrainSpec{
				Enable: true,
			},
		}

		drainManagerMock := mocks.DrainManager{}
		drainManagerMock.
			On("ScheduleNodesDrain", mock.Anything, mock.Anything).
			Return(func(ctx context.Context, config *upgrade.DrainConfiguration) error {
				return errors.New("drain failed")
			})
		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManagerMock, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).ToNot(Succeed())
	})
	It("UpgradeStateManager should not restart pod if it's up to date or already terminating", func() {
		ctx := context.TODO()

		daemonSet := &appsv1.DaemonSet{ObjectMeta: v1.ObjectMeta{Generation: 3}}
		upToDatePod := &corev1.Pod{
			Status:     corev1.PodStatus{Phase: "Running"},
			ObjectMeta: v1.ObjectMeta{Labels: map[string]string{utils.PodTemplateGenerationLabel: "3"}}}
		outdatedRunningPod := &corev1.Pod{
			Status:     corev1.PodStatus{Phase: "Running"},
			ObjectMeta: v1.ObjectMeta{Labels: map[string]string{utils.PodTemplateGenerationLabel: "2"}}}
		outdatedTerminatingPod := &corev1.Pod{
			Status:     corev1.PodStatus{Phase: "Terminating"},
			ObjectMeta: v1.ObjectMeta{Labels: map[string]string{utils.PodTemplateGenerationLabel: "1"}}}

		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStatePodRestart] = []*upgrade.NodeUpgradeState{
			{
				Node:            nodeWithUpgradeState(upgrade.UpgradeStatePodRestart),
				DriverPod:       upToDatePod,
				DriverDaemonSet: daemonSet,
			},
			{
				Node:            nodeWithUpgradeState(upgrade.UpgradeStatePodRestart),
				DriverPod:       outdatedRunningPod,
				DriverDaemonSet: daemonSet,
			},
			{
				Node:            nodeWithUpgradeState(upgrade.UpgradeStatePodRestart),
				DriverPod:       outdatedTerminatingPod,
				DriverDaemonSet: daemonSet,
			},
		}

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
		}

		podDeleteManagerMock := mocks.PodDeleteManager{}
		podDeleteManagerMock.
			On("SchedulePodsRestart", mock.Anything, mock.Anything).
			Return(func(ctx context.Context, podsToDelete []*corev1.Pod) error {
				Expect(podsToDelete).To(HaveLen(1))
				Expect(podsToDelete[0]).To(Equal(outdatedRunningPod))
				return nil
			})
		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManagerMock, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).To(Succeed())
	})
	It("UpgradeStateManager should move pod to UncordonRequired state"+
		"if it's in PodRestart or DrainFailed, up to date and ready", func() {
		ctx := context.TODO()

		daemonSet := &appsv1.DaemonSet{ObjectMeta: v1.ObjectMeta{Generation: 3}}
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				Phase:             "Running",
				ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
			},
			ObjectMeta: v1.ObjectMeta{Labels: map[string]string{utils.PodTemplateGenerationLabel: "3"}}}
		podRestartNode := nodeWithUpgradeState(upgrade.UpgradeStatePodRestart)
		drainFailedNode := nodeWithUpgradeState(upgrade.UpgradeStatePodRestart)

		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStatePodRestart] = []*upgrade.NodeUpgradeState{
			{
				Node:            podRestartNode,
				DriverPod:       pod,
				DriverDaemonSet: daemonSet,
			},
		}
		clusterState.NodeStates[upgrade.UpgradeStateDrainFailed] = []*upgrade.NodeUpgradeState{
			{
				Node:            drainFailedNode,
				DriverPod:       pod,
				DriverDaemonSet: daemonSet,
			},
		}

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
		}

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManager, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).To(Succeed())
		Expect(getNodeUpgradeState(podRestartNode)).To(Equal(upgrade.UpgradeStateUncordonRequired))
		Expect(getNodeUpgradeState(drainFailedNode)).To(Equal(upgrade.UpgradeStateUncordonRequired))
	})
	It("UpgradeStateManager should uncordon UncordonRequired pod and finish upgrade", func() {
		ctx := context.TODO()

		node := nodeWithUpgradeState(upgrade.UpgradeStateUncordonRequired)

		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStateUncordonRequired] = []*upgrade.NodeUpgradeState{
			{
				Node: node,
			},
		}

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
		}

		uncordonManagerMock := mocks.UncordonManager{}
		uncordonManagerMock.
			On("CordonOrUncordonNode", mock.Anything, mock.Anything, mock.Anything).
			Return(func(ctx context.Context, node *corev1.Node, desired bool) error {
				Expect(node).To(Equal(node))
				return nil
			})

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManagerMock, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).To(Succeed())
		Expect(getNodeUpgradeState(node)).To(Equal(upgrade.UpgradeStateDone))
	})
	It("UpgradeStateManager should fail if uncordonManager fails", func() {
		ctx := context.TODO()

		node := nodeWithUpgradeState(upgrade.UpgradeStateUncordonRequired)

		clusterState := upgrade.NewClusterUpgradeState()
		clusterState.NodeStates[upgrade.UpgradeStateUncordonRequired] = []*upgrade.NodeUpgradeState{
			{
				Node: node,
			},
		}

		policy := &v1alpha1.OfedUpgradePolicySpec{
			AutoUpgrade: true,
		}

		uncordonManagerMock := mocks.UncordonManager{}
		uncordonManagerMock.
			On("CordonOrUncordonNode", mock.Anything, mock.Anything, mock.Anything).
			Return(func(ctx context.Context, node *corev1.Node, desired bool) error {
				return errors.New("uncordonManagerFailed")
			})

		stateManager := upgrade.NewClusterUpdateStateManager(
			&drainManager, &podDeleteManager, &uncordonManagerMock, &nodeUpgradeStateProvider, log, k8sClient, k8sInterface)
		Expect(stateManager.ApplyState(ctx, &clusterState, policy)).ToNot(Succeed())
		Expect(getNodeUpgradeState(node)).ToNot(Equal(upgrade.UpgradeStateDone))
	})
})

func nodeWithUpgradeState(state string) *corev1.Node {
	return &corev1.Node{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{upgrade.UpgradeStateAnnotation: state}}}
}
