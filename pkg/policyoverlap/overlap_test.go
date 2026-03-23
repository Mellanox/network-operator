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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("failed to add client-go scheme: %v", err)
	}
	if err := mellanoxv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add mellanox scheme: %v", err)
	}
	return s
}

func makeNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func makeNicNodePolicy(name string, selector map[string]string) mellanoxv1alpha1.NicNodePolicy {
	return mellanoxv1alpha1.NicNodePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: mellanoxv1alpha1.NicNodePolicySpec{
			NodeSelector: selector,
		},
	}
}

// --- DetectNodeOverlap tests ---

func TestDetectNodeOverlap_OverlappingSelectors(t *testing.T) {
	scheme := newScheme(t)
	node1 := makeNode("node-a", map[string]string{"role": "gpu", "zone": "us-east"})
	node2 := makeNode("node-b", map[string]string{"role": "gpu", "zone": "us-west"})
	node3 := makeNode("node-c", map[string]string{"role": "cpu"})

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node1, node2, node3).Build()

	policies := []mellanoxv1alpha1.NicNodePolicy{
		makeNicNodePolicy("policy-1", map[string]string{"role": "gpu"}),
		makeNicNodePolicy("policy-2", map[string]string{"role": "gpu"}),
	}

	overlaps, err := DetectNodeOverlap(context.Background(), c, policies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(overlaps) != 2 {
		t.Fatalf("expected 2 overlapping nodes, got %d", len(overlaps))
	}
	// Results should be sorted by node name
	if overlaps[0].NodeName != "node-a" {
		t.Errorf("expected first overlap node to be node-a, got %s", overlaps[0].NodeName)
	}
	if overlaps[1].NodeName != "node-b" {
		t.Errorf("expected second overlap node to be node-b, got %s", overlaps[1].NodeName)
	}
	for _, o := range overlaps {
		if len(o.PolicyNames) != 2 {
			t.Errorf("expected 2 policies for node %s, got %d", o.NodeName, len(o.PolicyNames))
		}
	}
}

func TestDetectNodeOverlap_DisjointSelectors(t *testing.T) {
	scheme := newScheme(t)
	node1 := makeNode("node-a", map[string]string{"role": "gpu"})
	node2 := makeNode("node-b", map[string]string{"role": "cpu"})

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node1, node2).Build()

	policies := []mellanoxv1alpha1.NicNodePolicy{
		makeNicNodePolicy("policy-gpu", map[string]string{"role": "gpu"}),
		makeNicNodePolicy("policy-cpu", map[string]string{"role": "cpu"}),
	}

	overlaps, err := DetectNodeOverlap(context.Background(), c, policies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overlaps != nil {
		t.Fatalf("expected nil overlaps for disjoint selectors, got %v", overlaps)
	}
}

func TestDetectNodeOverlap_EmptySelector(t *testing.T) {
	scheme := newScheme(t)
	node1 := makeNode("node-a", map[string]string{"role": "gpu"})
	node2 := makeNode("node-b", map[string]string{"role": "cpu"})

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node1, node2).Build()

	policies := []mellanoxv1alpha1.NicNodePolicy{
		makeNicNodePolicy("policy-all", map[string]string{}),
		makeNicNodePolicy("policy-gpu", map[string]string{"role": "gpu"}),
	}

	overlaps, err := DetectNodeOverlap(context.Background(), c, policies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(overlaps) == 0 {
		t.Fatal("expected overlaps when one policy has empty selector, got none")
	}
	// node-a should be in the overlap (matched by both policies)
	found := false
	for _, o := range overlaps {
		if o.NodeName == "node-a" {
			found = true
			if len(o.PolicyNames) != 2 {
				t.Errorf("expected node-a to be selected by 2 policies, got %d", len(o.PolicyNames))
			}
		}
	}
	if !found {
		t.Error("expected node-a to appear in overlaps")
	}
	// node-b should also overlap since empty selector matches all
	foundB := false
	for _, o := range overlaps {
		if o.NodeName == "node-b" {
			foundB = true
		}
	}
	// node-b is only matched by policy-all, not by policy-gpu, so it should NOT overlap
	if foundB {
		t.Error("node-b should not be in overlap since policy-gpu does not select it")
	}
}

