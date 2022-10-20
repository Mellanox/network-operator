/*
2022 NVIDIA CORPORATION & AFFILIATES

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
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

var _ = Describe("IB Kubernetes state rendering tests", func() {

	Context("IB Kubernetes state", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			client := mocks.ControllerRutimeClient{}
			manifestBaseDir := "../../manifests/stage-ib-kubernetes"
			scheme := runtime.NewScheme()

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			stateName := "state-ib-kubernetes"
			ibKubernetesState := stateIBKubernetes{
				stateSkel: stateSkel{
					name:        stateName,
					description: "ib-kubernetes deployed in the cluster",
					client:      &client,
					scheme:      scheme,
					renderer:    renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(ibKubernetesState.Name()).To(Equal(stateName))

			envValues := map[string]string{
				"DAEMON_SM_PLUGIN":       "ufm",
				"DAEMON_PERIODIC_UPDATE": "6",
				"GUID_POOL_RANGE_START":  "rangestart",
				"GUID_POOL_RANGE_END":    "rangeend",
			}
			ufmSecret := "secret"
			ibKubernetesSpec := &mellanoxv1alpha1.IBKubernetesSpec{}
			ibKubernetesSpec.PeriodicUpdateSeconds, _ = strconv.Atoi(envValues["DAEMON_PERIODIC_UPDATE"])
			ibKubernetesSpec.PKeyGUIDPoolRangeStart = envValues["GUID_POOL_RANGE_START"]
			ibKubernetesSpec.PKeyGUIDPoolRangeEnd = envValues["GUID_POOL_RANGE_END"]
			ibKubernetesSpec.UfmSecret = ufmSecret
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"

			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			cr.Spec.IBKubernetes = ibKubernetesSpec
			objs, err := ibKubernetesState.getManifestObjects(cr, &dummyProvider{})

			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			ds := objs[3]
			Expect(ds.GetKind()).To(Equal("Deployment"))
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})
			containers := templateSpec["containers"].([]interface{})
			env := containers[0].(map[string]interface{})["env"].([]interface{})

			for _, envVar := range env {
				if _, ok := envVar.(map[string]interface{})["value"]; ok {
					value := envVar.(map[string]interface{})["value"].(string)
					name := envVar.(map[string]interface{})["name"].(string)
					Expect(value).To(Equal(envValues[name]))
				}
			}
		})
	})
})
