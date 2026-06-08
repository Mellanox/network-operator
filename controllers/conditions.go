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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

// computePolicyConditions derives the conditions slice from the sync results.
// Components in SyncStateIgnore are omitted from status.conditions; their
// presence is still reflected in status.appliedStates.
//
// It merges into existing so that lastTransitionTime is preserved for
// conditions whose status has not changed (standard Kubernetes semantics,
// implemented by apimeta.SetStatusCondition).
//
// Error takes priority over NotReady for the aggregate Ready condition: if any
// component is in error state, Ready reason is ComponentError regardless of
// whether other components are also not ready.
//
// The function is used for both NicClusterPolicy and NicNodePolicy; the
// StateNameToConditionType map controls which states produce conditions for
// each CRD.
func computePolicyConditions(
	existing []metav1.Condition,
	results state.Results,
	generation int64,
) []metav1.Condition {
	conditions := make([]metav1.Condition, len(existing))
	copy(conditions, existing)

	hasError := false
	hasNotReady := false
	seenComponentConditions := make(map[string]struct{})

	for _, r := range results.StatesStatus {
		condType, ok := mellanoxv1alpha1.StateNameToConditionType[r.StateName]
		if !ok {
			continue
		}
		seenComponentConditions[condType] = struct{}{}

		cond := metav1.Condition{
			Type:               condType,
			ObservedGeneration: generation,
		}

		switch r.Status {
		case state.SyncStateReady:
			cond.Status = metav1.ConditionTrue
			cond.Reason = mellanoxv1alpha1.ConditionReasonComponentReady
			cond.Message = ""
		case state.SyncStateIgnore:
			apimeta.RemoveStatusCondition(&conditions, condType)
			continue
		case state.SyncStateError:
			cond.Status = metav1.ConditionFalse
			cond.Reason = mellanoxv1alpha1.ConditionReasonComponentError
			cond.Message = errMessage(r.ErrInfo)
			hasError = true
		default:
			// SyncStateNotReady, SyncStateReset, or any future value.
			cond.Status = metav1.ConditionFalse
			cond.Reason = mellanoxv1alpha1.ConditionReasonComponentNotReady
			cond.Message = errMessage(r.ErrInfo)
			hasNotReady = true
		}

		apimeta.SetStatusCondition(&conditions, cond)
	}

	// Prune stale component conditions for mapped states that did not appear in
	// this reconcile result snapshot.
	for _, condType := range mellanoxv1alpha1.StateNameToConditionType {
		if _, seen := seenComponentConditions[condType]; seen {
			continue
		}
		apimeta.RemoveStatusCondition(&conditions, condType)
	}

	ready := metav1.Condition{
		Type:               mellanoxv1alpha1.ConditionTypeReady,
		ObservedGeneration: generation,
	}
	switch {
	case hasError:
		ready.Status = metav1.ConditionFalse
		ready.Reason = mellanoxv1alpha1.ConditionReasonComponentError
		ready.Message = "One or more components are in error state"
	case hasNotReady:
		ready.Status = metav1.ConditionFalse
		ready.Reason = mellanoxv1alpha1.ConditionReasonComponentsNotReady
		ready.Message = "One or more components are not yet ready"
	default:
		ready.Status = metav1.ConditionTrue
		ready.Reason = mellanoxv1alpha1.ConditionReasonAllComponentsReady
		ready.Message = ""
	}
	apimeta.SetStatusCondition(&conditions, ready)

	return conditions
}

func errMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
