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

package upgrade

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/utils"
)

// NodeUpgradeState contains a mapping between a node,
// the driver POD running on them and the daemon set, controlling this pod
type NodeUpgradeState struct {
	Node            *v1.Node
	DriverPod       *v1.Pod
	DriverDaemonSet *appsv1.DaemonSet
}

// ClusterUpgradeState contains a snapshot of the OFED upgrade state in the cluster
// It contains OFED upgrade policy and mappings between nodes and their upgrade state
// Nodes are grouped together with the driver POD running on them and the daemon set, controlling this pod
// This state is then used as an input for the ClusterUpgradeStateManager
type ClusterUpgradeState struct {
	NodeStates map[string][]*NodeUpgradeState
}

// NewClusterUpgradeState creates an empty ClusterUpgradeState object
func NewClusterUpgradeState() ClusterUpgradeState {
	return ClusterUpgradeState{NodeStates: make(map[string][]*NodeUpgradeState)}
}

// ClusterUpgradeStateManager serves as a state machine for the ClusterUpgradeState
// It processes each node and based on its state schedules the required jobs to change their state to the next one
type ClusterUpgradeStateManager struct {
	K8sClient                client.Client
	K8sInterface             kubernetes.Interface
	Log                      logr.Logger
	DrainManager             DrainManager
	PodDeleteManager         PodDeleteManager
	UncordonManager          UncordonManager
	NodeUpgradeStateProvider NodeUpgradeStateProvider
}

// NewClusterUpdateStateManager creates a new instance of ClusterUpgradeStateManager
func NewClusterUpdateStateManager(
	drainManager DrainManager,
	podDeleteManager PodDeleteManager,
	uncordonManager UncordonManager,
	nodeUpgradeStateProvider NodeUpgradeStateProvider,
	log logr.Logger,
	k8sClient client.Client,
	k8sInterface kubernetes.Interface) *ClusterUpgradeStateManager {
	manager := &ClusterUpgradeStateManager{
		DrainManager:             drainManager,
		PodDeleteManager:         podDeleteManager,
		UncordonManager:          uncordonManager,
		NodeUpgradeStateProvider: nodeUpgradeStateProvider,
		Log:                      log,
		K8sClient:                k8sClient,
		K8sInterface:             k8sInterface,
	}

	return manager
}

// ApplyState receives a complete cluster upgrade state and, based on upgrade policy, processes each node's state.
// Based on the current state of the node, it is calculated if the node can be moved to the next state right now
// or whether any actions need to be scheduled for the node to move to the next state.
// The function is stateless and idempotent. If the error was returned before all nodes' states were processed,
// ApplyState would be called again and complete the processing - all the decisions are based on the input data.
func (m *ClusterUpgradeStateManager) ApplyState(ctx context.Context,
	currentState *ClusterUpgradeState, upgradePolicy *v1alpha1.OfedUpgradePolicySpec) error {
	m.Log.V(consts.LogLevelInfo).Info("State Manager, got state update")

	if currentState == nil {
		return fmt.Errorf("currentState should not be empty")
	}

	if upgradePolicy == nil || !upgradePolicy.AutoUpgrade {
		m.Log.V(consts.LogLevelInfo).Info("Driver auto upgrade is disabled, skipping")
		return nil
	}

	m.Log.V(consts.LogLevelInfo).Info("Node states:",
		"Unknown", len(currentState.NodeStates[UpgradeStateUnknown]),
		UpgradeStateDone, len(currentState.NodeStates[UpgradeStateDone]),
		UpgradeStateUpgradeRequired, len(currentState.NodeStates[UpgradeStateUpgradeRequired]),
		UpgradeStateDrain, len(currentState.NodeStates[UpgradeStateDrain]),
		UpgradeStateDrainFailed, len(currentState.NodeStates[UpgradeStateDrainFailed]),
		UpgradeStatePodRestart, len(currentState.NodeStates[UpgradeStatePodRestart]),
		UpgradeStateUncordonRequired, len(currentState.NodeStates[UpgradeStateUncordonRequired]))

	upgradesInProgress := len(currentState.NodeStates[UpgradeStateDrain]) +
		len(currentState.NodeStates[UpgradeStatePodRestart]) +
		len(currentState.NodeStates[UpgradeStateDrainFailed]) +
		len(currentState.NodeStates[UpgradeStateUncordonRequired])

	var upgradesAvailable int
	if upgradePolicy.MaxParallelUpgrades == 0 {
		// Only nodes in UpgradeStateUpgradeRequired can start upgrading, so all of them will move to drain stage
		upgradesAvailable = len(currentState.NodeStates[UpgradeStateUpgradeRequired])
	} else {
		upgradesAvailable = upgradePolicy.MaxParallelUpgrades - upgradesInProgress
	}

	m.Log.V(consts.LogLevelInfo).Info("Upgrades in progress",
		"currently in progress", upgradesInProgress,
		"max parallel upgrades", upgradePolicy.MaxParallelUpgrades,
		"upgrade slots available", upgradesAvailable)

	// First, check if unknown or ready nodes need to be upgraded
	err := m.ProcessDoneOrUnknownNodes(ctx, currentState, UpgradeStateUnknown)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(err, "Failed to process nodes", "state", UpgradeStateUnknown)
		return err
	}
	err = m.ProcessDoneOrUnknownNodes(ctx, currentState, UpgradeStateDone)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(err, "Failed to process nodes", "state", UpgradeStateDone)
		return err
	}
	// Start upgrade process for upgradesAvailable number of nodes
	err = m.ProcessUpgradeRequiredNodes(ctx, currentState, upgradesAvailable)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(
			err, "Failed to process nodes", "state", UpgradeStateUpgradeRequired)
		return err
	}
	// Schedule nodes for drain
	err = m.ProcessDrainNodes(ctx, currentState, upgradePolicy.DrainSpec)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(err, "Failed to schedule nodes drain")
		return err
	}
	err = m.ProcessPodRestartNodes(ctx, currentState)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(err, "Failed to schedule pods restart")
		return err
	}
	err = m.ProcessDrainFailedNodes(ctx, currentState)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(err, "Failed to process nodes which failed to drain")
		return err
	}
	err = m.ProcessUncordonRequiredNodes(ctx, currentState)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(err, "Failed to uncordon nodes")
		return err
	}
	m.Log.V(consts.LogLevelInfo).Info("State Manager, finished processing")
	return nil
}

