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
	// Name of the image
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\-]+
	Image string `json:"image"`
	// Address of the registry that stores the image
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
	Repository string `json:"repository"`
	// Version of the image to use
	Version string `json:"version"`
	// ImagePullSecrets is an optional list of references to secrets in the same
	// namespace to use for pulling the image
	// +optional
	// +kubebuilder:default:={}
	ImagePullSecrets []string `json:"imagePullSecrets"`
	// ResourceRequirements describes the compute resource requirements
	ContainerResources []ResourceRequirements `json:"containerResources,omitempty"`
}

// GetContainerResources is a method to easily get container resources from struct, that embed ImageSpec
func (is *ImageSpec) GetContainerResources() []ResourceRequirements {
	if is == nil {
		return nil
	}
	return is.ContainerResources
}

// ImageSpecWithConfig Contains ImageSpec and optional configuration
type ImageSpecWithConfig struct {
	// Image information for the component
	ImageSpec `json:""`
	// Configuration for the component as a string
	Config *string `json:"config,omitempty"`
}

// PodProbeSpec describes a pod probe.
type PodProbeSpec struct {
	// Number of seconds after the container has started before the probe is initiated
	InitialDelaySeconds int `json:"initialDelaySeconds"`
	// How often (in seconds) to perform the probe
	PeriodSeconds int `json:"periodSeconds"`
}

// ConfigMapNameReference references a config map in a specific namespace.
// The namespace must be specified at the point of use.
type ConfigMapNameReference struct {
	// Name of the ConfigMap
	Name string `json:"name,omitempty"`
}

// OFEDDriverSpec describes configuration options for DOCA Driver Container
type OFEDDriverSpec struct {
	// Image information for DOCA driver container
	ImageSpec `json:""`
	// Pod startup probe settings
	StartupProbe *PodProbeSpec `json:"startupProbe,omitempty"`
	// Pod liveness probe settings
	LivenessProbe *PodProbeSpec `json:"livenessProbe,omitempty"`
	// Pod readiness probe settings
	ReadinessProbe *PodProbeSpec `json:"readinessProbe,omitempty"`
	// List of environment variables to set in the DOCA driver container.
	Env []v1.EnvVar `json:"env,omitempty"`
	// DOCA driver auto-upgrade settings
	OfedUpgradePolicy *DriverUpgradePolicySpec `json:"upgradePolicy,omitempty"`
	// Optional: Custom TLS certificates configuration for DOCA driver container
	CertConfig *ConfigMapNameReference `json:"certConfig,omitempty"`
	// Optional: Custom package repository configuration for DOCA driver container
	RepoConfig *ConfigMapNameReference `json:"repoConfig,omitempty"`
	// TerminationGracePeriodSeconds specifies the length of time in seconds
	// to wait before killing the DOCA driver container pod on termination
	// +optional
	// +kubebuilder:default:=300
	// +kubebuilder:validation:Minimum:=0
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// ForcePrecompiled specifies if only DOCA driver precompiled images are allowed
	// If set to false and precompiled image does not exists, DOCA driver will be compiled on Nodes
	// If set to true and precompiled image does not exists, OFED state will be Error.
	// +optional
	// +kubebuilder:default:=false
	ForcePrecompiled bool `json:"forcePrecompiled,omitempty"`
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
	MaxParallelUpgrades int `json:"maxParallelUpgrades,omitempty"`
	// The configuration for waiting on pods completions
	WaitForCompletion *WaitForCompletionSpec `json:"waitForCompletion,omitempty"`
	// The configuration for node drain during automatic upgrade
	DrainSpec *DrainSpec `json:"drain,omitempty"`
	// SafeLoad turn on safe driver loading (cordon and drain the node before loading the driver)
	// +optional
	// +kubebuilder:default:=false
	SafeLoad bool `json:"safeLoad,omitempty"`
}

// WaitForCompletionSpec describes the configuration for waiting on pods completions
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
	// Image information for the device plugin and optional configuration
	ImageSpecWithConfig `json:""`
	// Enables use of container device interface (CDI)
	// NOTE: NVIDIA Network Operator does not configure container runtime to enable CDI.
	UseCdi bool `json:"useCdi,omitempty"`
}

// MultusSpec describes configuration options for Multus CNI
//  1. Image information for Multus CNI
//  2. Multus CNI config if config is missing or empty then multus config will be automatically generated from the CNI
//     configuration file of the master plugin (the first file in lexicographical order in cni-conf-dir)
type MultusSpec struct {
	// Image information for Multus and optional configuration
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

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	// Name of the container the requirements are set for
	Name string `json:"name"`
	// Limits describes the maximum amount of compute resources allowed.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Limits v1.ResourceList `json:"limits,omitempty"`
	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified,
	// otherwise to an implementation-defined value. Requests cannot exceed Limits.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Requests v1.ResourceList `json:"requests,omitempty"`
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
	PKeyGUIDPoolRangeStart string `json:"pKeyGUIDPoolRangeStart"`
	// The last guid in the pool
	PKeyGUIDPoolRangeEnd string `json:"pKeyGUIDPoolRangeEnd"`
	// Secret containing credentials to UFM service
	UfmSecret string `json:"ufmSecret"`
}

// NVIPAMSpec describes configuration options for nv-ipam
// 1. Image information for nv-ipam
// 2. Configuration for nv-ipam
type NVIPAMSpec struct {
	// Enable deployment of the validation webhook
	EnableWebhook bool `json:"enableWebhook,omitempty"`
	// Image information for nv-ipam
	ImageSpec `json:""`
}

