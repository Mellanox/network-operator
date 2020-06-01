package nodeinfo

import corev1 "k8s.io/api/core/v1"

// A Filter applies a filter on a list of Nodes
type Filter interface {
	Apply([]*corev1.Node) []*corev1.Node
}

// A node label filter. use NewNodeLabelFilterBuilder to create instances
type nodeLabelFilter struct {
	labels map[string]string
}

// addLabel adds a label to nodeLabelFilter
func (nlf *nodeLabelFilter) addLabel(key, val string) {
	nlf.labels[key] = val
}

// Apply Filter on Nodes
func (nlf *nodeLabelFilter) Apply([]*corev1.Node) []*corev1.Node {
	//TODO: Implement
	return []*corev1.Node{}
}

// newNodeLabelFilter creates a new nodeLabelFilter
func newNodeLabelFilter() nodeLabelFilter {
	return nodeLabelFilter{labels: make(map[string]string)}
}

// NodeLabelFilterBuilder is a builder for nodeLabelFilter
type NodeLabelFilterBuilder struct {
	filter nodeLabelFilter
}

// NewNodeLabelFilterBuilder returns a new NodeLabelFilterBuilder
func NewNodeLabelFilterBuilder() NodeLabelFilterBuilder {
	return NodeLabelFilterBuilder{filter: newNodeLabelFilter()}
}

// WithLabel adds a label for the Build process of the Label filter
func (b *NodeLabelFilterBuilder) WithLabel(key, val string) *NodeLabelFilterBuilder {
	b.filter.addLabel(key, val)
	return b
}

// Build the Filter
func (b *NodeLabelFilterBuilder) Build() Filter {
	return &b.filter
}

// Reset NodeLabelFilterBuilder
func (b *NodeLabelFilterBuilder) Reset() *NodeLabelFilterBuilder {
	b.filter = newNodeLabelFilter()
	return b
}