// ProcessDoneOrUnknownNodes iterates over UpgradeStateDone or UpgradeStateUnknown nodes and determines
// whether each specific node should be in UpgradeStateUpgradeRequired or UpgradeStateDone state.
func (m *ClusterUpgradeStateManager) ProcessDoneOrUnknownNodes(
	ctx context.Context, currentClusterState *ClusterUpgradeState, nodeStateName string) error {
	m.Log.V(consts.LogLevelInfo).Info("ProcessDoneOrUnknownNodes")

	for _, nodeState := range currentClusterState.NodeStates[nodeStateName] {
		podTemplateGeneration, err := utils.GetPodTemplateGeneration(nodeState.DriverPod, m.Log)
		if err != nil {
			m.Log.V(consts.LogLevelError).Error(
				err, "Failed to get pod template generation", "pod", nodeState.DriverPod)
			return err
		}
		if podTemplateGeneration != nodeState.DriverDaemonSet.GetGeneration() {
			err := m.NodeUpgradeStateProvider.ChangeNodeUpgradeState(ctx, nodeState.Node, UpgradeStateUpgradeRequired)
			if err != nil {
				m.Log.V(consts.LogLevelError).Error(
					err, "Failed to change node upgrade state", "state", UpgradeStateUpgradeRequired)
				return err
			}
			m.Log.V(consts.LogLevelInfo).Info("Node requires upgrade, changed its state to UpgradeRequired",
				"node", nodeState.Node.Name)
			continue
		}

		if nodeStateName == UpgradeStateUnknown {
			err := m.NodeUpgradeStateProvider.ChangeNodeUpgradeState(ctx, nodeState.Node, UpgradeStateDone)
			if err != nil {
				m.Log.V(consts.LogLevelError).Error(
					err, "Failed to change node upgrade state", "state", UpgradeStateDone)
				return err
			}
			m.Log.V(consts.LogLevelInfo).Info("Changed node state to UpgradeDOne",
				"node", nodeState.Node.Name)
			continue
		}
		m.Log.V(consts.LogLevelDebug).Info("Node in UpgradeDone state, upgrade not required",
			"node", nodeState.Node.Name)
	}
	return nil
}

