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
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

var _ = Describe("IPoIBNetwork Network state rendering tests", func() {

	Context("IPoIBNetwork Network state", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ipoib-network"
			scheme := runtime.NewScheme()

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			stateName := "state-ipoib-network"
			ipoibState := stateIPoIBNetwork{
				stateSkel: stateSkel{
					name:        stateName,
					description: "IPoIBNetwork net-attach-def CR deployed in cluster",
					client:      &client,
					scheme:      scheme,
					renderer:    renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(ipoibState.Name()).To(Equal(stateName))

			namespace := "namespace"
			name := "ibs3"
			ipam := "fakeIPAM"
			spec := &mellanoxv1alpha1.IPoIBNetworkSpec{}
			spec.NetworkNamespace = namespace
			spec.Master = name
			spec.IPAM = ipam

			cr := &mellanoxv1alpha1.IPoIBNetwork{}
			cr.Name = name
			cr.Spec = *spec
			objs, err := ipoibState.getManifestObjects(cr, testLogger)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))

			checkRenderedNetAttachDef(objs[0], namespace, name, ipam)
		})
	})
})
