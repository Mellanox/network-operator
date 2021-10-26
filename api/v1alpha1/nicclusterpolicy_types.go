/*
Copyright 2021 NVIDIA

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	NicClusterPolicyCRDName = "NicClusterPolicy"
)

// Represents reconcile state of the system
type State string

// TODO: use state.SyncState, but avoid circular dependency

const (
	StateReady    = "ready"
	StateNotReady = "notReady"
	StateIgnore   = "ignore"
	StateError    = "error"
)

// ImageSpec Contains container image specifications
type ImageSpec struct {
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\-]+
	Image string `json:"image"`
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
	Repository string `json:"repository"`
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.-]+
	Version string `json:"version"`
	// +optional
	// +kubebuilder:default:={}
	ImagePullSecrets []string `json:"imagePullSecrets"`
}

type PodProbeSpec struct {
	InitialDelaySeconds int `json:"initialDelaySeconds"`
	PeriodSeconds       int `json:"periodSeconds"`
}

// OFEDDriverSpec describes configuration options for OFED driver
type OFEDDriverSpec struct {
	// Image information for ofed driver container
	ImageSpec `json:""`
	// Pod startup probe settings
	StartupProbe *PodProbeSpec `json:"startupProbe,omitempty"`
	// Pod liveness probe settings
	LivenessProbe *PodProbeSpec `json:"livenessProbe,omitempty"`
	// Pod readiness probe settings
	ReadinessProbe *PodProbeSpec `json:"readinessProbe,omitempty"`
}

// NVPeerDriverSpec describes configuration options for NV Peer Memory driver
type NVPeerDriverSpec struct {
	// Image information for nv peer memory driver container
	ImageSpec `json:""`
	// GPU driver sources path - Optional
	GPUDriverSourcePath string `json:"gpuDriverSourcePath,omitempty"`
}

// DevicePluginSpec describes configuration options for device plugin
type DevicePluginSpec struct {
	// Image information for device plugin
	ImageSpec `json:""`
	// Device plugin configuration
	Config string `json:"config"`
}

// MultusSpec describes configuration options for Multus CNI
type MultusSpec struct {
	// Image information for device plugin
	ImageSpec `json:""`
	// Multus CNI config if config is missing or empty then multus config will be automatically generated from the CNI
	// configuration file of the master plugin (the first file in lexicographical order in cni-conf-dir)
	Config string `json:"config,omitempty"`
}

// SecondaryNetwork describes configuration options for secondary network
type SecondaryNetworkSpec struct {
	// Image and configuration information for multus
	Multus *MultusSpec `json:"multus,omitempty"`
	// Image information for CNI plugins
	CniPlugins *ImageSpec `json:"cniPlugins,omitempty"`
	// Image information for IPAM plugin
	IpamPlugin *ImageSpec `json:"ipamPlugin,omitempty"`
}

// PSPSpec describes configuration for PodSecurityPolicies to apply for all Pods
type PSPSpec struct {
	// Enabled indicates if PodSecurityPolicies needs to be enabled for all Pods
	// +optional
	// +kubebuilder:default:=false
	Enabled bool `json:"enabled,omitempty"`
}

// NicClusterPolicySpec defines the desired state of NicClusterPolicy
type NicClusterPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	NodeAffinity           *v1.NodeAffinity      `json:"nodeAffinity,omitempty"`
	OFEDDriver             *OFEDDriverSpec       `json:"ofedDriver,omitempty"`
	NVPeerDriver           *NVPeerDriverSpec     `json:"nvPeerDriver,omitempty"`
	RdmaSharedDevicePlugin *DevicePluginSpec     `json:"rdmaSharedDevicePlugin,omitempty"`
	SriovDevicePlugin      *DevicePluginSpec     `json:"sriovDevicePlugin,omitempty"`
	SecondaryNetwork       *SecondaryNetworkSpec `json:"secondaryNetwork,omitempty"`
	PSP                    *PSPSpec              `json:"psp,omitempty"`
}

// AppliedState defines a finer-grained view of the observed state of NicClusterPolicy
type AppliedState struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Enum={"ready", "notReady", "ignore", "error"}
	State State `json:"state"`
}

// NicClusterPolicyStatus defines the observed state of NicClusterPolicy
type NicClusterPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Reflects the current state of the cluster policy
	// +kubebuilder:validation:Enum={"ignore", "notReady", "ready", "error"}
	State State `json:"state"`
	// Informative string in case the observed state is error
	Reason string `json:"reason,omitempty"`
	// AppliedStates provide a finer view of the observed state
	AppliedStates []AppliedState `json:"appliedStates,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`,priority=0
// +kubebuilder:printcolumn:name="Age",type=string,JSONPath=`.metadata.creationTimestamp`,priority=0

// NicClusterPolicy is the Schema for the nicclusterpolicies API
type NicClusterPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NicClusterPolicySpec   `json:"spec,omitempty"`
	Status NicClusterPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true

// NicClusterPolicyList contains a list of NicClusterPolicy
type NicClusterPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NicClusterPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NicClusterPolicy{}, &NicClusterPolicyList{})
}
