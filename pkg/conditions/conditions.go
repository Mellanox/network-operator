/*
Copyright 2025 NVIDIA

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

// Package conditions provides helpers for managing standard Kubernetes conditions
// on the NicClusterPolicy CRD status.
package conditions

import (
	"fmt"
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

// stateNameToConditionType maps the internal state machine name (state.Result.StateName)
// to the corresponding per-component Kubernetes condition type.
var stateNameToConditionType = map[string]string{
	"state-OFED":                         mellanoxv1alpha1.ConditionTypeOFEDDriverReady,
	"state-RDMA-device-plugin":           mellanoxv1alpha1.ConditionTypeRDMASharedDevicePluginReady,
	"state-SRIOV-device-plugin":          mellanoxv1alpha1.ConditionTypeSRIOVDevicePluginReady,
	"state-ib-kubernetes":                mellanoxv1alpha1.ConditionTypeIBKubernetesReady,
	"state-multus-cni":                   mellanoxv1alpha1.ConditionTypeMultusCNIReady,
	"state-container-networking-plugins": mellanoxv1alpha1.ConditionTypeCNIPluginsReady,
	"state-ipoib-cni":                    mellanoxv1alpha1.ConditionTypeIPoIBCNIReady,
	"state-nv-ipam-cni":                  mellanoxv1alpha1.ConditionTypeNVIPAMReady,
	"state-nic-feature-discovery":        mellanoxv1alpha1.ConditionTypeNICFeatureDiscoveryReady,
	"state-doca-telemetry-service":       mellanoxv1alpha1.ConditionTypeDOCATelemetryServiceReady,
	"state-nic-configuration-operator":   mellanoxv1alpha1.ConditionTypeNICConfigurationOperatorReady,
	"state-spectrum-x-operator":          mellanoxv1alpha1.ConditionTypeSpectrumXOperatorReady,
}

// ComponentConditionFromResult converts a single state.Result into a metav1.Condition for
// the corresponding component. The second return value is false when the state name is not
// recognized and the result should be skipped.
func ComponentConditionFromResult(result state.Result, generation int64) (metav1.Condition, bool) {
	condType, ok := stateNameToConditionType[result.StateName]
	if !ok {
		return metav1.Condition{}, false
	}

	var condStatus metav1.ConditionStatus
	var reason, message string

	switch result.Status {
	case state.SyncStateReady:
		condStatus = metav1.ConditionTrue
		reason = mellanoxv1alpha1.ConditionReasonComponentReady
	case state.SyncStateIgnore:
		// Component not configured in spec — not a problem.
		condStatus = metav1.ConditionTrue
		reason = mellanoxv1alpha1.ConditionReasonComponentNotRequired
		message = "Component not configured in spec"
	case state.SyncStateNotReady:
		condStatus = metav1.ConditionFalse
		reason = mellanoxv1alpha1.ConditionReasonComponentNotReady
		if result.ErrInfo != nil {
			message = result.ErrInfo.Error()
		}
	case state.SyncStateError:
		condStatus = metav1.ConditionFalse
		reason = mellanoxv1alpha1.ConditionReasonComponentError
		if result.ErrInfo != nil {
			message = result.ErrInfo.Error()
		}
	default:
		condStatus = metav1.ConditionUnknown
		reason = mellanoxv1alpha1.ConditionReasonComponentNotReady
		message = fmt.Sprintf("Unrecognized sync state: %s", result.Status)
	}

	return metav1.Condition{
		Type:               condType,
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	}, true
}

// aggregateReadyCondition derives the Ready aggregate condition from the per-component conditions
// already present in the slice. It returns Ready=True only when every per-component condition
// is True (either ComponentReady or ComponentNotRequired).
func aggregateReadyCondition(conditions []metav1.Condition, generation int64) metav1.Condition {
	var notReadyTypes, errorTypes []string

	for i := range conditions {
		c := &conditions[i]
		if isAggregateCondition(c.Type) {
			continue
		}
		if c.Status == metav1.ConditionFalse || c.Status == metav1.ConditionUnknown {
			if c.Reason == mellanoxv1alpha1.ConditionReasonComponentError {
				errorTypes = append(errorTypes, c.Type)
			} else {
				notReadyTypes = append(notReadyTypes, c.Type)
			}
		}
	}

	if len(errorTypes) > 0 || len(notReadyTypes) > 0 {
		reason := mellanoxv1alpha1.ConditionReasonComponentsNotReady
		if len(errorTypes) > 0 {
			reason = mellanoxv1alpha1.ConditionReasonComponentError
		}
		var parts []string
		if len(notReadyTypes) > 0 {
			parts = append(parts, fmt.Sprintf("deploying: %s", strings.Join(notReadyTypes, ", ")))
		}
		if len(errorTypes) > 0 {
			parts = append(parts, fmt.Sprintf("errored: %s", strings.Join(errorTypes, ", ")))
		}
		return metav1.Condition{
			Type:               mellanoxv1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            fmt.Sprintf("Components not ready: %s", strings.Join(parts, "; ")),
			ObservedGeneration: generation,
		}
	}

	return metav1.Condition{
		Type:               mellanoxv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             mellanoxv1alpha1.ConditionReasonAllComponentsReady,
		Message:            "All components are ready",
		ObservedGeneration: generation,
	}
}

// aggregateProgressingCondition derives the Progressing aggregate condition.
// It is True when at least one per-component condition is not True with reason ComponentNotReady
// (i.e. still deploying, not in error).
func aggregateProgressingCondition(conditions []metav1.Condition, generation int64) metav1.Condition {
	var deploying []string

	for i := range conditions {
		c := &conditions[i]
		if isAggregateCondition(c.Type) {
			continue
		}
		if (c.Status == metav1.ConditionFalse || c.Status == metav1.ConditionUnknown) &&
			c.Reason == mellanoxv1alpha1.ConditionReasonComponentNotReady {
			deploying = append(deploying, c.Type)
		}
	}

	if len(deploying) > 0 {
		return metav1.Condition{
			Type:               mellanoxv1alpha1.ConditionTypeProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             mellanoxv1alpha1.ConditionReasonComponentsNotReady,
			Message:            fmt.Sprintf("Components deploying: %s", strings.Join(deploying, ", ")),
			ObservedGeneration: generation,
		}
	}

	return metav1.Condition{
		Type:               mellanoxv1alpha1.ConditionTypeProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             mellanoxv1alpha1.ConditionReasonIdle,
		ObservedGeneration: generation,
	}
}

// aggregateDegradedCondition derives the Degraded aggregate condition.
// It is True when at least one per-component condition is not True with reason ComponentError.
func aggregateDegradedCondition(conditions []metav1.Condition, generation int64) metav1.Condition {
	var messages []string

	for i := range conditions {
		c := &conditions[i]
		if isAggregateCondition(c.Type) {
			continue
		}
		if (c.Status == metav1.ConditionFalse || c.Status == metav1.ConditionUnknown) &&
			c.Reason == mellanoxv1alpha1.ConditionReasonComponentError {
			if c.Message != "" {
				messages = append(messages, fmt.Sprintf("%s: %s", c.Type, c.Message))
			} else {
				messages = append(messages, c.Type)
			}
		}
	}

	if len(messages) > 0 {
		return metav1.Condition{
			Type:               mellanoxv1alpha1.ConditionTypeDegraded,
			Status:             metav1.ConditionTrue,
			Reason:             mellanoxv1alpha1.ConditionReasonComponentError,
			Message:            strings.Join(messages, "; "),
			ObservedGeneration: generation,
		}
	}

	return metav1.Condition{
		Type:               mellanoxv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             mellanoxv1alpha1.ConditionReasonHealthy,
		ObservedGeneration: generation,
	}
}

// UpdateConditions updates the NicClusterPolicy conditions slice in-place based on the
// state manager results. It sets one per-component condition for each known state result
// and then recomputes the three aggregate conditions (Ready, Progressing, Degraded).
// Unknown state names are silently skipped.
//
// apimeta.SetStatusCondition is used for every write so that LastTransitionTime is only
// bumped when the condition Status actually changes, preventing spurious status updates.
func UpdateConditions(conditions *[]metav1.Condition, results state.Results, generation int64) {
	for _, result := range results.StatesStatus {
		cond, ok := ComponentConditionFromResult(result, generation)
		if !ok {
			continue
		}
		apimeta.SetStatusCondition(conditions, cond)
	}

	// Aggregates must be computed after all per-component conditions are up-to-date.
	apimeta.SetStatusCondition(conditions, aggregateReadyCondition(*conditions, generation))
	apimeta.SetStatusCondition(conditions, aggregateProgressingCondition(*conditions, generation))
	apimeta.SetStatusCondition(conditions, aggregateDegradedCondition(*conditions, generation))
}

// SetPolicyNotSupportedConditions sets the aggregate conditions for a NicClusterPolicy
// whose instance name is not the supported singleton name.
func SetPolicyNotSupportedConditions(conditions *[]metav1.Condition, generation int64, reason string) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               mellanoxv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             mellanoxv1alpha1.ConditionReasonPolicyNotSupported,
		Message:            reason,
		ObservedGeneration: generation,
	})
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               mellanoxv1alpha1.ConditionTypeProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             mellanoxv1alpha1.ConditionReasonIdle,
		ObservedGeneration: generation,
	})
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               mellanoxv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             mellanoxv1alpha1.ConditionReasonPolicyNotSupported,
		Message:            reason,
		ObservedGeneration: generation,
	})
}

// isAggregateCondition returns true for the three top-level aggregate condition types
// so that aggregate computation functions can skip them when scanning per-component conditions.
func isAggregateCondition(condType string) bool {
	return condType == mellanoxv1alpha1.ConditionTypeReady ||
		condType == mellanoxv1alpha1.ConditionTypeDegraded ||
		condType == mellanoxv1alpha1.ConditionTypeProgressing
}
