/*
2025 NVIDIA CORPORATION & AFFILIATES

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

package state_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

//nolint:dupl
var _ = Describe("Spectrum X Operator", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateSpectrumXOperator, "../../manifests/state-spectrum-x-operator")
		Expect(ts).NotTo(BeNil())
	})

	Context("should render", func() {
		It("Kubernetes manifests", func() {
			cr := getSpectrumXOperator()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(5))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, &cr.Spec.SpectrumXOperator.ImageSpec, ts.renderer)
		})
		It("Openshift manifests", func() {
			cr := getSpectrumXOperator()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.openshiftCatalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(7))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, &cr.Spec.SpectrumXOperator.ImageSpec, ts.renderer)
		})
	})
	Context("should sync", func() {
		It("without any errors", func() {
			cr := getSpectrumXOperator()
			err := ts.client.Create(ts.context, cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			// We do not expect that the sync state (i.e. the DaemonSet) will be ready.
			// There is no real Kubernetes cluster in the unit tests and thus the Pods cannot be scheduled.
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})
	})
})

func getSpectrumXOperator() *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()
	imageSpec := getTestImageSpec()
	imageSpec = addContainerResources(imageSpec, "spectrum-x-operator", "1", "9")
	imageSpec = addContainerResources(imageSpec, "spectrum-x-flowcontroller", "1", "9")
	cr.Name = "nic-cluster-policy"
	cr.Spec.SpectrumXOperator = &mellanoxv1alpha1.SpectrumXOperatorSpec{
		ImageSpec: *imageSpec,
		SpectrumXConfig: &mellanoxv1alpha1.ConfigMapNameReference{
			Name: "spectrum-x-config",
		},
		SriovObjNamespace: "default",
	}
	return cr
}
