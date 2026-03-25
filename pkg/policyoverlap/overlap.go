/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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

package policyoverlap

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

// NodeOverlap describes which policies select a given node.
type NodeOverlap struct {
	NodeName    string
	PolicyNames []string
}

// ResolveNodesByPolicy returns policyName → set of node names for each policy passing the filter.
// If filter is nil, all policies are included.
func ResolveNodesByPolicy(ctx context.Context, c client.Reader,
	policies []mellanoxv1alpha1.NicNodePolicy,
	filter func(*mellanoxv1alpha1.NicNodePolicy) bool,
) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)

	for i := range policies {
		policy := &policies[i]
		if filter != nil && !filter(policy) {
			continue
		}

		nodeList := &corev1.NodeList{}
		listOpts := []client.ListOption{}
		if len(policy.Spec.NodeSelector) > 0 {
			listOpts = append(listOpts, client.MatchingLabels(policy.Spec.NodeSelector))
		}
		if err := c.List(ctx, nodeList, listOpts...); err != nil {
			return nil, fmt.Errorf("failed to list nodes for policy %s: %w", policy.Name, err)
		}

		nodes := make(map[string]bool, len(nodeList.Items))
		for j := range nodeList.Items {
			nodes[nodeList.Items[j].Name] = true
		}
		result[policy.Name] = nodes
	}

	return result, nil
}

// DetectNodeOverlap checks if any nodes are selected by multiple NicNodePolicies.
// It returns a list of NodeOverlap entries for nodes matched by 2+ policies, or nil if no overlap.
// The `policies` parameter should include ALL policies to check, including the incoming one
// (with its new spec on update).
func DetectNodeOverlap(ctx context.Context, c client.Reader,
	policies []mellanoxv1alpha1.NicNodePolicy) ([]NodeOverlap, error) {
	policyNodes, err := ResolveNodesByPolicy(ctx, c, policies, nil)
	if err != nil {
		return nil, err
	}

	// Invert: nodeName → list of policy names
	nodeToPolices := make(map[string][]string)
	for policyName, nodes := range policyNodes {
		for nodeName := range nodes {
			nodeToPolices[nodeName] = append(nodeToPolices[nodeName], policyName)
		}
	}

	// Filter to overlapping entries
	var overlaps []NodeOverlap
	for nodeName, policyNames := range nodeToPolices {
		if len(policyNames) > 1 {
			sort.Strings(policyNames)
			overlaps = append(overlaps, NodeOverlap{
				NodeName:    nodeName,
				PolicyNames: policyNames,
			})
		}
	}

	// Sort for deterministic output
	sort.Slice(overlaps, func(i, j int) bool {
		return overlaps[i].NodeName < overlaps[j].NodeName
	})

	if len(overlaps) == 0 {
		return nil, nil
	}
	return overlaps, nil
}

// FormatNodeOverlap returns a human-readable error message for node overlaps.
func FormatNodeOverlap(overlaps []NodeOverlap) string {
	var parts []string
	for _, o := range overlaps {
		parts = append(parts, fmt.Sprintf("node %q is selected by policies: [%s]",
			o.NodeName, strings.Join(o.PolicyNames, ", ")))
	}
	return "node selector overlap detected: " + strings.Join(parts, "; ")
}

// SectionConflict describes a section that is defined in both NicClusterPolicy and a NicNodePolicy.
type SectionConflict struct {
	Section        string
	NodePolicyName string
}

// DetectSectionConflict checks if NicClusterPolicy and any NicNodePolicy both define the same
// sections (OFEDDriver, RdmaSharedDevicePlugin, SriovDevicePlugin).
// Returns a list of conflicts, or nil if no conflicts found.
func DetectSectionConflict(clusterPolicy *mellanoxv1alpha1.NicClusterPolicy,
	nodePolicies []mellanoxv1alpha1.NicNodePolicy) []SectionConflict {
	if clusterPolicy == nil {
		return nil
	}

	type sectionCheck struct {
		name           string
		clusterHas     bool
		nodePolicyHas  func(*mellanoxv1alpha1.NicNodePolicy) bool
	}

	checks := []sectionCheck{
		{
			name:       "ofedDriver",
			clusterHas: clusterPolicy.Spec.OFEDDriver != nil,
			nodePolicyHas: func(np *mellanoxv1alpha1.NicNodePolicy) bool {
				return np.Spec.OFEDDriver != nil
			},
		},
		{
			name:       "rdmaSharedDevicePlugin",
			clusterHas: clusterPolicy.Spec.RdmaSharedDevicePlugin != nil,
			nodePolicyHas: func(np *mellanoxv1alpha1.NicNodePolicy) bool {
				return np.Spec.RdmaSharedDevicePlugin != nil
			},
		},
		{
			name:       "sriovDevicePlugin",
			clusterHas: clusterPolicy.Spec.SriovDevicePlugin != nil,
			nodePolicyHas: func(np *mellanoxv1alpha1.NicNodePolicy) bool {
				return np.Spec.SriovDevicePlugin != nil
			},
		},
	}

	var conflicts []SectionConflict
	for _, np := range nodePolicies {
		for _, check := range checks {
			if check.clusterHas && check.nodePolicyHas(&np) {
				conflicts = append(conflicts, SectionConflict{
					Section:        check.name,
					NodePolicyName: np.Name,
				})
			}
		}
	}

	return conflicts
}

// FormatSectionConflicts returns a human-readable error message for section conflicts.
func FormatSectionConflicts(conflicts []SectionConflict, fromNodePolicy bool) string {
	var parts []string
	for _, c := range conflicts {
		if fromNodePolicy {
			parts = append(parts, fmt.Sprintf(
				"NicClusterPolicy already defines %s; NicNodePolicy %q cannot also define it",
				c.Section, c.NodePolicyName))
		} else {
			parts = append(parts, fmt.Sprintf(
				"NicNodePolicy %q already defines %s; NicClusterPolicy cannot also define it",
				c.NodePolicyName, c.Section))
		}
	}
	return strings.Join(parts, "; ")
}

// MatchesNodeSelector checks if a node's labels match the given selector.
func MatchesNodeSelector(node *corev1.Node, selector map[string]string) bool {
	if len(selector) == 0 {
		return true // empty selector matches all nodes
	}
	return labels.SelectorFromSet(labels.Set(selector)).Matches(labels.Set(node.Labels))
}
