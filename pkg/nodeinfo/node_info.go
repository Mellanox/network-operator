package nodeinfo

/*
 nodeinfo package provides k8s node information. Apart from fetching k8s API Node objects, it wraps the lookup
 of specific attributes (mainly labels) for easier use.
*/

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provider provides Node information
type Provider interface {
	// ListNodes gets all nodes that match a given filter criteria.
	// Example for filter criteria according to node labels:
	// 	[]client.ListOption{client.MatchingLabels{"feature.node.kubernetes.io/pci-15b3-1019.present": "true"},}
	ListNodes(filters []client.ListOption) ([]corev1.Node, error)
	// GetNodesKernelVersion gets kernel versions of provided nodes
	GetNodesKernelVersion(nodeList []*corev1.Node) []string
	// GetNodesOSNameFull gets the full OS name of nodes that match the given filter criteria
	GetNodesOSNameFull(nodeList []*corev1.Node) []string
	// CreateListOptionsForMellanoxNICs creates []client.ListOption to filter nodes that have supported Mellanox NICs
	CreateListOptionsForMellanoxNICs() []client.ListOption
}

// NewProvider creates a new Provider object
func NewProvider(k8sAPIClient client.Client) Provider {
	return &provider{client: k8sAPIClient}
}

// provider is an implementation of the Provider interface
type provider struct {
	client client.Client
}

// ListNodes gets all nodes that match a given filter criteria.
// Example for filter criteria according to node labels:
// 	[]client.ListOption{client.MatchingLabels{"feature.node.kubernetes.io/pci-15b3-1019.present": "true"},}
func (p *provider) ListNodes(filters []client.ListOption) ([]corev1.Node, error) {
	return []corev1.Node{}, nil
}

// GetNodesKernelVersion gets kernel versions of provided nodes
func (p *provider) GetNodesKernelVersion(nodeList []*corev1.Node) []string {
	return []string{}
}

// GetNodesOSNameFull gets the full OS name of nodes that match the given filter criteria
func (p *provider) GetNodesOSNameFull(nodeList []*corev1.Node) []string {
	return []string{}
}

// CreateListOptionsForMellanoxNICs creates []client.ListOption to filter nodes that have supported Mellanox NICs
func (p *provider) CreateListOptionsForMellanoxNICs() []client.ListOption {
	// TODO: fill correct labels
	return []client.ListOption{client.MatchingLabels{"feature.node.kubernetes.io/pci-15b3-1019.present": "true"}}
}
