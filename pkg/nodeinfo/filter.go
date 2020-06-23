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

package nodeinfo

import corev1 "k8s.io/api/core/v1"

// A Filter applies a filter on a list of Nodes
type Filter interface {
	// Apply filters a list of nodes according to some internal predicate
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
func (nlf *nodeLabelFilter) Apply(nodes []*corev1.Node) (filtered []*corev1.Node) {
NextIter:
	for _, node := range nodes {
		nodeLabels := node.GetLabels()
		for k, v := range nlf.labels {
			if nodeLabelVal, ok := nodeLabels[k]; ok && nodeLabelVal == v {
				continue
			}
			// label not found on node or label value missmatch
			continue NextIter
		}
		filtered = append(filtered, node)
	}
	return filtered
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
func NewNodeLabelFilterBuilder() *NodeLabelFilterBuilder {
	return &NodeLabelFilterBuilder{filter: newNodeLabelFilter()}
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
