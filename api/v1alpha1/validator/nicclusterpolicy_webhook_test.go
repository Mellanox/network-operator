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
package validator //nolint:dupl

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mellanox/network-operator/api/v1alpha1"
	env "github.com/Mellanox/network-operator/pkg/config"
)

//nolint:dupl
var _ = Describe("Validate", func() {
	Context("NicClusterPolicy tests", func() {
		BeforeEach(func() {
			envConfig = env.StateConfig{
				ManifestBaseDir: "../../../manifests",
			}
		})
		It("Valid GUID range", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					IBKubernetes: &v1alpha1.IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00:00:00:00:00",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00:00:00:00:01",
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}

			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Invalid GUID range", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					IBKubernetes: &v1alpha1.IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00:00:00:00:02",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00:00:00:00:00",
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"pKeyGUIDPoolRangeStart-pKeyGUIDPoolRangeEnd must be a valid range"))
		})
		It("Invalid start and end GUID", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					IBKubernetes: &v1alpha1.IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00",
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(And(
				ContainSubstring("pKeyGUIDPoolRangeStart must be a valid GUID format"),
				ContainSubstring("pKeyGUIDPoolRangeEnd must be a valid GUID format")))
		})
		It("Valid MOFED version (old scheme)", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Valid MOFED version (old scheme with container version suffix)", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0.1",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Valid MOFED version", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "24.01-0.3.3.1-0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("InValid MOFED version", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23-10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("invalid OFED version"))
		})
		It("MOFED SafeLoad requires AutoUpgrade to be enabled", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
						OfedUpgradePolicy: &v1alpha1.DriverUpgradePolicySpec{
							SafeLoad: true,
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("autoUpgrade"))
		})
		It("MOFED valid SafeLoad config", func() {
			validator := nicClusterPolicyValidator{}
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
						OfedUpgradePolicy: &v1alpha1.DriverUpgradePolicySpec{
							SafeLoad:    true,
							AutoUpgrade: true,
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).To(BeNil())
		})
		It("RDMA no config", func() {
			nicClusterPolicy := rdmaDPNicClusterPolicy(nil)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := rdmaDPNicClusterPolicy(&rdmaConfig)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := rdmaDPNicClusterPolicy(&rdmaConfig)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"Invalid json of RdmaSharedDevicePluginConfig"))
		})
		It("Invalid RDMA config JSON schema, empty configList", func() {
			invalidRdmaConfigJSON := `{
						"configList": []}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("Array must have at least 1 items"))
		})
		It("Invalid RDMA config JSON schema, resourceName not valid", func() {
			invalidRdmaConfigJSON := `{
				"configList": [{
					"resourceName": "rdma-shared-device-a!!",
					"rdmaHcaMax": 63,
					"selectors": {
						"vendors": ["15b3"],
						"deviceIDs": ["101b"]}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("configList is required"))
		})
		It("Invalid RDMA config JSON schema, none of the selectors are provided", func() {
			invalidRdmaConfigJSON := `{
				"configList": [{
					"resourceName": "rdma_shared_device_a",
					"rdmaHcaMax": 63,
					"selectors": {}}]}`
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := rdmaDPNicClusterPolicy(&invalidRdmaConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"Invalid Resource prefix, it must be a valid FQDN"))
		})
		It("SriovDevicePlugin no config", func() {
			nicClusterPolicy := sriovDPNicClusterPolicy(nil)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Valid SriovDevicePlugin config JSON", func() {
			sriovConfig := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&sriovConfig)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Valid SriovDevicePlugin config JSON, selectors object is a list ", func() {
			sriovConfig := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": [{
						"vendors": ["15b3"],
						"devices": ["101b"]}]}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&sriovConfig)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Invalid SriovDevicePlugin config JSON, missing starting {", func() {
			invalidSriovConfigJSON := `
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"Invalid json of SriovNetworkDevicePluginConfig"))
		})
		It("Invalid SriovDevicePlugin config JSON schema, empty resourceList", func() {
			invalidSriovConfigJSON := `{
						"resourceList": []}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("Array must have at least 1 items"))
		})
		It("Invalid SriovDevicePlugin config JSON schema, resourceName not valid", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "sriov-network-device-plugin",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("Invalid Resource name"))
		})
		It("Invalid SriovDevicePlugin config JSON schema, no resourceList provided", func() {
			invalidSriovConfigJSON := `{
				"resourceList_a": [{
					"resourceName": "sriov_network_device_plugin",
					"selectors": {
						"vendors": ["15b3"],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("resourceList is required"))
		})
		It("Invalid SriovDevicePlugin config JSON schema, none of the selectors are provided", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "sriov_network_device_plugin",
					"selectors": {}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("vendors is required"))
		})
		It("Invalid SriovDevicePlugin config JSON, vendors must be list of strings", func() {
			invalidSriovConfigJSON := `{
				"resourceList": [{
					"resourceName": "hostdev",
					"selectors": {
						"vendors": [15],
						"devices": ["101b"]}}]}`
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
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
			nicClusterPolicy := sriovDPNicClusterPolicy(&invalidSriovConfigJSON)
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), &nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"Invalid Resource prefix, it must be a valid FQDN"))
		})
	})
	Context("Image repository tests", func() {
		It("Invalid Repository IBKubernetes", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					IBKubernetes: &v1alpha1.IBKubernetesSpec{
						PKeyGUIDPoolRangeStart: "00:00:00:00:00:00:00:00",
						PKeyGUIDPoolRangeEnd:   "00:00:00:00:00:00:00:02",
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "ib-kubernetes",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "v1.0.2",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository OFEDDriver", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
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
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					RdmaSharedDevicePlugin: &v1alpha1.DevicePluginSpec{
						ImageSpecWithConfig: v1alpha1.ImageSpecWithConfig{
							Config: &rdmaConfig,
							ImageSpec: v1alpha1.ImageSpec{
								Image:            "k8s-rdma-shared-dev-plugin",
								Repository:       "ghcr.io/mellanox!@!#$!",
								Version:          "sha-fe7f371c7e1b8315bf900f71cd25cfc1251dc775",
								ImagePullSecrets: []string{},
							},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
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
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					SriovDevicePlugin: &v1alpha1.DevicePluginSpec{
						ImageSpecWithConfig: v1alpha1.ImageSpecWithConfig{
							Config: &sriovConfig,
							ImageSpec: v1alpha1.ImageSpec{
								Image:            "sriov-network-device-plugin",
								Repository:       "nvcr.io/nvstaging/mellanox!@!#$!",
								Version:          "network-operator-23.10.0-beta.1",
								ImagePullSecrets: []string{},
							},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository NVIPAM", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					NvIpam: &v1alpha1.NVIPAMSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository NicFeatureDiscovery", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					NicFeatureDiscovery: &v1alpha1.NICFeatureDiscoverySpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork Multus", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					SecondaryNetwork: &v1alpha1.SecondaryNetworkSpec{
						Multus: &v1alpha1.MultusSpec{
							ImageSpecWithConfig: v1alpha1.ImageSpecWithConfig{
								ImageSpec: v1alpha1.ImageSpec{
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
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork Multus", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					SecondaryNetwork: &v1alpha1.SecondaryNetworkSpec{
						Multus: &v1alpha1.MultusSpec{
							ImageSpecWithConfig: v1alpha1.ImageSpecWithConfig{
								ImageSpec: v1alpha1.ImageSpec{
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
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork CniPlugins", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					SecondaryNetwork: &v1alpha1.SecondaryNetworkSpec{
						CniPlugins: &v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork IPoIB", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					SecondaryNetwork: &v1alpha1.SecondaryNetworkSpec{
						IPoIB: &v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Invalid Repository SecondaryNetwork IpamPlugin", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					SecondaryNetwork: &v1alpha1.SecondaryNetworkSpec{
						IpamPlugin: &v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox!@!#$!",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"invalid container image repository format"))
		})
		It("Empty ContainerResources OFEDDriver", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:              "mofed",
							Repository:         "ghcr.io/mellanox",
							Version:            "23.10-0.2.2.0",
							ImagePullSecrets:   []string{},
							ContainerResources: []v1alpha1.ResourceRequirements{},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Resource Requests > Limits OFEDDriver", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
							ContainerResources: []v1alpha1.ResourceRequirements{
								{
									Name:     "mofed-container",
									Requests: v1.ResourceList{"cpu": resource.MustParse("500Mi")},
									Limits:   v1.ResourceList{"cpu": resource.MustParse("100Mi")},
								},
							},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"resource request for cpu is greater than the limit"))
		})
		It("Invalid Resource Requests OFEDDriver", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
							ContainerResources: []v1alpha1.ResourceRequirements{
								{
									Name:     "mofed-container",
									Requests: v1.ResourceList{"cpu": resource.MustParse("0Mi")},
								},
							},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"resource Requests for cpu is zero"))
		})
		It("Unsupported Resource Request Type OFEDDriver", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
							ContainerResources: []v1alpha1.ResourceRequirements{
								{
									Name:     "mofed-container",
									Requests: v1.ResourceList{"ephemeral-storage": resource.MustParse("2Gi")},
								},
							},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"Unsupported value: ephemeral-storage: supported values: \"cpu\", \"memory\""))
		})
		It("Invalid Resource Requests Container Name OFEDDriver", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "ghcr.io/mellanox",
							Version:          "23.10-0.2.2.0",
							ImagePullSecrets: []string{},
							ContainerResources: []v1alpha1.ResourceRequirements{
								{
									Name:     "invalid-container-name",
									Requests: v1.ResourceList{"cpu": resource.MustParse("100Mi")},
								},
							},
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring(
				"Unsupported value: \"invalid-container-name\": supported values: \"mofed-container\""))
		})
		It("passes when DocaTelemetryService imageSpec is valid", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					DOCATelemetryService: &v1alpha1.DOCATelemetryServiceSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "doca-telemetry-service",
							Repository: "ghcr.io/mellanox",
							Version:    "1.2",
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).NotTo(HaveOccurred())
		})
		It("fails when DocaTelemetryService has an invalid repository", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					DOCATelemetryService: &v1alpha1.DOCATelemetryServiceSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "doca-telemetry-service",
							Repository: "{ghcr.io/mellanox}",
							Version:    "1.2",
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("spec.docaTelemetryService.repository: Invalid value"))
		})
		It("passes when config.ConfigMap is valid", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					DOCATelemetryService: &v1alpha1.DOCATelemetryServiceSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "doca-telemetry-service",
							Repository: "ghcr.io/mellanox",
							Version:    "1.2",
						},
						Config: &v1alpha1.DOCATelemetryServiceConfig{
							// ConfigMap must not be empty.
							FromConfigMap: "telemetry-configmap",
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err).ToNot(HaveOccurred())
		})
		It("fails when config.ConfigMap is too short", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					DOCATelemetryService: &v1alpha1.DOCATelemetryServiceSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "doca-telemetry-service",
							Repository: "ghcr.io/mellanox",
							Version:    "1.2",
						},
						Config: &v1alpha1.DOCATelemetryServiceConfig{
							// ConfigMap has a minimum length.
							FromConfigMap: "",
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("a lowercase RFC 1123 subdomain must consist of"))
		})
		It("fails when config.ConfigMap contains invalid characters", func() {
			nicClusterPolicy := &v1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.NicClusterPolicySpec{
					DOCATelemetryService: &v1alpha1.DOCATelemetryServiceSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "doca-telemetry-service",
							Repository: "ghcr.io/mellanox",
							Version:    "1.2",
						},
						Config: &v1alpha1.DOCATelemetryServiceConfig{
							// ConfigMap must not contain `/`.
							FromConfigMap: "telemetry/configmap",
						},
					},
				},
			}
			validator := nicClusterPolicyValidator{}
			_, err := validator.ValidateCreate(context.TODO(), nicClusterPolicy)
			Expect(err.Error()).To(ContainSubstring("a lowercase RFC 1123 subdomain must consist of"))
		})
	})
})

func rdmaDPNicClusterPolicy(config *string) v1alpha1.NicClusterPolicy {
	return v1alpha1.NicClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1alpha1.NicClusterPolicySpec{
			RdmaSharedDevicePlugin: &v1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: v1alpha1.ImageSpecWithConfig{
					Config: config,
					ImageSpec: v1alpha1.ImageSpec{
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

func sriovDPNicClusterPolicy(config *string) v1alpha1.NicClusterPolicy {
	return v1alpha1.NicClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1alpha1.NicClusterPolicySpec{
			SriovDevicePlugin: &v1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: v1alpha1.ImageSpecWithConfig{
					Config: config,
					ImageSpec: v1alpha1.ImageSpec{
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
