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

package state

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

func checkResourceNameAnnotation(obj *unstructured.Unstructured) {
	annotations := obj.Object["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
	resourceName := annotations["k8s.v1.cni.cncf.io/resourceName"].(string)

	Expect(HavePrefix(resourceName, resourceNamePrefix))
	Expect(strings.Count(resourceName, resourceNamePrefix)).To(Equal(1))
}

var _ = Describe("HostDevice Network State rendering tests", func() {

	Context("HostDevice Network State", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-hostdevice-network"
			scheme := runtime.NewScheme()

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			stateName := "state-host-device-network"
			sriovDpState := stateHostDeviceNetwork{
				stateSkel: stateSkel{
					name:        stateName,
					description: "Host Device net-attach-def CR deployed in cluster",
					client:      &client,
					scheme:      scheme,
					renderer:    renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(sriovDpState.Name()).To(Equal(stateName))

			namespace := "namespace"
			name := "test_resource_without_prefix"
			ipam := "fake IPAM"
			spec := &mellanoxv1alpha1.HostDeviceNetworkSpec{}
			spec.NetworkNamespace = namespace
			spec.ResourceName = name
			spec.IPAM = ipam

			cr := &mellanoxv1alpha1.HostDeviceNetwork{}
			cr.Name = name
			cr.Spec = *spec
			objs, err := sriovDpState.getManifestObjects(cr, testLogger)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))

			checkRenderedNetAttachDef(objs[0], namespace, name, ipam)
			checkResourceNameAnnotation(objs[0])

			spec.ResourceName = resourceNamePrefix + "test_resource_with_prefix"
			objs, err = sriovDpState.getManifestObjects(cr, testLogger)

			Expect(err).NotTo(HaveOccurred())
			checkResourceNameAnnotation(objs[0])
		})
	})
})
