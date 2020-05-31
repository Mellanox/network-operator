package nodeinfo

/*
 nodeinfo package provides k8s node information. Apart from fetching k8s API Node objects, it wraps the lookup
 of specific attributes (mainly labels) for easier use.
 In the future this may be expanded with a Watcher for K8s API Node objects to provide caching and reduce API calls.
*/

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provider provides Node information
type Provider interface {
	// GetAllNodes gets all nodes that match a given filter criteria.
	// Example for filter criteria according to node labels:
	// 	[]client.ListOption{client.MatchingLabels{"feature.node.kubernetes.io/pci-15b3-1019.present": "true"},}
	GetAllNodes(filters []client.ListOption) (corev1.NodeList, error)
	// GetAllNodesKernelVersion gets kernel version of provided nodes
	GetAllNodesKernelVersion(nodeList *corev1.NodeList) []string
	// GetAllNodesOSNameFull gets the full OS name of nodes that match the given filter criteria
	GetAllNodesOSNameFull(nodeList *corev1.NodeList) []string
	// CreateListOptionForGPUDirectNodes creates []client.ListOption to filter nodes that support GPU direct RDMA
	CreateListOptionForGPUDirectNodes() []client.ListOption
	// CreateListOptionsForMellanoxNICs creates []client.ListOption to filter nodes that have supported Mellanox NICs
	CreateListOptionsForMellanoxNICs() []client.ListOption
}

// NewProvider creates a new Provider object
func NewProvider(k8sAPIlient client.Client) Provider {
	return &provider{client: k8sAPIlient}
}

// provider is an implementation of the Provider interface
type provider struct {
	client client.Client
}

// GetAllNodes gets all nodes that match a given filter criteria.
// Example for filter criteria according to node labels:
// 	[]client.ListOption{client.MatchingLabels{"feature.node.kubernetes.io/pci-15b3-1019.present": "true"},}
func (p *provider) GetAllNodes(filters []client.ListOption) (corev1.NodeList, error) {
	return corev1.NodeList{}, nil
}

// GetAllNodesKernelVersion gets kernel version of provided nodes
func (p *provider) GetAllNodesKernelVersion(nodeList *corev1.NodeList) []string {
	return []string{}
}

// GetAllNodesOSNameFull gets the full OS name of nodes that match the given filter criteria
func (p *provider) GetAllNodesOSNameFull(nodeList *corev1.NodeList) []string {
	return []string{}
}

// CreateListOptionForGPUDirectNodes creates []client.ListOption to filter nodes that support GPU direct RDMA
func (p *provider) CreateListOptionForGPUDirectNodes() []client.ListOption {
	// TODO: fill correct labels
	return []client.ListOption{client.MatchingLabels{"feature.node.kubernetes.io/pci-10de.present": "true"}}
}

// CreateListOptionsForMellanoxNICs creates []client.ListOption to filter nodes that have supported Mellanox NICs
func (p *provider) CreateListOptionsForMellanoxNICs() []client.ListOption {
	// TODO: fill correct labels
	return []client.ListOption{client.MatchingLabels{"feature.node.kubernetes.io/pci-15b3-1019.present": "true"}}
}