func TestDetectNodeOverlap_SinglePolicy(t *testing.T) {
	scheme := newScheme(t)
	node1 := makeNode("node-a", map[string]string{"role": "gpu"})

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node1).Build()

	policies := []mellanoxv1alpha1.NicNodePolicy{
		makeNicNodePolicy("policy-1", map[string]string{"role": "gpu"}),
	}

	overlaps, err := DetectNodeOverlap(context.Background(), c, policies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overlaps != nil {
		t.Fatalf("expected nil overlaps for single policy, got %v", overlaps)
	}
}

func TestDetectNodeOverlap_NoPolicies(t *testing.T) {
	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	overlaps, err := DetectNodeOverlap(context.Background(), c, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overlaps != nil {
		t.Fatalf("expected nil overlaps for no policies, got %v", overlaps)
	}
}

func TestDetectNodeOverlap_ThreePoliciesPartialOverlap(t *testing.T) {
	scheme := newScheme(t)
	node1 := makeNode("node-a", map[string]string{"role": "gpu", "zone": "east"})
	node2 := makeNode("node-b", map[string]string{"role": "gpu", "zone": "west"})
	node3 := makeNode("node-c", map[string]string{"role": "cpu", "zone": "east"})

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node1, node2, node3).Build()

	policies := []mellanoxv1alpha1.NicNodePolicy{
		makeNicNodePolicy("policy-gpu", map[string]string{"role": "gpu"}),
		makeNicNodePolicy("policy-east", map[string]string{"zone": "east"}),
		makeNicNodePolicy("policy-cpu", map[string]string{"role": "cpu"}),
	}

	overlaps, err := DetectNodeOverlap(context.Background(), c, policies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// node-a: matched by policy-gpu and policy-east -> overlap
	// node-c: matched by policy-east and policy-cpu -> overlap
	// node-b: matched only by policy-gpu -> no overlap
	if len(overlaps) != 2 {
		t.Fatalf("expected 2 overlapping nodes, got %d: %v", len(overlaps), overlaps)
	}
	if overlaps[0].NodeName != "node-a" || overlaps[1].NodeName != "node-c" {
		t.Errorf("expected nodes [node-a, node-c], got [%s, %s]", overlaps[0].NodeName, overlaps[1].NodeName)
	}
}

// --- FormatNodeOverlap tests ---

func TestFormatNodeOverlap(t *testing.T) {
	overlaps := []NodeOverlap{
		{NodeName: "node-a", PolicyNames: []string{"policy-1", "policy-2"}},
		{NodeName: "node-b", PolicyNames: []string{"policy-2", "policy-3"}},
	}

	result := FormatNodeOverlap(overlaps)

	expected := `node selector overlap detected: ` +
		`node "node-a" is selected by policies: [policy-1, policy-2]; ` +
		`node "node-b" is selected by policies: [policy-2, policy-3]`
	if result != expected {
		t.Errorf("unexpected format result.\ngot:  %s\nwant: %s", result, expected)
	}
}

func TestFormatNodeOverlap_SingleOverlap(t *testing.T) {
	overlaps := []NodeOverlap{
		{NodeName: "worker-1", PolicyNames: []string{"a", "b", "c"}},
	}

	result := FormatNodeOverlap(overlaps)
	expected := `node selector overlap detected: node "worker-1" is selected by policies: [a, b, c]`
	if result != expected {
		t.Errorf("unexpected format result.\ngot:  %s\nwant: %s", result, expected)
	}
}

// --- DetectSectionConflict tests ---

func TestDetectSectionConflict_OFEDConflict(t *testing.T) {
	clusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{
		Spec: mellanoxv1alpha1.NicClusterPolicySpec{
			OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
		},
	}
	nodePolicies := []mellanoxv1alpha1.NicNodePolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "np-1"},
			Spec: mellanoxv1alpha1.NicNodePolicySpec{
				OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
			},
		},
	}

	conflicts := DetectSectionConflict(clusterPolicy, nodePolicies)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Section != "ofedDriver" {
		t.Errorf("expected section ofedDriver, got %s", conflicts[0].Section)
	}
	if conflicts[0].NodePolicyName != "np-1" {
		t.Errorf("expected node policy name np-1, got %s", conflicts[0].NodePolicyName)
	}
}