// NICFeatureDiscoverySpec describes configuration options for nic-feature-discovery
type NICFeatureDiscoverySpec struct {
	// Image information for nic-feature-discovery
	ImageSpec `json:""`
}

// DOCATelemetryServiceConfig contains configuration for the DOCATelemetryService.
type DOCATelemetryServiceConfig struct {
	// FromConfigMap sets the configMap the DOCATelemetryService gets its configuration from. The ConfigMap must be in
	// the same namespace as the NICClusterPolicy.
	// +optional
	FromConfigMap string `json:"fromConfigMap"`
}

// DOCATelemetryServiceSpec is the configuration for DOCA Telemetry Service.
type DOCATelemetryServiceSpec struct {
	// Image information for DOCA Telemetry Service
	ImageSpec `json:""`
	// +optional
	// Config contains custom config for the DOCATelemetryService.
	// If set no default config will be deployed.
	Config *DOCATelemetryServiceConfig `json:"config"`
}

// NicClusterPolicySpec defines the desired state of NicClusterPolicy
type NicClusterPolicySpec struct {
	// OFEDDriver is a specialized driver for NVIDIA NICs which can replace the inbox driver that comes with an OS.
	// See https://network.nvidia.com/support/mlnx-ofed-matrix/
	OFEDDriver *OFEDDriverSpec `json:"ofedDriver,omitempty"`
	// RdmaSharedDevicePlugin manages support IB and RoCE HCAs through the Kubernetes device plugin framework.
	// The config field is a json representation of the RDMA shared device plugin configuration.
	// See https://github.com/Mellanox/k8s-rdma-shared-dev-plugin
	RdmaSharedDevicePlugin *DevicePluginSpec `json:"rdmaSharedDevicePlugin,omitempty"`
	// SriovDevicePlugin manages SRIOV through the Kubernetes device plugin framework.
	// The config field is a json representation of the RDMA shared device plugin configuration.
	// See https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin
	SriovDevicePlugin *DevicePluginSpec `json:"sriovDevicePlugin,omitempty"`
	// IBKubernetes provides a daemon that works in conjunction with the SR-IOV Network Device Plugin.
	// It acts on Kubernetes pod object changes and reads the pod's network annotation.
	// From there it fetches the corresponding network CRD and reads the PKey.
	// This is done in order to add the newly generated GUID or the predefined GUID in the GUID field of the CRD.
	// This is then passed in cni-args to that PKey for pods with mellanox.infiniband.app annotation.
	// See: https://github.com/Mellanox/ib-kubernetes
	IBKubernetes *IBKubernetesSpec `json:"ibKubernetes,omitempty"`
	// SecondaryNetwork Specifies components to deploy in order to facilitate a secondary network in Kubernetes.
	// It consists of the following optionally deployed components:
	// - Multus-CNI: Delegate CNI plugin to support secondary networks in Kubernetes
	// - CNI plugins: Currently only containernetworking-plugins is supported
	// - IPAM CNI: Currently only Whereabout IPAM CNI is supported as a part of the secondaryNetwork section.
	// - IPoIB CNI: Allows the user to create IPoIB child link and move it to the pod
	SecondaryNetwork *SecondaryNetworkSpec `json:"secondaryNetwork,omitempty"`
	// NvIpam is an IPAM provider that dynamically assigns IP addresses with speed and performance in mind.
	// Note: NvIPam requires certificate management e.g. cert-manager or OpenShift cert management.
	// See https://github.com/Mellanox/nvidia-k8s-ipam
	NvIpam *NVIPAMSpec `json:"nvIpam,omitempty"`
	// NicFeatureDiscovery works with NodeFeatureDiscovery to expose information about NVIDIA NICs.
	// https://github.com/Mellanox/nic-feature-discovery
	NicFeatureDiscovery *NICFeatureDiscoverySpec `json:"nicFeatureDiscovery,omitempty"`
	// DOCATelemetryService exposes telemetry from NVIDIA networking components to prometheus.
	// See: https://docs.nvidia.com/doca/sdk/nvidia+doca+telemetry+service+guide/index.html
	DOCATelemetryService *DOCATelemetryServiceSpec `json:"docaTelemetryService,omitempty"`
	// NodeAffinity rules to inject to the DaemonSets objects that are managed by the operator
	NodeAffinity *v1.NodeAffinity `json:"nodeAffinity,omitempty"`
	// Tolerations to inject to the DaemonSets objects that are managed by the operator
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// Configuration options for OFED driver

}

// AppliedState defines a finer-grained view of the observed state of NicClusterPolicy
type AppliedState struct {
	// Name of the deployed component this state refers to
	Name string `json:"name"`
	// The state of the deployed component. ("ready", "notReady", "ignore", "error")
	// +kubebuilder:validation:Enum={"ready", "notReady", "ignore", "error"}
	State State `json:"state"`
	// Message is a human readable message indicating details about why
	// the state is in this condition
	Message string `json:"message,omitempty"`
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
// +kubebuilder:resource:scope=Cluster,shortName="ncp"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`,priority=0
// +kubebuilder:printcolumn:name="Age",type=string,JSONPath=`.metadata.creationTimestamp`,priority=0

// NicClusterPolicy is the Schema for the nicclusterpolicies API
type NicClusterPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of NicClusterPolicy
	Spec NicClusterPolicySpec `json:"spec,omitempty"`
	// Defines the observed state of NicClusterPolicy
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
