package nodeinfo

/*
 nodeinfo package provides k8s node information. Apart from fetching k8s API Node objects, it wraps the lookup
 of specific attributes (mainly labels) for easier use.
*/

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: fill correct labels

// MellanoxNICListOptions will match on Mellanox NIC bearing Nodes when queried via k8s client
var MellanoxNICListOptions = []client.ListOption{
	client.MatchingLabels{"feature.node.kubernetes.io/pci-15b3-1019.present": "true"}}

type AttributeType int

const (
	AttrTypeOS = iota
	AttrTypeKernel
)

// NodeAttributes provides attributes of a specific node
type NodeAttributes struct {
	// Node Name
	Name string
	// Node Attributes
	Attributes map[AttributeType]string
}

// Provider provides Node information
type Provider interface {
	// GetNodesAttributes retrieves node attributes for nodes matching the filter criteria
	GetNodesAttributes(filters ...Filter) []NodeAttributes
}

// NewProvider creates a new Provider object
func NewProvider(nodeList []*corev1.Node) Provider {
	return &provider{nodes: nodeList}
}

// provider is an implementation of the Provider interface
type provider struct {
	nodes []*corev1.Node
}

// GetNodesKernelVersion gets kernel versions of provided nodes
func (p *provider) GetNodesAttributes(filters ...Filter) []NodeAttributes {
	return []NodeAttributes{}
}
