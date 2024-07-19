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
		testNamespace  = "macvlan"
		testName       = "mac-vlan"
		testType       = "macvlan"
		testMaster     = "eth0"
		testBridgeMode = "bridge"
		testMtu        = 150
	)

	var macvlanState state.State
	var catalog state.InfoCatalog
	var client client.Client
	var expectedNadConfig nadConfig

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
		expectedNadConfig = defaultNADConfig(&nadConfig{
			Name:   testName,
			Type:   testType,
			Master: testMaster,
			Mode:   testBridgeMode,
			IPAM:   nadConfigIPAM{},
			MTU:    testMtu,
		})
	})

	Context("MacVlan Network State", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			cr := getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode, testMtu)
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(client, &expectedNadConfig, testName, testNamespace, "")
		})
		It("Should Render NetworkAttachmentDefinition with IPAM", func() {
			ipam := nadConfigIPAM{
				Type:    "whereabouts",
				Range:   "192.168.2.225/28",
				Exclude: []string{"192.168.2.229/30", "192.168.2.236/32"},
			}
			cr := getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode, testMtu)
			cr.Spec.IPAM = getNADConfigIPAMJSON(ipam)
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = ipam
			assertNetworkAttachmentDefinition(client, &expectedNadConfig, testName, testNamespace, "")
		})
		It("Should Render NetworkAttachmentDefinition with default namespace", func() {
			cr := getMacvlanNetwork(testName, testNamespace, testMaster, testBridgeMode, testMtu)
			cr.Spec.NetworkNamespace = ""
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := macvlanState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(client, &expectedNadConfig, testName, "default", "")
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

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(client, &expectedNadConfig, testName, "default", "")

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
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: testName}, nad)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			assertNetworkAttachmentDefinition(client, &expectedNadConfig, testName, testNamespace, "")
		})
	})
})

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
