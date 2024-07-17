/*
2024 NVIDIA CORPORATION & AFFILIATES

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("MacVlan Network State rendering tests", func() {
	const (
		testNamespace  = "namespace"
		testName       = "mac-vlan"
		testType       = "macvlan"
		testMaster     = "eth0"
		testBridgeMode = "bridge"
		testMtu        = 150
	)

	var macvlanState state.State
	var catalog state.InfoCatalog
	var client client.Client

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(netattdefv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir := "../../manifests/state-macvlan-network"
		s, err := state.NewStateMacvlanNetwork(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		macvlanState = s
		catalog = getTestCatalog()
	})

	Context("MacVlan Network State", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			cr := getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode, testMtu)
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			expectedNad := getExpectedNAD(testName, testMaster, testBridgeMode, "{}", testMtu)
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal(testNamespace))
		})
		It("Should Render NetworkAttachmentDefinition with IPAM", func() {
			ipam := `{"type":"whereabouts","range":"192.168.2.225/28",` +
				`"exclude":["192.168.2.229/30","192.168.2.236/32"]}`
			cr := getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode, testMtu)
			cr.Spec.IPAM = ipam
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal(testNamespace))

			expectedNad := getExpectedNAD(testName, testMaster, testBridgeMode, ipam, testMtu)
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
		})
		It("Should Render NetworkAttachmentDefinition with default namespace", func() {
			cr := getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode, testMtu)
			cr.Spec.NetworkNamespace = ""
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal("default"))
		})
	})

	Context("Verify Sync flows", func() {
		It("Should recreate NetworkAttachmentDefinition with different namespace", func() {
			cr := getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode, testMtu)
			cr.Spec.NetworkNamespace = ""
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal("default"))

			By("Update network namespace")
			cr = &mellanoxv1alpha1.MacvlanNetwork{}
			err = client.Get(context.Background(), types.NamespacedName{Name: testName}, cr)
			Expect(err).NotTo(HaveOccurred())
			cr.Spec.NetworkNamespace = testNamespace
			err = client.Update(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())

			By("Sync")
			_, err = macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			err = client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: testName}, nad)
			Expect(errors.IsNotFound(err)).To(BeTrue())
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			expectedNad := getExpectedNAD(testName, testMaster, testBridgeMode, "{}", testMtu)
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal(testNamespace))
		})
	})
})

func getExpectedNAD(testName, testMaster, testBridgeMode, ipam string, testMtu int,
) *netattdefv1.NetworkAttachmentDefinition {
	nad := &netattdefv1.NetworkAttachmentDefinition{}
	cfg := fmt.Sprintf(`{ "cniVersion":"0.3.1", "name":%q, "type":"macvlan","master": %q,`+
		`"mode" : %q,"mtu" : %d,"ipam":%s }`,
		testName, testMaster, testBridgeMode, testMtu, ipam)
	nad.Spec.Config = cfg
	return nad
}

// nolint:unparam
func getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode string, testMtu int,
) *mellanoxv1alpha1.MacvlanNetwork {
	cr := &mellanoxv1alpha1.MacvlanNetwork{
		Spec: mellanoxv1alpha1.MacvlanNetworkSpec{
			NetworkNamespace: testNamespace,
			Master:           testMaster,
			Mode:             testBridgeMode,
			Mtu:              testMtu,
		},
	}
	cr.Name = testName
	return cr
}
