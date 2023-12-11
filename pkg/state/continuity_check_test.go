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

package state

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

func extractContainerNamesFromHelmChart(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var parsedData map[string]interface{}
	err = yaml.Unmarshal(data, &parsedData)
	if err != nil {
		return nil, err
	}

	containerNames := extractContainerNamesFromSubsection(parsedData)

	return containerNames, nil
}

//nolint:gocognit
func extractContainerNamesFromSubsection(data interface{}) []string {
	var names []string

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			names = append(names, extractContainerNamesFromSubsection(item)...)
		}
	case map[string]interface{}:
		for key, value := range v {
			if key == "containerResources" {
				if resources, ok := value.([]interface{}); ok {
					for _, resource := range resources {
						if resMap, ok := resource.(map[string]interface{}); ok {
							if name, ok := resMap["name"].(string); ok {
								names = append(names, name)
							}
						}
					}
				}
			} else {
				names = append(names, extractContainerNamesFromSubsection(value)...)
			}
		}
	}

	return names
}

func extractContainerNamesFromState(manifestsDirectoryPath string, state ManifestRenderer,
	cr *mellanoxv1alpha1.NicClusterPolicy, logger logr.Logger) ([]string, error) {
	files, err := utils.GetFilesWithSuffix(manifestsDirectoryPath, render.ManifestFileSuffix...)
	if err != nil {
		return nil, err
	}

	state.SetRenderer(render.NewRenderer(files))

	return ParseContainerNames(state, cr, logger)
}

var _ = Describe("Continuity check", func() {

	Context("Resource requirements", func() {
		It("Resource requirements from helm chart should cover all deployable containers", func() {
			wd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			chartPath := filepath.Join(wd, "..", "..", "deployment", "network-operator", "values.yaml")

			namesFromChart, err := extractContainerNamesFromHelmChart(chartPath)
			Expect(err).NotTo(HaveOccurred())

			var namesFromManifests []string

			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			imageSpec := mellanoxv1alpha1.ImageSpec{Image: "image", Repository: "", Version: "version"}
			imageSpecWithConfig := mellanoxv1alpha1.ImageSpecWithConfig{ImageSpec: imageSpec}
			cr.Spec.IBKubernetes = &mellanoxv1alpha1.IBKubernetesSpec{ImageSpec: imageSpec}
			cr.Spec.OFEDDriver = &mellanoxv1alpha1.OFEDDriverSpec{ImageSpec: imageSpec}
			cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{ImageSpecWithConfig: imageSpecWithConfig}
			cr.Spec.SriovDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{ImageSpecWithConfig: imageSpecWithConfig}
			cr.Spec.NvIpam = &mellanoxv1alpha1.NVIPAMSpec{ImageSpec: imageSpec}
			cr.Spec.NicFeatureDiscovery = &mellanoxv1alpha1.NICFeatureDiscoverySpec{ImageSpec: imageSpec}
			cr.Spec.SecondaryNetwork = &mellanoxv1alpha1.SecondaryNetworkSpec{}
			cr.Spec.SecondaryNetwork.CniPlugins = &imageSpec
			cr.Spec.SecondaryNetwork.IpamPlugin = &imageSpec
			cr.Spec.SecondaryNetwork.IPoIB = &imageSpec
			cr.Spec.SecondaryNetwork.Multus = &mellanoxv1alpha1.MultusSpec{ImageSpecWithConfig: imageSpecWithConfig}

			stateMap := map[string]ManifestRenderer{
				"state-ib-kubernetes":                &stateIBKubernetes{},
				"state-ofed-driver":                  &stateOFED{},
				"state-rdma-device-plugin":           &stateSharedDp{},
				"state-sriov-device-plugin":          &stateSriovDp{},
				"state-nv-ipam-cni":                  &stateNVIPAMCNI{},
				"state-nic-feature-discovery":        &stateNICFeatureDiscovery{},
				"state-container-networking-plugins": &stateCNIPlugins{},
				"state-whereabouts-cni":              &stateWhereaboutsCNI{},
				"state-ipoib-cni":                    &stateIPoIBCNI{},
				"state-multus-cni":                   &stateMultusCNI{},
			}

			for manifestsDirectory, state := range stateMap {
				names, err := extractContainerNamesFromState(
					filepath.Join("..", "..", "manifests", manifestsDirectory), state, cr, testLogger)
				Expect(err).NotTo(HaveOccurred())
				namesFromManifests = append(namesFromManifests, names...)
			}

			sort.Strings(namesFromChart)
			sort.Strings(namesFromManifests)

			Expect(namesFromChart).To(Equal(namesFromManifests))

		})
	})
})
