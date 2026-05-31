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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Per-component condition type constants.
// Each component present in the spec exposes exactly one condition of the
// form <ComponentName>Ready. Components absent from the spec are omitted.
const (
	ConditionTypeOFEDDriverReady               = "OFEDDriverReady"
	ConditionTypeRDMASharedDevicePluginReady   = "RDMASharedDevicePluginReady"
	ConditionTypeSRIOVDevicePluginReady        = "SRIOVDevicePluginReady"
	ConditionTypeIBKubernetesReady             = "IBKubernetesReady"
	ConditionTypeMultusCNIReady                = "MultusCNIReady"
	ConditionTypeCNIPluginsReady               = "CNIPluginsReady"
	ConditionTypeIPoIBCNIReady                 = "IPoIBCNIReady"
	ConditionTypeNVIPAMReady                   = "NVIPAMReady"
	ConditionTypeNICFeatureDiscoveryReady      = "NICFeatureDiscoveryReady"
	ConditionTypeDOCATelemetryServiceReady     = "DOCATelemetryServiceReady"
	ConditionTypeNICConfigurationOperatorReady = "NICConfigurationOperatorReady"
	ConditionTypeSpectrumXOperatorReady        = "SpectrumXOperatorReady"

	// ConditionTypeReady is the aggregate condition that is True only when every
	// configured component is ready.
	ConditionTypeReady = "Ready"
)

// Reason constants for per-component <ComponentName>Ready conditions.
const (
	// ConditionReasonComponentReady indicates the component's workloads are fully available.
	ConditionReasonComponentReady = "ComponentReady"
	// ConditionReasonComponentNotReady indicates the component is deploying and expected
	// to become ready without operator intervention.
	ConditionReasonComponentNotReady = "ComponentNotReady"
	// ConditionReasonComponentError indicates a reconcile failure that requires attention.
	// Also used as a reason for the aggregate Ready condition.
	ConditionReasonComponentError = "ComponentError"
)

// Reason constants for the aggregate Ready condition.
const (
	// ConditionReasonAllComponentsReady indicates every configured component is ready.
	ConditionReasonAllComponentsReady = "AllComponentsReady"
	// ConditionReasonComponentsNotReady indicates at least one component is still deploying.
	ConditionReasonComponentsNotReady = "ComponentsNotReady"
)

// StateNameToConditionType maps the Name() of a pkg/state.State to the
// corresponding NicClusterPolicy condition type constant.
// States absent from this map (e.g. states used only by NicNodePolicy) produce
// no condition entry.
var StateNameToConditionType = map[string]string{
	"state-OFED":                         ConditionTypeOFEDDriverReady,
	"state-RDMA-device-plugin":           ConditionTypeRDMASharedDevicePluginReady,
	"state-SRIOV-device-plugin":          ConditionTypeSRIOVDevicePluginReady,
	"state-ib-kubernetes":                ConditionTypeIBKubernetesReady,
	"state-multus-cni":                   ConditionTypeMultusCNIReady,
	"state-container-networking-plugins": ConditionTypeCNIPluginsReady,
	"state-ipoib-cni":                    ConditionTypeIPoIBCNIReady,
	"state-nv-ipam-cni":                  ConditionTypeNVIPAMReady,
	"state-nic-feature-discovery":        ConditionTypeNICFeatureDiscoveryReady,
	"state-doca-telemetry-service":       ConditionTypeDOCATelemetryServiceReady,
	"state-nic-configuration-operator":   ConditionTypeNICConfigurationOperatorReady,
	"state-spectrum-x-operator":          ConditionTypeSpectrumXOperatorReady,
}

// ConditionHolder is implemented by CRDs that carry a status.conditions array.
// NicClusterPolicy implements this interface; NicNodePolicy does not.
//
// +kubebuilder:object:generate=false
type ConditionHolder interface {
	GetConditions() []metav1.Condition
	SetConditions([]metav1.Condition)
}
