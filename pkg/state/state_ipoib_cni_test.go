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

package state_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("IPoIB CNI State tests", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateIPoIBCNI, "../../manifests/state-ipoib-cni")
		Expect(ts).NotTo(BeNil())

		ts.catalog.Add(state.InfoTypeStaticConfig,
			staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "custom-cni-bin-directory"}))
	})

	Context("should render", func() {
		It("manifests with IPoIB CNI", func() {
			cr := getNICForIPoIBCNI()
			objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, cr.Spec.SecondaryNetwork.IPoIB, ts.renderer)
		})
		It("manifests with IPoIB CNI - image as SHA256", func() {
			cr := getNICForIPoIBCNI()
			cr.Spec.SecondaryNetwork.IPoIB.Version = defaultTestVersionSha256
			objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, cr.Spec.SecondaryNetwork.IPoIB, ts.renderer)
		})
		It("manifests with IPoIB CNI for Openshift", func() {
			cr := getNICForIPoIBCNI()
			objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.openshiftCatalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, cr.Spec.SecondaryNetwork.IPoIB, ts.renderer)
		})
	})

	Context("should sync", func() {
		It("without any errors", func() {
			cr := getNICForIPoIBCNI()
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

func getNICForIPoIBCNI() *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()
	imageSpec := addContainerResources(getTestImageSpec(), "ipoib-cni", "1", "9")
	cr.Name = "ipoib-cni-nic-cluster-policy"
	cr.Spec.SecondaryNetwork = &mellanoxv1alpha1.SecondaryNetworkSpec{
		IPoIB: imageSpec,
	}
	return cr
}
