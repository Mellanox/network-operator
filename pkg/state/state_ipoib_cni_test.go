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
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

var _ = Describe("IPoIB CNI State tests", func() {

	Context("GetNodesAttributes with provide", func() {
		It("Should Apply", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ipoib-cni"
			scheme := runtime.NewScheme()

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			stateName := "state-ipoib-cni"
			sriovDpState := stateIPoIBCNI{
				stateSkel: stateSkel{
					name:        stateName,
					description: "IPoIB CNI deployed in the cluster",
					client:      &client,
					scheme:      scheme,
					renderer:    renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(sriovDpState.Name()).To(Equal(stateName))

			cr := &mellanoxv1alpha1.NicClusterPolicy{}

			imageSpec := &mellanoxv1alpha1.ImageSpec{
				Image:      "image",
				Repository: "Repository",
				Version:    "v0.0",
			}
			netSpec := &mellanoxv1alpha1.SecondaryNetworkSpec{}
			netSpec.IPoIB = imageSpec
			cr.Spec.SecondaryNetwork = netSpec

			nodeAffinitySpec := "{\"requiredDuringSchedulingIgnoredDuringExecution\":{\"nodeSelectorTerms\":" +
				"[{\"matchExpressions\":[{\"key\":\"node-role.kubernetes.io/master\"," +
				"\"operator\":\"DoesNotExist\"}]}]}}"

			nodeAffinity := &v1.NodeAffinity{}
			_ = json.Unmarshal([]byte(nodeAffinitySpec), &nodeAffinity)

			cr.Spec.NodeAffinity = nodeAffinity

			staticConfig := &dummyProvider{}
			nodeInfo := &dummyProvider{}
			objs, err := sriovDpState.getManifestObjects(cr, staticConfig, nodeInfo, testLogger)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))
		})
	})
})
