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

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("IPoIB CNI State tests", func() {
	var ctx context.Context
	var catalog state.InfoCatalog
	var client client.Client
	var cniState state.State
	var cniRenderer state.ManifestRenderer

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(netattdefv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir := "../../manifests/state-ipoib-cni"
		s, r, err := state.NewStateIPoIBCNI(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		cniState = s
		cniRenderer = r
		catalog = getTestCatalog()
		catalog.Add(state.InfoTypeStaticConfig,
			staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "custom-cni-bin-directory"}))
	})

	Context("should render", func() {
		It("manifests with IPoIB CNI", func() {
			cr := getNICForIPoIBCNI()
			objs, err := cniRenderer.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))
			GetManifestObjectsTest(ctx, cr, catalog, cr.Spec.SecondaryNetwork.IPoIB, cniRenderer)
		})
		It("manifests with IPoIB CNI for Openshift", func() {
			cr := getNICForIPoIBCNI()
			openshiftCatalog := getOpenshiftTestCatalog()
			objs, err := cniRenderer.GetManifestObjects(context.TODO(), cr, openshiftCatalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			GetManifestObjectsTest(ctx, cr, catalog, cr.Spec.SecondaryNetwork.IPoIB, cniRenderer)
		})
	})

	Context("should sync", func() {
		It("without any errors", func() {
			cr := getNICForIPoIBCNI()
			err := client.Create(ctx, cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := cniState.Sync(ctx, cr, catalog)
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