// ProcessUpgradeRequiredNodes processes UpgradeStateUpgradeRequired nodes and moves them to UpgradeStateDrain until
// the limit on max parallel upgrades is reached.
func (m *ClusterUpgradeStateManager) ProcessUpgradeRequiredNodes(
	ctx context.Context, currentClusterState *ClusterUpgradeState, limit int) error {
	m.Log.V(consts.LogLevelInfo).Info("ProcessUpgradeRequiredNodes")
	for _, nodeState := range currentClusterState.NodeStates[UpgradeStateUpgradeRequired] {
		if limit <= 0 {
			m.Log.V(consts.LogLevelInfo).Info("Limit for new upgrades is exceeded, skipping the iteration")
			break
		}

		err := m.NodeUpgradeStateProvider.ChangeNodeUpgradeState(ctx, nodeState.Node, UpgradeStateDrain)
		if err == nil {
			limit--
			m.Log.V(consts.LogLevelInfo).Info("Node waiting for drain",
				"node", nodeState.Node.Name)
		} else {
			m.Log.V(consts.LogLevelError).Error(
				err, "Failed to change node upgrade state", "state", UpgradeStateDrain)
			return err
		}
	}

	return nil
}

// ProcessDrainNodes schedules UpgradeStateDrain nodes for drain.
// If drain is disabled by upgrade policy, moves the nodes straight to UpgradeStatePodRestart state.
func (m *ClusterUpgradeStateManager) ProcessDrainNodes(
	ctx context.Context, currentClusterState *ClusterUpgradeState, drainSpec *v1alpha1.DrainSpec) error {
	m.Log.V(consts.LogLevelInfo).Info("ProcessDrainNodes")
	if drainSpec == nil || !drainSpec.Enable {
		// If node drain is disabled, move nodes straight to PodRestart stage
		m.Log.V(consts.LogLevelInfo).Info("Node drain is disabled by policy, skipping this step")
		for _, nodeState := range currentClusterState.NodeStates[UpgradeStateDrain] {
			err := m.NodeUpgradeStateProvider.ChangeNodeUpgradeState(ctx, nodeState.Node, UpgradeStatePodRestart)
			if err != nil {
				m.Log.V(consts.LogLevelError).Error(
					err, "Failed to change node upgrade state", "state", UpgradeStatePodRestart)
				return err
			}
		}
		return nil
	}

	// We want to skip network-operator itself during the drain because the upgrade process might hang
	// if the operator is evicted and can't be rescheduled to any other node, e.g. in a single-node cluster.
	// It's safe to do because the goal of the node draining during the upgrade is to
	// evict pods that might use MOFED and network-operator doesn't use in its own pod.
	skipDrainPodSelector := fmt.Sprintf("%s!=true", OfedUpgradeSkipDrainLabel)
	if drainSpec.PodSelector == "" {
		drainSpec.PodSelector = skipDrainPodSelector
	} else {
		drainSpec.PodSelector = fmt.Sprintf("%s,%s", drainSpec.PodSelector, skipDrainPodSelector)
	}

	drainConfig := DrainConfiguration{
		Spec:  drainSpec,
		Nodes: make([]*v1.Node, 0, len(currentClusterState.NodeStates[UpgradeStateDrain])),
	}
	for _, nodeState := range currentClusterState.NodeStates[UpgradeStateDrain] {
		drainConfig.Nodes = append(drainConfig.Nodes, nodeState.Node)
	}

	return m.DrainManager.ScheduleNodesDrain(ctx, &drainConfig)
}

// ProcessPodRestartNodes processes UpgradeStatePodRestart nodes and schedules driver pod restart for them.
// If the pod has already been restarted and is in Ready state - moves the node to UpgradeStateUncordonRequired state.
func (m *ClusterUpgradeStateManager) ProcessPodRestartNodes(
	ctx context.Context, currentClusterState *ClusterUpgradeState) error {
	m.Log.V(consts.LogLevelInfo).Info("ProcessPodRestartNodes")

	pods := make([]*v1.Pod, 0, len(currentClusterState.NodeStates[UpgradeStatePodRestart]))
	for _, nodeState := range currentClusterState.NodeStates[UpgradeStatePodRestart] {
		podTemplateGeneration, err := utils.GetPodTemplateGeneration(nodeState.DriverPod, m.Log)
		if err != nil {
			m.Log.V(consts.LogLevelError).Error(
				err, "Failed to get pod template generation", "pod", nodeState.DriverPod)
			return err
		}
		if podTemplateGeneration != nodeState.DriverDaemonSet.GetGeneration() {
			// Pods should only be scheduled for restart if they are not terminating or restarting already
			if nodeState.DriverPod.Status.Phase != "Terminating" {
				pods = append(pods, nodeState.DriverPod)
			}
		} else {
			driverPodInSync, err := m.isDriverPodInSync(nodeState)
			if err != nil {
				m.Log.V(consts.LogLevelError).Error(
					err, "Failed to check if driver pod on the node is in sync", "nodeState", nodeState)
				return err
			}
			if driverPodInSync {
				err := m.NodeUpgradeStateProvider.ChangeNodeUpgradeState(
					ctx, nodeState.Node, UpgradeStateUncordonRequired)
				if err != nil {
					m.Log.V(consts.LogLevelError).Error(
						err, "Failed to change node upgrade state", "state", UpgradeStateUncordonRequired)
					return err
				}
			}
		}
	}

	// Create pod restart manager to handle pod restarts
	return m.PodDeleteManager.SchedulePodsRestart(ctx, pods)
}

