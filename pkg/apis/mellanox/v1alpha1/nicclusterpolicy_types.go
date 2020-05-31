package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ImageSpec Contains container image specifications
type ImageSpec struct {
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\-]+
	Image string `json:"image"`
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
	Repository string `json:"repository"`
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.-]+
	Version string `json:"version"`
}

// OFEDDriverSpec describes configuration options for OFED driver
type OFEDDriverSpec struct {
	// Image information for ofed driver container
	ImageSpec `json:""`
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

// NicClusterPolicySpec defines the desired state of NicClusterPolicy
type NicClusterPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	OFEDDriver   *OFEDDriverSpec   `json:"ofedDriver,omitempty"`
	NVPeerDriver *NVPeerDriverSpec `json:"nvPeerDriver,omitempty"`
	DevicePlugin *DevicePluginSpec `json:"devicePluginSpec,omitempty"`
}

type State string

const (
	StateNotReady State = "notReady"
	StateReady    State = "ready"
	StateError    State = "error"
)

// AppliedState defines a finer-grained view of the observed state of NicClusterPolicy
type AppliedState struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Enum={"notReady", "ready", "error"}
	State State `json:"state"`
}

// NicClusterPolicyStatus defines the observed state of NicClusterPolicy
type NicClusterPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Reflects the current state of the cluster policy
	// +kubebuilder:validation:Enum={"notReady", "ready", "error"}
	State State `json:"state"`
	// Informative string in case the observed state is error
	Reason string `json:"reason,omitempty"`
	// AppliedStates provide a finer view of the observed state
	AppliedStates []AppliedState `json:"appliedStates,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NicClusterPolicy is the Schema for the nicclusterpolicies API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=nicclusterpolicies,scope=Namespaced
type NicClusterPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NicClusterPolicySpec   `json:"spec,omitempty"`
	Status NicClusterPolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NicClusterPolicyList contains a list of NicClusterPolicy
type NicClusterPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NicClusterPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NicClusterPolicy{}, &NicClusterPolicyList{})
}
