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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkAttachmentDefinitionSpec defines the desired state of NetworkAttachmentDefinition
type NetworkAttachmentDefinitionSpec struct {
	Config string `json:"config"`
}

// NetworkAttachmentDefinitionStatus defines the observed state of NetworkAttachmentDefinition
type NetworkAttachmentDefinitionStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkAttachmentDefinition is the Schema for the networkattachmentdefinitions API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=networkattachmentdefinitions,scope=Cluster
type NetworkAttachmentDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkAttachmentDefinitionSpec   `json:"spec,omitempty"`
	Status NetworkAttachmentDefinitionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkAttachmentDefinitionList contains a list of NetworkAttachmentDefinition
type NetworkAttachmentDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkAttachmentDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkAttachmentDefinition{}, &NetworkAttachmentDefinitionList{})
}
