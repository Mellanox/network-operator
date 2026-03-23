/*
Copyright 2026 NVIDIA

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

package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
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

func makeTestNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func makeOFEDPod(name, nodeName, dsOwner string, ready bool) *corev1.Pod {
	labels := map[string]string{consts.OfedDriverLabel: ""}
	if dsOwner != "" {
		labels[consts.DSOwnerLabel] = dsOwner
	}

	containerStatuses := []corev1.ContainerStatus{
		{Ready: ready},
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "nvidia-network-operator",
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			ContainerStatuses: containerStatuses,
		},
	}
}

// --- handleOFEDWaitLabelsForPods tests ---

func TestHandleOFEDWaitLabelsForPods_ReadyPod(t *testing.T) {
	scheme := newTestScheme(t)
	node := makeTestNode("node-a", map[string]string{nodeinfo.NodeLabelWaitOFED: "true"})
	pod := makeOFEDPod("ofed-pod", "node-a", "", true)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, pod).Build()

	err := handleOFEDWaitLabelsForPods(context.Background(), c,
		map[string]string{consts.OfedDriverLabel: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedNode := &corev1.Node{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(node), updatedNode); err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	if updatedNode.Labels[nodeinfo.NodeLabelWaitOFED] != "false" {
		t.Errorf("expected mofed.wait=false for ready pod, got %q",
			updatedNode.Labels[nodeinfo.NodeLabelWaitOFED])
	}
}

func TestHandleOFEDWaitLabelsForPods_NotReadyPod(t *testing.T) {
	scheme := newTestScheme(t)
	node := makeTestNode("node-a", map[string]string{nodeinfo.NodeLabelWaitOFED: "false"})
	pod := makeOFEDPod("ofed-pod", "node-a", "", false)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, pod).Build()

	err := handleOFEDWaitLabelsForPods(context.Background(), c,
		map[string]string{consts.OfedDriverLabel: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedNode := &corev1.Node{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(node), updatedNode); err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	if updatedNode.Labels[nodeinfo.NodeLabelWaitOFED] != "true" {
		t.Errorf("expected mofed.wait=true for not-ready pod, got %q",
			updatedNode.Labels[nodeinfo.NodeLabelWaitOFED])
	}
}

func TestHandleOFEDWaitLabelsForPods_PendingPodSkipped(t *testing.T) {
	scheme := newTestScheme(t)
	// Pod with no NodeName (pending)
	pod := makeOFEDPod("ofed-pod", "", "", false)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()

	err := handleOFEDWaitLabelsForPods(context.Background(), c,
		map[string]string{consts.OfedDriverLabel: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No node to update — just verify no error
}

func TestHandleOFEDWaitLabelsForPods_FiltersByLabels(t *testing.T) {
	scheme := newTestScheme(t)
	nodeA := makeTestNode("node-a", map[string]string{})
	nodeB := makeTestNode("node-b", map[string]string{})
	podA := makeOFEDPod("ofed-ncp", "node-a", "NicClusterPolicy", true)
	podB := makeOFEDPod("ofed-nnp", "node-b", "NicNodePolicy-gpu", true)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodeA, nodeB, podA, podB).Build()

	// Filter to only NNP pods
	err := handleOFEDWaitLabelsForPods(context.Background(), c,
		map[string]string{consts.OfedDriverLabel: "", consts.DSOwnerLabel: "NicNodePolicy-gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// node-b should have mofed.wait=false (NNP pod is ready)
	updatedNodeB := &corev1.Node{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(nodeB), updatedNodeB); err != nil {
		t.Fatalf("failed to get node-b: %v", err)
	}
	if updatedNodeB.Labels[nodeinfo.NodeLabelWaitOFED] != "false" {
		t.Errorf("expected mofed.wait=false on node-b, got %q",
			updatedNodeB.Labels[nodeinfo.NodeLabelWaitOFED])
	}

	// node-a should NOT have been touched (NCP pod, filtered out)
	updatedNodeA := &corev1.Node{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(nodeA), updatedNodeA); err != nil {
		t.Fatalf("failed to get node-a: %v", err)
	}
	if _, exists := updatedNodeA.Labels[nodeinfo.NodeLabelWaitOFED]; exists {
		t.Errorf("expected node-a to not have mofed.wait label, but it has %q",
			updatedNodeA.Labels[nodeinfo.NodeLabelWaitOFED])
	}
}

// --- getNodesManagedByNNPsWithOFED tests ---

func TestGetNodesManagedByNNPsWithOFED_MultipleNNPs(t *testing.T) {
	scheme := newTestScheme(t)
	nodeGPU := makeTestNode("gpu-node", map[string]string{"role": "gpu"})
	nodeCPU := makeTestNode("cpu-node", map[string]string{"role": "cpu"})
	nodeOther := makeTestNode("other-node", map[string]string{"role": "storage"})

	ndpGPU := &mellanoxv1alpha1.NicNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-policy"},
		Spec: mellanoxv1alpha1.NicNodePolicySpec{
			OFEDDriver:   &mellanoxv1alpha1.OFEDDriverSpec{},
			NodeSelector: map[string]string{"role": "gpu"},
		},
	}
	ndpCPU := &mellanoxv1alpha1.NicNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cpu-policy"},
		Spec: mellanoxv1alpha1.NicNodePolicySpec{
			OFEDDriver:   &mellanoxv1alpha1.OFEDDriverSpec{},
			NodeSelector: map[string]string{"role": "cpu"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(nodeGPU, nodeCPU, nodeOther, ndpGPU, ndpCPU).Build()

	result, err := getNodesManagedByNNPsWithOFED(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result["gpu-node"] {
		t.Error("expected gpu-node to be in NNP-managed set")
	}
	if !result["cpu-node"] {
		t.Error("expected cpu-node to be in NNP-managed set")
	}
	if result["other-node"] {
		t.Error("expected other-node NOT to be in NNP-managed set")
	}
}

func TestGetNodesManagedByNNPsWithOFED_NNPWithoutOFED(t *testing.T) {
	scheme := newTestScheme(t)
	node := makeTestNode("node-a", map[string]string{"role": "gpu"})

	nnp := &mellanoxv1alpha1.NicNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "rdma-only"},
		Spec: mellanoxv1alpha1.NicNodePolicySpec{
			RdmaSharedDevicePlugin: &mellanoxv1alpha1.DevicePluginSpec{},
			NodeSelector:           map[string]string{"role": "gpu"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, nnp).Build()

	result, err := getNodesManagedByNNPsWithOFED(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Errorf("expected nil result for NNP without OFED, got %v", result)
	}
}

func TestGetNodesManagedByNNPsWithOFED_NoNNPs(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	result, err := getNodesManagedByNNPsWithOFED(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for no NNPs, got %v", result)
	}
}
