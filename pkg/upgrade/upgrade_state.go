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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
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
