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
	// NicClusterPolicyCRDName is used for the CRD Kind.
	NicClusterPolicyCRDName = "NicClusterPolicy"
)

// State represents reconcile state of the system.
type State string

// TODO: use state.SyncState, but avoid circular dependency

const (
	// StateReady describes when reconcile has completed successfully.
	StateReady = "ready"
	// StateNotReady describes when the reconcile has not fully completed.
	StateNotReady = "notReady"
	// StateIgnore describes when the controller ignores the reconcile request.
	StateIgnore = "ignore"
	// StateError describes when the state is an error.
	StateError = "error"
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

// ImageSpecWithConfig Contains ImageSpec and optional configuration
type ImageSpecWithConfig struct {
	ImageSpec `json:""`
	Config    *string `json:"config,omitempty"`
}

// PodProbeSpec describes a pod probe.
type PodProbeSpec struct {
	InitialDelaySeconds int `json:"initialDelaySeconds"`
	PeriodSeconds       int `json:"periodSeconds"`
}

// ConfigMapNameReference references a config map in a specific namespace.
// The namespace must be specified at the point of use.
type ConfigMapNameReference struct {
	Name string `json:"name,omitempty"`
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
	// List of environment variables to set in the OFED container.
	Env []v1.EnvVar `json:"env,omitempty"`
	// Ofed auto-upgrade settings
	OfedUpgradePolicy *DriverUpgradePolicySpec `json:"upgradePolicy,omitempty"`
	// Optional: Custom TLS certificates configuration for driver container
	CertConfig *ConfigMapNameReference `json:"certConfig,omitempty"`
	// Optional: Custom package repository configuration for OFED container
	RepoConfig *ConfigMapNameReference `json:"repoConfig,omitempty"`
	// TerminationGracePeriodSeconds specifies the length of time in seconds
	// to wait before killing the OFED pod on termination
	// +optional
	// +kubebuilder:default:=300
	// +kubebuilder:validation:Minimum:=0
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// DriverUpgradePolicySpec describes policy configuration for automatic upgrades
type DriverUpgradePolicySpec struct {
	// AutoUpgrade is a global switch for automatic upgrade feature
	// if set to false all other options are ignored
	// +optional
	// +kubebuilder:default:=false
	AutoUpgrade bool `json:"autoUpgrade,omitempty"`
	// MaxParallelUpgrades indicates how many nodes can be upgraded in parallel
	// 0 means no limit, all nodes will be upgraded in parallel
	// +optional
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=0
	MaxParallelUpgrades int                    `json:"maxParallelUpgrades,omitempty"`
	WaitForCompletion   *WaitForCompletionSpec `json:"waitForCompletion,omitempty"`
	DrainSpec           *DrainSpec             `json:"drain,omitempty"`
	// SafeLoad turn on safe driver loading (cordon and drain the node before loading the driver)
	// +optional
	// +kubebuilder:default:=false
	SafeLoad bool `json:"safeLoad,omitempty"`
}

// WaitForCompletionSpec describes the configuration for waiting on job completions
type WaitForCompletionSpec struct {
	// PodSelector specifies a label selector for the pods to wait for completion
	// For more details on label selectors, see:
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	PodSelector string `json:"podSelector,omitempty"`
	// TimeoutSecond specifies the length of time in seconds
	// to wait before giving up on pod termination, zero means infinite
	// +optional
	// +kubebuilder:default:=0
	// +kubebuilder:validation:Minimum:=0
	TimeoutSecond int `json:"timeoutSeconds,omitempty"`
}

// DrainSpec describes configuration for node drain during automatic upgrade
type DrainSpec struct {
	// Enable indicates if node draining is allowed during upgrade
	// +optional
	// +kubebuilder:default:=true
	Enable bool `json:"enable,omitempty"`
	// Force indicates if force draining is allowed
	// +optional
	// +kubebuilder:default:=false
	Force bool `json:"force,omitempty"`
	// PodSelector specifies a label selector to filter pods on the node that need to be drained
	// For more details on label selectors, see:
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	PodSelector string `json:"podSelector,omitempty"`
	// TimeoutSecond specifies the length of time in seconds to wait before giving up drain, zero means infinite
	// +optional
	// +kubebuilder:default:=300
	// +kubebuilder:validation:Minimum:=0
	TimeoutSecond int `json:"timeoutSeconds,omitempty"`
	// DeleteEmptyDir indicates if should continue even if there are pods using emptyDir
	// (local data that will be deleted when the node is drained)
	// +optional
	// +kubebuilder:default:=false
	DeleteEmptyDir bool `json:"deleteEmptyDir,omitempty"`
}

// DevicePluginSpec describes configuration options for device plugin
// 1. Image information for device plugin
// 2. Device plugin configuration
type DevicePluginSpec struct {
	ImageSpecWithConfig `json:""`
	UseCdi              bool `json:"useCdi,omitempty"`
}

// MultusSpec describes configuration options for Multus CNI
//  1. Image information for Multus CNI
//  2. Multus CNI config if config is missing or empty then multus config will be automatically generated from the CNI
//     configuration file of the master plugin (the first file in lexicographical order in cni-conf-dir)
type MultusSpec struct {
	ImageSpecWithConfig `json:""`
}

// SecondaryNetworkSpec describes configuration options for secondary network
type SecondaryNetworkSpec struct {
	// Image and configuration information for multus
	Multus *MultusSpec `json:"multus,omitempty"`
	// Image information for CNI plugins
	CniPlugins *ImageSpec `json:"cniPlugins,omitempty"`
	// Image information for IPoIB CNI
	IPoIB *ImageSpec `json:"ipoib,omitempty"`
	// Image information for IPAM plugin
	IpamPlugin *ImageSpec `json:"ipamPlugin,omitempty"`
}

// IBKubernetesSpec describes configuration options for ib-kubernetes
type IBKubernetesSpec struct {
	// Image information for ib-kubernetes
	ImageSpec `json:""`
	// Interval of updates in seconds
	// +optional
	// +kubebuilder:default:=5
	// +kubebuilder:validation:Minimum:=0
	PeriodicUpdateSeconds int `json:"periodicUpdateSeconds,omitempty"`
	// The first guid in the pool
	PKeyGUIDPoolRangeStart string `json:"pKeyGUIDPoolRangeStart,omitempty"`
	// The last guid in the pool
	PKeyGUIDPoolRangeEnd string `json:"pKeyGUIDPoolRangeEnd,omitempty"`
	// Secret containing credentials to UFM service
	UfmSecret string `json:"ufmSecret,omitempty"`
}

// NVIPAMSpec describes configuration options for nv-ipam
// 1. Image information for nv-ipam
// 2. Configuration for nv-ipam
type NVIPAMSpec struct {
	// Enable deployment of the validation webhook
	// +kubebuilder:default:=false
	EnableWebhook bool `json:"enableWebhook,omitempty"`
	ImageSpec     `json:""`
}

// NICFeatureDiscoverySpec describes configuration options for nic-feature-discovery
type NICFeatureDiscoverySpec struct {
	ImageSpec `json:""`
}

// NicClusterPolicySpec defines the desired state of NicClusterPolicy
type NicClusterPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	NodeAffinity           *v1.NodeAffinity         `json:"nodeAffinity,omitempty"`
	Tolerations            []v1.Toleration          `json:"tolerations,omitempty"`
	OFEDDriver             *OFEDDriverSpec          `json:"ofedDriver,omitempty"`
	RdmaSharedDevicePlugin *DevicePluginSpec        `json:"rdmaSharedDevicePlugin,omitempty"`
	SriovDevicePlugin      *DevicePluginSpec        `json:"sriovDevicePlugin,omitempty"`
	IBKubernetes           *IBKubernetesSpec        `json:"ibKubernetes,omitempty"`
	SecondaryNetwork       *SecondaryNetworkSpec    `json:"secondaryNetwork,omitempty"`
	NvIpam                 *NVIPAMSpec              `json:"nvIpam,omitempty"`
	NicFeatureDiscovery    *NICFeatureDiscoverySpec `json:"nicFeatureDiscovery,omitempty"`
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