func TestDetectSectionConflict_NoConflict(t *testing.T) {
	clusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{
		Spec: mellanoxv1alpha1.NicClusterPolicySpec{
			OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
		},
	}
	nodePolicies := []mellanoxv1alpha1.NicNodePolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "np-1"},
			Spec: mellanoxv1alpha1.NicNodePolicySpec{
				RdmaSharedDevicePlugin: &mellanoxv1alpha1.DevicePluginSpec{},
			},
		},
	}

	conflicts := DetectSectionConflict(clusterPolicy, nodePolicies)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestDetectSectionConflict_NilClusterPolicy(t *testing.T) {
	nodePolicies := []mellanoxv1alpha1.NicNodePolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "np-1"},
			Spec: mellanoxv1alpha1.NicNodePolicySpec{
				OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
			},
		},
	}

	conflicts := DetectSectionConflict(nil, nodePolicies)
	if conflicts != nil {
		t.Fatalf("expected nil conflicts for nil cluster policy, got %v", conflicts)
	}
}

func TestDetectSectionConflict_MultipleSections(t *testing.T) {
	clusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{
		Spec: mellanoxv1alpha1.NicClusterPolicySpec{
			OFEDDriver:             &mellanoxv1alpha1.OFEDDriverSpec{},
			RdmaSharedDevicePlugin: &mellanoxv1alpha1.DevicePluginSpec{},
			SriovDevicePlugin:      &mellanoxv1alpha1.DevicePluginSpec{},
		},
	}
	nodePolicies := []mellanoxv1alpha1.NicNodePolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "np-1"},
			Spec: mellanoxv1alpha1.NicNodePolicySpec{
				OFEDDriver:             &mellanoxv1alpha1.OFEDDriverSpec{},
				RdmaSharedDevicePlugin: &mellanoxv1alpha1.DevicePluginSpec{},
				SriovDevicePlugin:      &mellanoxv1alpha1.DevicePluginSpec{},
			},
		},
	}

	conflicts := DetectSectionConflict(clusterPolicy, nodePolicies)
	if len(conflicts) != 3 {
		t.Fatalf("expected 3 conflicts, got %d: %v", len(conflicts), conflicts)
	}

	expectedSections := map[string]bool{
		"ofedDriver":             false,
		"rdmaSharedDevicePlugin": false,
		"sriovDevicePlugin":      false,
	}
	for _, c := range conflicts {
		if _, ok := expectedSections[c.Section]; !ok {
			t.Errorf("unexpected section in conflict: %s", c.Section)
		}
		expectedSections[c.Section] = true
	}
	for section, found := range expectedSections {
		if !found {
			t.Errorf("missing expected conflict for section: %s", section)
		}
	}
}

func TestDetectSectionConflict_MultipleNodePolicies(t *testing.T) {
	clusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{
		Spec: mellanoxv1alpha1.NicClusterPolicySpec{
			OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
		},
	}
	nodePolicies := []mellanoxv1alpha1.NicNodePolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "np-1"},
			Spec: mellanoxv1alpha1.NicNodePolicySpec{
				OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "np-2"},
			Spec: mellanoxv1alpha1.NicNodePolicySpec{
				OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
			},
		},
	}

	conflicts := DetectSectionConflict(clusterPolicy, nodePolicies)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
	}
	if conflicts[0].NodePolicyName != "np-1" || conflicts[1].NodePolicyName != "np-2" {
		t.Errorf("expected conflicts from np-1 and np-2, got %s and %s",
			conflicts[0].NodePolicyName, conflicts[1].NodePolicyName)
	}
}

