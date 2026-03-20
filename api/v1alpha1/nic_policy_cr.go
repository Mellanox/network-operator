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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NicPolicyCR is the common interface satisfied by NicClusterPolicy and NicNodePolicy.
// State sync methods use it to access the desired NIC configuration and set owner references
// without depending on a concrete CRD type.
//
// +kubebuilder:object:generate=false
type NicPolicyCR interface {
	client.Object
	GetOFEDDriverSpec() *OFEDDriverSpec
	GetRdmaSharedDevicePluginSpec() *DevicePluginSpec
	GetSriovDevicePluginSpec() *DevicePluginSpec
	GetTolerations() []v1.Toleration
	GetNodeAffinity() *v1.NodeAffinity
	GetNodeSelector() map[string]string
	GetGlobalConfig() *GlobalConfig
	GetCRDName() string
}