// ProcessDrainFailedNodes processes UpgradeStateDrainFailed nodes and checks whether the driver pod on the node
// has been successfully restarted. If the pod is in Ready state - moves the node to UpgradeStateUncordonRequired state.
func (m *ClusterUpgradeStateManager) ProcessDrainFailedNodes(
	ctx context.Context, currentClusterState *ClusterUpgradeState) error {
	m.Log.V(consts.LogLevelInfo).Info("ProcessDrainFailedNodes")

	for _, nodeState := range currentClusterState.NodeStates[UpgradeStateDrainFailed] {
		driverPodInSync, err := m.isDriverPodInSync(nodeState)
		if err != nil {
			m.Log.V(consts.LogLevelError).Error(
				err, "Failed to check if driver pod on the node is in sync", "nodeState", nodeState)
			return err
		}
		if driverPodInSync {
			err := m.NodeUpgradeStateProvider.ChangeNodeUpgradeState(ctx, nodeState.Node, UpgradeStateUncordonRequired)
			if err != nil {
				m.Log.V(consts.LogLevelError).Error(
					err, "Failed to change node upgrade state", "state", UpgradeStateUncordonRequired)
				return err
			}
		}
	}

	return nil
}

// ProcessUncordonRequiredNodes processes UpgradeStateUncordonRequired nodes,
// uncordons them and moves them to UpgradeStateDone state
func (m *ClusterUpgradeStateManager) ProcessUncordonRequiredNodes(
	ctx context.Context, currentClusterState *ClusterUpgradeState) error {
	m.Log.V(consts.LogLevelInfo).Info("ProcessUncordonRequiredNodes")

	for _, nodeState := range currentClusterState.NodeStates[UpgradeStateUncordonRequired] {
		err := m.UncordonManager.CordonOrUncordonNode(ctx, nodeState.Node, false)
		if err != nil {
			m.Log.V(consts.LogLevelWarning).Error(
				err, "Node uncordone failed", "node", nodeState.Node)
			return err
		}
		err = m.NodeUpgradeStateProvider.ChangeNodeUpgradeState(ctx, nodeState.Node, UpgradeStateDone)
		if err != nil {
			m.Log.V(consts.LogLevelError).Error(
				err, "Failed to change node upgrade state", "state", UpgradeStateDone)
			return err
		}
	}
	return nil
}

func (m *ClusterUpgradeStateManager) isDriverPodInSync(nodeState *NodeUpgradeState) (bool, error) {
	podTemplateGeneration, err := utils.GetPodTemplateGeneration(nodeState.DriverPod, m.Log)
	if err != nil {
		m.Log.V(consts.LogLevelError).Error(
			err, "Failed to get pod template generation", "pod", nodeState.DriverPod)
		return false, err
	}
	// If the pod generation matches the daemonset generation
	if podTemplateGeneration == nodeState.DriverDaemonSet.GetGeneration() &&
		// And the pod is running
		nodeState.DriverPod.Status.Phase == "Running" &&
		// And it has at least 1 container
		len(nodeState.DriverPod.Status.ContainerStatuses) != 0 {
		for i := range nodeState.DriverPod.Status.ContainerStatuses {
			if !nodeState.DriverPod.Status.ContainerStatuses[i].Ready {
				// Return false if at least 1 container isn't ready
				return false, nil
			}
		}

		// And each container is ready
		return true, nil
	}

	return false, nil
}
