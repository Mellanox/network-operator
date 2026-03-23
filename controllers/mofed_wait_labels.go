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

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pkg/errors"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/policyoverlap"
)

// setNodeLabel sets the value for the given label on a node.
// If value is "", the label is removed.
func setNodeLabel(ctx context.Context, c client.Client, node, label, value string) error {
	reqLogger := log.FromContext(ctx)
	var patch []byte
	if value == "" {
		patch = []byte(fmt.Sprintf(`{"metadata":{"labels":{%q: null}}}`, label))
		reqLogger.V(consts.LogLevelDebug).Info("remove given label from the node", "node", node, "label", label)
	} else {
		patch = []byte(fmt.Sprintf(`{"metadata":{"labels":{%q: %q}}}`, label, value))
		reqLogger.V(consts.LogLevelDebug).Info("update given label for the node",
			"node", node, "label", label, "value", value)
	}

	err := c.Patch(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: node,
		},
	}, client.RawPatch(types.StrategicMergePatchType, patch))

	if err != nil {
		return errors.Wrapf(err, "unable to patch %s label for node %s", label, node)
	}
	return nil
}

// handleOFEDWaitLabelsForPodsWithFallback tries the primary label selector, and if it matches
// zero pods, falls back to the fallback selector. This handles the OnDelete upgrade migration
// where existing OFED pods don't yet have the ds-owner label on the pod template.
func handleOFEDWaitLabelsForPodsWithFallback(ctx context.Context, c client.Client,
	primaryLabels, fallbackLabels map[string]string) error {
	reqLogger := log.FromContext(ctx)
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.MatchingLabels(primaryLabels)); err != nil {
		return errors.Wrap(err, "failed to list OFED pods with primary selector")
	}
	if len(pods.Items) == 0 && fallbackLabels != nil {
		reqLogger.V(consts.LogLevelDebug).Info(
			"no pods matched primary selector, falling back to broader selector")
		if err := c.List(ctx, pods, client.MatchingLabels(fallbackLabels)); err != nil {
			return errors.Wrap(err, "failed to list OFED pods with fallback selector")
		}
	}
	return processOFEDPodsForWaitLabels(ctx, c, pods)
}

// handleOFEDWaitLabelsForPods lists OFED pods matching the given labels,
// checks container readiness, and sets mofed.wait on each pod's node.
// Pods without a NodeName (pending) are skipped.
func handleOFEDWaitLabelsForPods(ctx context.Context, c client.Client,
	matchLabels map[string]string) error {
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.MatchingLabels(matchLabels)); err != nil {
		return errors.Wrap(err, "failed to list OFED pods")
	}
	return processOFEDPodsForWaitLabels(ctx, c, pods)
}

// processOFEDPodsForWaitLabels iterates over OFED pods, checks mofed-container readiness,
// and sets mofed.wait on each pod's node. Pods without a NodeName (pending) are skipped.
func processOFEDPodsForWaitLabels(ctx context.Context, c client.Client, pods *corev1.PodList) error {
	reqLogger := log.FromContext(ctx)
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Spec.NodeName == "" {
			continue
		}
		labelValue := "true"
		// Check readiness of the mofed-container specifically.
		// On OCP, OFED pods may include a DTK (Driver Toolkit) sidecar container.
		for j := range pod.Status.ContainerStatuses {
			if pod.Status.ContainerStatuses[j].Name == "mofed-container" {
				if pod.Status.ContainerStatuses[j].Ready {
					reqLogger.V(consts.LogLevelDebug).Info("OFED Pod is ready on the node",
						"node", pod.Spec.NodeName)
					labelValue = "false"
				}
				break
			}
		}
		if err := setNodeLabel(ctx, c, pod.Spec.NodeName, nodeinfo.NodeLabelWaitOFED, labelValue); err != nil {
			return err
		}
	}
	return nil
}

// clearOrphanedMOFEDWaitLabels finds nodes with mofed.wait=true that have no running OFED pods
// and sets mofed.wait=false on them. This handles cleanup after NNP deletion when all pods are gone.
func clearOrphanedMOFEDWaitLabels(ctx context.Context, c client.Client) error {
	reqLogger := log.FromContext(ctx)

	// List all OFED pods to find which nodes still have them
	ofedPods := &corev1.PodList{}
	if err := c.List(ctx, ofedPods, client.MatchingLabels{consts.OfedDriverLabel: ""}); err != nil {
		return errors.Wrap(err, "failed to list OFED pods")
	}
	nodesWithOFEDPod := make(map[string]bool)
	for i := range ofedPods.Items {
		if ofedPods.Items[i].Spec.NodeName != "" {
			nodesWithOFEDPod[ofedPods.Items[i].Spec.NodeName] = true
		}
	}

	// List nodes with mofed.wait=true
	nodeList := &corev1.NodeList{}
	if err := c.List(ctx, nodeList, client.MatchingLabels{
		nodeinfo.NodeLabelWaitOFED: "true",
	}); err != nil {
		return errors.Wrap(err, "failed to list nodes with mofed.wait=true")
	}

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if !nodesWithOFEDPod[node.Name] {
			reqLogger.V(consts.LogLevelDebug).Info("clearing orphaned mofed.wait label",
				"node", node.Name)
			if err := setNodeLabel(ctx, c, node.Name, nodeinfo.NodeLabelWaitOFED, "false"); err != nil {
				return err
			}
		}
	}
	return nil
}

// getNodesManagedByNNPsWithOFED returns the set of node names managed by NicNodePolicies
// that have ofedDriver configured. Used by the NCP controller to exclude these nodes
// from its fallback mofed.wait label management.
func getNodesManagedByNNPsWithOFED(ctx context.Context, c client.Client) (map[string]bool, error) {
	nodePolicyList := &mellanoxv1alpha1.NicNodePolicyList{}
	if err := c.List(ctx, nodePolicyList); err != nil {
		return nil, errors.Wrap(err, "failed to list NicNodePolicies")
	}

	if len(nodePolicyList.Items) == 0 {
		return nil, nil
	}

	hasOFED := func(np *mellanoxv1alpha1.NicNodePolicy) bool {
		return np.Spec.OFEDDriver != nil
	}

	policyNodes, err := policyoverlap.ResolveNodesByPolicy(ctx, c, nodePolicyList.Items, hasOFED)
	if err != nil {
		return nil, err
	}

	// Flatten to a single set
	result := make(map[string]bool)
	for _, nodes := range policyNodes {
		for nodeName := range nodes {
			result[nodeName] = true
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}
