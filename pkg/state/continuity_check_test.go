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
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
)

func extractContainerNamesFromHelmChart(path string) ([]string, error) {
	//nolint:gosec
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var parsedData map[string]interface{}
	err = yaml.Unmarshal(uncommentContainerResources(data), &parsedData)
	if err != nil {
		return nil, err
	}

	containerNames := extractContainerNamesFromSubsection(parsedData)

	return containerNames, nil
}

// uncommentContainerResources iterates through the document and removes '#' comments from containerResources sections
func uncommentContainerResources(fileData []byte) []byte {
	var result bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(fileData))
	var processNextLines bool

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "# containerResources:" {
			// Remove "# " and set flag to process next lines
			line = strings.Replace(line, "# ", "", 1)
			processNextLines = true
		} else if processNextLines && strings.HasPrefix(strings.TrimSpace(line), "#  ") {
			// For subsequent lines starting with "#  ", remove "# "
			line = strings.Replace(line, "# ", "", 1)
		} else {
			processNextLines = false
		}

		result.WriteString(line + "\n")
	}

	return result.Bytes()
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

			manifestsBaseDir := filepath.Join("..", "..", "manifests")
			envConfig = &config.OperatorConfig{State: config.StateConfig{ManifestBaseDir: manifestsBaseDir}}
			states, err := newNicClusterPolicyStates(nil)
			Expect(err).NotTo(HaveOccurred())

			for _, state := range states {
				names, err := ParseContainerNames(state.(ManifestRenderer), cr, testLogger)
				Expect(err).NotTo(HaveOccurred())
				namesFromManifests = append(namesFromManifests, names...)
			}

			sort.Strings(namesFromChart)
			sort.Strings(namesFromManifests)
			Expect(namesFromChart).To(Equal(namesFromManifests))

		})
	})
})
