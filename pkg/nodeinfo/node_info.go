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

/*
Package nodeinfo provides k8s node information. Apart from fetching k8s API Node objects, it wraps the lookup
of specific attributes (mainly labels) for easier use.
*/
package nodeinfo

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MellanoxNICListOptions will match on Mellanox NIC bearing Nodes when queried via k8s client
var MellanoxNICListOptions = []client.ListOption{
	client.MatchingLabels{NodeLabelMlnxNIC: "true"}}

// Provider provides Node attributes
//
//go:generate mockery --name Provider
type Provider interface {
	// GetNodePools partitions nodes into one or more node pools for nodes matching the filter criteria
	GetNodePools(filters ...Filter) []NodePool
}

// NewProvider creates a new Provider object
func NewProvider(nodeList []*corev1.Node) Provider {
	return &provider{nodes: nodeList}
}

// provider is an implementation of the Provider interface
type provider struct {
	nodes []*corev1.Node
}

// NodePool represent a set of Nodes grouped by common attributes
type NodePool struct {
	Name             string
	OsName           string
	OsVersion        string
	RhcosVersion     string
	Kernel           string
	Arch             string
	ContainerRuntime string
}

// GetNodePools partitions nodes into one or more node pools. The list of nodes to partition
// is defined by the filters provided as input.
//
// Nodes are partitioned by osVersion-kernelVersion pair.
func (p *provider) GetNodePools(filters ...Filter) []NodePool {
	filtered := p.nodes
	for _, filter := range filters {
		filtered = filter.Apply(filtered)
	}

	nodePoolMap := make(map[string]NodePool)

	for _, node := range filtered {
		nodeLabels := node.GetLabels()

		nodePool := NodePool{}
		osName, ok := nodeLabels[NodeLabelOSName]
		if !ok {
			log.Info("WARNING: Could not find NFD labels for node. Is NFD installed?",
				"Node", node.Name, "Label", NodeLabelOSName)
			continue
		}
		nodePool.OsName = osName

		osVersion, ok := nodeLabels[NodeLabelOSVer]
		if !ok {
			log.Info("WARNING: Could not find NFD labels for node. Is NFD installed?",
				"Node", node.Name, "Label", NodeLabelOSVer)
			continue
		}
		nodePool.OsVersion = osVersion

		arch, ok := nodeLabels[NodeLabelCPUArch]
		if !ok {
			log.Info("WARNING: Could not find NFD labels for node. Is NFD installed?",
				"Node", node.Name, "Label", NodeLabelCPUArch)
			continue
		}
		nodePool.Arch = arch

		rhcos, ok := nodeLabels[NodeLabelOSTreeVersion]
		if ok {
			nodePool.RhcosVersion = rhcos
		}

		kernel, ok := nodeLabels[NodeLabelKernelVerFull]
		if !ok {
			log.Info("WARNING: Could not find NFD labels for node. Is NFD installed?",
				"Node", node.Name, "Label", NodeLabelKernelVerFull)
			continue
		}
		nodePool.Kernel = kernel

		nodePool.ContainerRuntime = getContainerRuntime(node)

		nodePool.Name = fmt.Sprintf("%s%s-%s", nodePool.OsName, nodePool.OsVersion, nodePool.Kernel)

		if _, exists := nodePoolMap[nodePool.Name]; !exists {
			nodePoolMap[nodePool.Name] = nodePool
			log.Info("NodePool found", "name", nodePool.Name)
		}
	}

	nodePools := make([]NodePool, 0)
	for _, np := range nodePoolMap {
		nodePools = append(nodePools, np)
	}

	return nodePools
}

func getContainerRuntime(node *corev1.Node) string {
	// runtimeVer string will look like <runtime>://<x.y.z>
	runtimeVer := node.Status.NodeInfo.ContainerRuntimeVersion
	var runtime string
	switch {
	case strings.HasPrefix(runtimeVer, "docker"):
		runtime = Docker
	case strings.HasPrefix(runtimeVer, "containerd"):
		runtime = Containerd
	case strings.HasPrefix(runtimeVer, "cri-o"):
		runtime = CRIO
	default:
		runtime = ""
	}
	return runtime
}
