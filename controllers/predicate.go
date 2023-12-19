/*
Copyright 2022 NVIDIA CORPORATION & AFFILIATES

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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/Mellanox/network-operator/pkg/nodeinfo"
)

// MlnxLabelChangedPredicate filters if nodeinfo.NodeLabelMlnxNIC label has changed.
type MlnxLabelChangedPredicate struct {
	predicate.Funcs
}

func (p MlnxLabelChangedPredicate) hasMlnxLabel(labels map[string]string) bool {
	// We don't need to check label value because NFD doesn't set it to "false"
	_, exist := labels[nodeinfo.NodeLabelMlnxNIC]
	return exist
}

// Update returns true if the nodeinfo.NodeLabelMlnxNIC has been changed.
func (p MlnxLabelChangedPredicate) Update(e event.UpdateEvent) bool {
	return p.hasMlnxLabel(e.ObjectOld.GetLabels()) != p.hasMlnxLabel(e.ObjectNew.GetLabels())
}

// IgnoreSameContentPredicate filters updates if old and new object are the same,
// ignores ResourceVersion and ManagedFields while comparing
type IgnoreSameContentPredicate struct {
	predicate.Funcs
}

// Update returns true if the Update event should be processed.
func (p IgnoreSameContentPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}
	oldObj := e.ObjectOld.DeepCopyObject().(client.Object)
	newObj := e.ObjectNew.DeepCopyObject().(client.Object)
	// ignore resource version
	oldObj.SetResourceVersion("")
	newObj.SetResourceVersion("")
	// ignore generation
	oldObj.SetGeneration(0)
	newObj.SetGeneration(0)
	// ignore managed fields
	oldObj.SetManagedFields([]metav1.ManagedFieldsEntry{})
	newObj.SetManagedFields([]metav1.ManagedFieldsEntry{})

	// logic specific to resource type
	switch v := oldObj.(type) {
	case *appsv1.Deployment:
		p.handleDeployment(v, newObj.(*appsv1.Deployment))
	default:
	}

	oldUnstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(oldObj)
	if err != nil {
		return false
	}
	newUnstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newObj)
	if err != nil {
		return false
	}
	return !reflect.DeepEqual(oldUnstr, newUnstr)
}

func (p IgnoreSameContentPredicate) handleDeployment(oldObj, newObj *appsv1.Deployment) {
	// ignore object if only Observed generation changed
	oldObj.Status.ObservedGeneration = 0
	newObj.Status.ObservedGeneration = 0

	// ignore annotation which is auto set by Kubernetes
	revAnnotation := "deployment.kubernetes.io/revision"
	delete(oldObj.Annotations, revAnnotation)
	delete(newObj.Annotations, revAnnotation)
}
