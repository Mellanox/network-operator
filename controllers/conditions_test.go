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
	"errors"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

const testObservedGeneration int64 = 3

func TestComputePolicyConditionsIgnoredComponentsOmitted(t *testing.T) {
	t.Parallel()

	results := state.Results{
		StatesStatus: []state.Result{
			{StateName: "state-OFED", Status: state.SyncStateIgnore},
			{StateName: "state-multus-cni", Status: state.SyncStateIgnore},
		},
	}

	conditions := computePolicyConditions(nil, results, testObservedGeneration)

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d: %+v", len(conditions), conditions)
	}
	ready := conditions[0]
	if ready.Type != mellanoxv1alpha1.ConditionTypeReady {
		t.Fatalf("expected Ready condition, got %s", ready.Type)
	}
	if ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready=True, got %s", ready.Status)
	}
	if ready.Reason != mellanoxv1alpha1.ConditionReasonAllComponentsReady {
		t.Fatalf("expected reason AllComponentsReady, got %s", ready.Reason)
	}
}

func TestComputePolicyConditionsConfiguredReadyComponentGetsCondition(t *testing.T) {
	t.Parallel()

	results := state.Results{
		StatesStatus: []state.Result{
			{StateName: "state-OFED", Status: state.SyncStateReady},
			{StateName: "state-multus-cni", Status: state.SyncStateIgnore},
		},
	}

	conditions := computePolicyConditions(nil, results, testObservedGeneration)

	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d: %+v", len(conditions), conditions)
	}

	ofed := apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeOFEDDriverReady)
	if ofed == nil {
		t.Fatal("expected OFEDDriverReady condition")
	}
	if ofed.Status != metav1.ConditionTrue || ofed.Reason != mellanoxv1alpha1.ConditionReasonComponentReady {
		t.Fatalf("unexpected OFED condition: %+v", *ofed)
	}
	if apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeMultusCNIReady) != nil {
		t.Fatal("expected MultusCNIReady condition to be omitted")
	}
}

func TestComputePolicyConditionsIgnoredComponentRemovesStaleCondition(t *testing.T) {
	t.Parallel()

	existing := []metav1.Condition{
		{
			Type:               mellanoxv1alpha1.ConditionTypeOFEDDriverReady,
			Status:             metav1.ConditionTrue,
			Reason:             mellanoxv1alpha1.ConditionReasonComponentReady,
			ObservedGeneration: 1,
		},
	}
	results := state.Results{
		StatesStatus: []state.Result{
			{StateName: "state-OFED", Status: state.SyncStateIgnore},
		},
	}

	conditions := computePolicyConditions(existing, results, testObservedGeneration)

	if apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeOFEDDriverReady) != nil {
		t.Fatal("expected stale OFEDDriverReady condition to be removed")
	}
}

func TestComputePolicyConditionsMissingStateRemovesStaleCondition(t *testing.T) {
	t.Parallel()

	existing := []metav1.Condition{
		{
			Type:               mellanoxv1alpha1.ConditionTypeOFEDDriverReady,
			Status:             metav1.ConditionTrue,
			Reason:             mellanoxv1alpha1.ConditionReasonComponentReady,
			ObservedGeneration: 1,
		},
		{
			Type:               mellanoxv1alpha1.ConditionTypeMultusCNIReady,
			Status:             metav1.ConditionTrue,
			Reason:             mellanoxv1alpha1.ConditionReasonComponentReady,
			ObservedGeneration: 1,
		},
	}
	results := state.Results{
		StatesStatus: []state.Result{
			{StateName: "state-OFED", Status: state.SyncStateReady},
		},
	}

	conditions := computePolicyConditions(existing, results, testObservedGeneration)

	if apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeMultusCNIReady) != nil {
		t.Fatal("expected stale MultusCNIReady condition to be removed when state is missing")
	}

	ofed := apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeOFEDDriverReady)
	if ofed == nil {
		t.Fatal("expected OFEDDriverReady condition")
	}
	if ofed.Status != metav1.ConditionTrue || ofed.Reason != mellanoxv1alpha1.ConditionReasonComponentReady {
		t.Fatalf("unexpected OFED condition: %+v", *ofed)
	}
}

func TestComputePolicyConditionsNotReadyComponentBlocksAggregateReady(t *testing.T) {
	t.Parallel()

	results := state.Results{
		StatesStatus: []state.Result{
			{StateName: "state-OFED", Status: state.SyncStateNotReady},
			{StateName: "state-multus-cni", Status: state.SyncStateIgnore},
		},
	}

	conditions := computePolicyConditions(nil, results, testObservedGeneration)

	ofed := apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeOFEDDriverReady)
	if ofed == nil {
		t.Fatal("expected OFEDDriverReady condition")
	}
	if ofed.Status != metav1.ConditionFalse || ofed.Reason != mellanoxv1alpha1.ConditionReasonComponentNotReady {
		t.Fatalf("unexpected OFED condition: %+v", *ofed)
	}

	ready := apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeReady)
	if ready == nil {
		t.Fatal("expected Ready condition")
	}
	if ready.Status != metav1.ConditionFalse || ready.Reason != mellanoxv1alpha1.ConditionReasonComponentsNotReady {
		t.Fatalf("unexpected Ready condition: %+v", *ready)
	}
}

func TestComputePolicyConditionsErrorTakesPriorityForAggregateReady(t *testing.T) {
	t.Parallel()

	syncErr := errors.New("sync failed")
	results := state.Results{
		StatesStatus: []state.Result{
			{StateName: "state-OFED", Status: state.SyncStateError, ErrInfo: syncErr},
			{StateName: "state-RDMA-device-plugin", Status: state.SyncStateNotReady},
		},
	}

	conditions := computePolicyConditions(nil, results, testObservedGeneration)

	ready := apimeta.FindStatusCondition(conditions, mellanoxv1alpha1.ConditionTypeReady)
	if ready == nil {
		t.Fatal("expected Ready condition")
	}
	if ready.Status != metav1.ConditionFalse || ready.Reason != mellanoxv1alpha1.ConditionReasonComponentError {
		t.Fatalf("unexpected Ready condition: %+v", *ready)
	}
}
