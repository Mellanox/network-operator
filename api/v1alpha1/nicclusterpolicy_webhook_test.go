/*
2023 NVIDIA CORPORATION & AFFILIATES

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
package v1alpha1 //nolint:dupl

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:dupl
var _ = Describe("Validate", func() {
	Context("NicClusterPolicy tests", func() {
		It("Valid GUID range", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					IBKubernetes: &IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00:00:00:00:00",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00:00:00:00:01",
						ImageSpec: ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err).NotTo(HaveOccurred())
		})
		It("Invalid GUID range", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					IBKubernetes: &IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00:00:00:00:02",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00:00:00:00:00",
						ImageSpec: ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"pKeyGUIDPoolRangeStart-pKeyGUIDPoolRangeEnd must be a valid range"))
		})
		It("Invalid start and end GUID", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					IBKubernetes: &IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00",
						ImageSpec: ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(And(
				ContainSubstring("pKeyGUIDPoolRangeStart must be a valid GUID format"),
				ContainSubstring("pKeyGUIDPoolRangeEnd must be a valid GUID format")))
		})
		It("Valid MOFED version", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					OFEDDriver: &OFEDDriverSpec{
						ImageSpec: ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err).NotTo(HaveOccurred())
		})
		It("InValid MOFED version", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					OFEDDriver: &OFEDDriverSpec{
						ImageSpec: ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23-10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("invalid OFED version"))
		})
		It("MOFED SafeLoad requires AutoUpgrade to be enabled", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					OFEDDriver: &OFEDDriverSpec{
						ImageSpec: ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
						OfedUpgradePolicy: &DriverUpgradePolicySpec{
							SafeLoad: true,
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("autoUpgrade"))
		})
		It("MOFED valid SafeLoad config", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					OFEDDriver: &OFEDDriverSpec{
						ImageSpec: ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
						OfedUpgradePolicy: &DriverUpgradePolicySpec{
							SafeLoad:    true,
							AutoUpgrade: true,
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err).To(BeNil())
		})
		It("Valid RDMA config JSON", func() {
			rdmaConfig := `{
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(rdmaConfig)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err).NotTo(HaveOccurred())
		})
		It("Valid RDMA config JSON", func() {
			rdmaConfig := `{
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(rdmaConfig)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err).NotTo(HaveOccurred())
		})
		It("Invalid RDMA config JSON, missing starting {", func() {
			invalidRdmaConfigJSON := `
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(invalidRdmaConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid json of RdmaSharedDevicePluginConfig"))
		})
		It("Invalid RDMA config JSON schema, resourceName not valid", func() {
			invalidRdmaConfigJSON := `{
				"configList": [{
					"resourceName": "rdma-shared-device-a!!",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(invalidRdmaConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("Invalid Resource name"))
		})
		It("Invalid RDMA config JSON schema, no configList provided", func() {
			invalidRdmaConfigJSON := `{
				"configList_a": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(invalidRdmaConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("configList is required"))
		})
		It("Invalid RDMA config JSON schema, none of the selectors are provided", func() {
			invalidRdmaConfigJSON := `{
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(invalidRdmaConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("vendors is required"))
		})
		It("Invalid RDMA config JSON, vendors must be list of strings", func() {
			invalidRdmaConfigJSON := `{
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": [15],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(invalidRdmaConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid type. Expected: string, given: integer"))
		})
		It("Invalid RDMA config JSON, deviceIDs must be list of strings", func() {
			invalidRdmaConfigJSON := `{
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": [1010]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(invalidRdmaConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid type. Expected: string, given: integer"))
		})
		It("Invalid RDMA config JSON resourcePrefix is not FQDN", func() {
			invalidRdmaConfigJSON := `{
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"resourcePrefix": "nvidia.com#$&&@",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(invalidRdmaConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid Resource prefix, it must be a valid FQDN"))
		})
		It("Valid SriovDevicePlugin config JSON", func() {
			sriovConfig := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(sriovConfig)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err).NotTo(HaveOccurred())
		})
		It("Valid SriovDevicePlugin config JSON, selectors object is a list ", func() {
			sriovConfig := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": [{
						"vendors": ["15b3"],
						"devices": ["101b"]}]}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(sriovConfig)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err).NotTo(HaveOccurred())
		})
		It("Invalid SriovDevicePlugin config JSON, missing starting {", func() {
			invalidSriovConfigJSON := `
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(invalidSriovConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid json of SriovNetworkDevicePluginConfig"))
		})
		It("Invalid SriovDevicePlugin config JSON schema, resourceName not valid", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "sriov-network-device-plugin",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(invalidSriovConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("Invalid Resource name"))
		})
		It("Invalid SriovDevicePlugin config JSON schema, no resourceList provided", func() {
			invalidSriovConfigJSON := `{
				"resourceList_a": [{
					"resourceName": "sriov_network_device_plugin",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(invalidSriovConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("resourceList is required"))
		})
		It("Invalid SriovDevicePlugin config JSON schema, none of the selectors are provided", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "sriov_network_device_plugin",
					"selectors": {}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(invalidSriovConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring("vendors is required"))
		})
		It("Invalid SriovDevicePlugin config JSON, vendors must be list of strings", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": [15],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(invalidSriovConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid type. Expected: string, given: integer"))
		})
		It("Invalid SriovDevicePlugin config JSON, devices must be list of strings", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": ["15b3"],
						"devices": [1020]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(invalidSriovConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid type. Expected: string, given: integer"))
		})
		It("Invalid SriovDevicePlugin resourcePrefix is not FQDN", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"resourcePrefix": "nvidia.com#$&&@",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(invalidSriovConfigJSON)
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"Invalid Resource prefix, it must be a valid FQDN"))
		})
	})
	Context("Image repository tests", func() {
		It("Invalid Repository IBKubernetes", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					IBKubernetes: &IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00:00:00:00:00",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00:00:00:00:02",
						ImageSpec: ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository OFEDDriver", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					OFEDDriver: &OFEDDriverSpec{
						ImageSpec: ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository RdmaSharedDevicePlugin", func() {
			rdmaConfig := `{
			"configList": [{
				"resourceName": "rdma_shared_device_a",
				"rdmaHcaMax": 63,
				"selectors": {
					"vendors": ["15b3"],
					"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					RdmaSharedDevicePlugin: &DevicePluginSpec{
						ImageSpecWithConfig: ImageSpecWithConfig{
							Config: &rdmaConfig,
							ImageSpec: ImageSpec{
								Image:            "k8s-rdma-shared-dev-plugin",
								Repository:       "ghcr.io/mellanox!@!#$!",
								Version:          "sha-fe7f371c7e1b8315bf900f71cd25cfc1251dc775",
								ImagePullSecrets: []string{},
							},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SriovDevicePlugin", func() {
			sriovConfig := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					SriovDevicePlugin: &DevicePluginSpec{
						ImageSpecWithConfig: ImageSpecWithConfig{
							Config: &sriovConfig,
							ImageSpec: ImageSpec{
								Image:            "sriov-network-device-plugin",
								Repository:       "nvcr.io/nvstaging/mellanox!@!#$!",
								Version:          "network-operator-23.10.0-beta.1",
								ImagePullSecrets: []string{},
							},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository NVIPAM", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					NvIpam: &NVIPAMSpec{
						ImageSpecWithConfig: ImageSpecWithConfig{
							ImageSpec: ImageSpec{
								Image:            "mofed",
								Repository:       "ghcr.io/mellanox!@!#$!",
								Version:          "23.10-0.2.2.0",
								ImagePullSecrets: []string{},
							},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository NicFeatureDiscovery", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					NicFeatureDiscovery: &NICFeatureDiscoverySpec{
						ImageSpec: ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork Multus", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					SecondaryNetwork: &SecondaryNetworkSpec{
						Multus: &MultusSpec{
							ImageSpecWithConfig: ImageSpecWithConfig{
								ImageSpec: ImageSpec{
									Image:            "mofed",
									Repository:       "ghcr.io/mellanox!@!#$!",
									Version:          "23.10-0.2.2.0",
									ImagePullSecrets: []string{},
								},
							},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork Multus", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					SecondaryNetwork: &SecondaryNetworkSpec{
						Multus: &MultusSpec{
							ImageSpecWithConfig: ImageSpecWithConfig{
								ImageSpec: ImageSpec{
									Image:            "mofed",
									Repository:       "ghcr.io/mellanox!@!#$!",
									Version:          "23.10-0.2.2.0",
									ImagePullSecrets: []string{},
								},
							},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork CniPlugins", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					SecondaryNetwork: &SecondaryNetworkSpec{
						CniPlugins: &ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork IPoIB", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					SecondaryNetwork: &SecondaryNetworkSpec{
						IPoIB: &ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork IpamPlugin", func() {
			nicClusterPolicy := NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: NicClusterPolicySpec{
					SecondaryNetwork: &SecondaryNetworkSpec{
						IpamPlugin: &ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := nicClusterPolicy.ValidateCreate()
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
	})
})

func rdmaDPNicClusterPolicy(config string) NicClusterPolicy {
	return NicClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: NicClusterPolicySpec{
			RdmaSharedDevicePlugin: &DevicePluginSpec{
				ImageSpecWithConfig: ImageSpecWithConfig{
					Config: &config,
					ImageSpec: ImageSpec{
						Image:            "k8s-rdma-shared-dev-plugin",
						Repository:       "ghcr.io/mellanox",
						Version:          "sha-fe7f371c7e1b8315bf900f71cd25cfc1251dc775",
						ImagePullSecrets: []string{},
					},
				},
			},
		},
	}
}

func sriovDPNicClusterPolicy(config string) NicClusterPolicy {
	return NicClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: NicClusterPolicySpec{
			SriovDevicePlugin: &DevicePluginSpec{
				ImageSpecWithConfig: ImageSpecWithConfig{
					Config: &config,
					ImageSpec: ImageSpec{
						Image:            "sriov-network-device-plugin",
						Repository:       "nvcr.io/nvstaging/mellanox",
						Version:          "network-operator-23.10.0-beta.1",
						ImagePullSecrets: []string{},
					},
				},
			},
		},
	}
}
