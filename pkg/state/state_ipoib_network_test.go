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
	"fmt"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("IPoIBNetwork Network state rendering tests", func() {
	const (
		testNamespace = "namespace"
	)

	var ipoibState state.State
	var catalog state.InfoCatalog
	var client client.Client

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(netattdefv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir := "../../manifests/state-ipoib-network"
		s, err := state.NewStateIPoIBNetwork(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		ipoibState = s
		catalog = getTestCatalog()
	})

	Context("IPoIBNetwork Network state", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			name := "ipoib"
			cr := getIPoIBNetwork(testNamespace)
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ipoibState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: name}, nad)
			Expect(err).NotTo(HaveOccurred())
			cfg := getNADConfig(nad.Spec.Config)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Type).To(Equal("ipoib"))
			expectedNad := getExpectedIPoIBNAD(name, cr.Spec.Master, "{}")
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
			Expect(nad.Name).To(Equal(name))
			Expect(nad.Namespace).To(Equal(testNamespace))
		})
		It("Should Render NetworkAttachmentDefinition with IPAM", func() {
			ipam := `{"type":"whereabouts","range":"192.168.2.225/28","exclude":["192.168.2.229/30","192.168.2.236/32"]}`
			name := "ipoib"
			cr := getIPoIBNetwork(testNamespace)
			cr.Spec.IPAM = ipam
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ipoibState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: name}, nad)
			Expect(err).NotTo(HaveOccurred())
			cfg := getNADConfig(nad.Spec.Config)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Type).To(Equal("ipoib"))
			Expect(nad.Name).To(Equal(name))
			Expect(nad.Namespace).To(Equal(testNamespace))
			expectedNad := getExpectedIPoIBNAD(name, cr.Spec.Master, ipam)
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
		})
		It("Should Render NetworkAttachmentDefinition with default namespace", func() {
			name := "ipoib"
			cr := getIPoIBNetwork("")
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ipoibState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, nad)
			Expect(err).NotTo(HaveOccurred())
			Expect(nad.Name).To(Equal(name))
			Expect(nad.Namespace).To(Equal("default"))
		})
	})
	Context("Verify Sync flows", func() {
		It("Should recreate NetworkAttachmentDefinition with different namespace", func() {
			name := "ipoib"
			cr := getIPoIBNetwork("")
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ipoibState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, nad)
			Expect(err).NotTo(HaveOccurred())
			Expect(nad.Name).To(Equal(name))
			Expect(nad.Namespace).To(Equal("default"))

			By("Update network namespace")
			cr = &mellanoxv1alpha1.IPoIBNetwork{}
			err = client.Get(context.Background(), types.NamespacedName{Name: name}, cr)
			Expect(err).NotTo(HaveOccurred())
			cr.Spec.NetworkNamespace = testNamespace
			err = client.Update(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())

			By("Sync")
			_, err = ipoibState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			err = client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, nad)
			Expect(errors.IsNotFound(err)).To(BeTrue())
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: name}, nad)
			Expect(err).NotTo(HaveOccurred())
			expectedNad := getExpectedIPoIBNAD(name, cr.Spec.Master, "{}")
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
			Expect(nad.Name).To(Equal(name))
			Expect(nad.Namespace).To(Equal(testNamespace))
		})
	})
})

func getExpectedIPoIBNAD(testName, testMaster, ipam string) *netattdefv1.NetworkAttachmentDefinition {
	nad := &netattdefv1.NetworkAttachmentDefinition{}
	cfg := fmt.Sprintf(`{ "cniVersion":"0.3.1", "name":%q, "type":"ipoib", "master": %q, "ipam":%s }`,
		testName, testMaster, ipam)
	nad.Spec.Config = cfg
	return nad
}

func getIPoIBNetwork(testNamespace string) *mellanoxv1alpha1.IPoIBNetwork {
	cr := &mellanoxv1alpha1.IPoIBNetwork{
		Spec: mellanoxv1alpha1.IPoIBNetworkSpec{
			NetworkNamespace: testNamespace,
			Master:           "eth0",
		},
	}
	cr.Name = "ipoib"
	return cr
}
