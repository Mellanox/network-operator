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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/Mellanox/network-operator/pkg/nodeinfo"
)

type MlnxLabelChangedPredicate struct {
	predicate.Funcs
}

func (p MlnxLabelChangedPredicate) hasMlnxLabel(labels map[string]string) bool {
	// We don't need to check label value because NFD doesn't set it to "false"
	_, exist := labels[nodeinfo.NodeLabelMlnxNIC]
	return exist
}

func (p MlnxLabelChangedPredicate) Update(e event.UpdateEvent) bool {
	return p.hasMlnxLabel(e.ObjectOld.GetLabels()) != p.hasMlnxLabel(e.ObjectNew.GetLabels())
}