func TestDetectSectionConflict_EmptyNodePolicies(t *testing.T) {
	clusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{
		Spec: mellanoxv1alpha1.NicClusterPolicySpec{
			OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{},
		},
	}

	conflicts := DetectSectionConflict(clusterPolicy, nil)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts for empty node policies, got %d", len(conflicts))
	}
}

// --- FormatSectionConflicts tests ---

func TestFormatSectionConflicts_FromNodePolicy(t *testing.T) {
	conflicts := []SectionConflict{
		{Section: "ofedDriver", NodePolicyName: "np-1"},
	}

	result := FormatSectionConflicts(conflicts, true)
	expected := "NicClusterPolicy already defines ofedDriver; NicNodePolicy \"np-1\" cannot also define it"
	if result != expected {
		t.Errorf("unexpected format result.\ngot:  %s\nwant: %s", result, expected)
	}
}

func TestFormatSectionConflicts_FromClusterPolicy(t *testing.T) {
	conflicts := []SectionConflict{
		{Section: "ofedDriver", NodePolicyName: "np-1"},
	}

	result := FormatSectionConflicts(conflicts, false)
	expected := "NicNodePolicy \"np-1\" already defines ofedDriver; NicClusterPolicy cannot also define it"
	if result != expected {
		t.Errorf("unexpected format result.\ngot:  %s\nwant: %s", result, expected)
	}
}

func TestFormatSectionConflicts_MultipleConflicts(t *testing.T) {
	conflicts := []SectionConflict{
		{Section: "ofedDriver", NodePolicyName: "np-1"},
		{Section: "sriovDevicePlugin", NodePolicyName: "np-2"},
	}

	result := FormatSectionConflicts(conflicts, true)
	expected := `NicClusterPolicy already defines ofedDriver; NicNodePolicy "np-1" cannot also define it; ` +
		`NicClusterPolicy already defines sriovDevicePlugin; NicNodePolicy "np-2" cannot also define it`
	if result != expected {
		t.Errorf("unexpected format result.\ngot:  %s\nwant: %s", result, expected)
	}
}

// --- MatchesNodeSelector tests ---

func TestMatchesNodeSelector_MatchingLabels(t *testing.T) {
	node := makeNode("node-a", map[string]string{"role": "gpu", "zone": "east"})
	selector := map[string]string{"role": "gpu"}

	if !MatchesNodeSelector(node, selector) {
		t.Error("expected node to match selector")
	}
}

func TestMatchesNodeSelector_NonMatchingLabels(t *testing.T) {
	node := makeNode("node-a", map[string]string{"role": "cpu"})
	selector := map[string]string{"role": "gpu"}

	if MatchesNodeSelector(node, selector) {
		t.Error("expected node not to match selector")
	}
}

func TestMatchesNodeSelector_EmptySelector(t *testing.T) {
	node := makeNode("node-a", map[string]string{"role": "gpu"})

	if !MatchesNodeSelector(node, map[string]string{}) {
		t.Error("expected empty selector to match all nodes")
	}
	if !MatchesNodeSelector(node, nil) {
		t.Error("expected nil selector to match all nodes")
	}
}

func TestMatchesNodeSelector_EmptyNodeLabels(t *testing.T) {
	node := makeNode("node-a", map[string]string{})
	selector := map[string]string{"role": "gpu"}

	if MatchesNodeSelector(node, selector) {
		t.Error("expected node with no labels not to match selector")
	}
}

func TestMatchesNodeSelector_MultipleLabelsInSelector(t *testing.T) {
	node := makeNode("node-a", map[string]string{"role": "gpu", "zone": "east", "tier": "high"})
	selector := map[string]string{"role": "gpu", "zone": "east"}

	if !MatchesNodeSelector(node, selector) {
		t.Error("expected node to match multi-label selector")
	}

	// Missing one label
	nodePartial := makeNode("node-b", map[string]string{"role": "gpu"})
	if MatchesNodeSelector(nodePartial, selector) {
		t.Error("expected node missing a selector label not to match")
	}
}
