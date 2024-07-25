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

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("IPoIBNetwork Network state rendering tests", func() {
	const (
		testName      = "ipo-ib"
		testNamespace = "ipoib"
		testType      = "ipoib"
		testMaster    = "eth0"
	)

	var (
		ts                testScope
		expectedNadConfig nadConfig
	)

	BeforeEach(func() {
		ts = ts.New(state.NewStateIPoIBNetwork, "../../manifests/state-ipoib-network")
		Expect(ts).NotTo(BeNil())
		expectedNadConfig = defaultNADConfig(&nadConfig{
			Name:   testName,
			Type:   testType,
			Master: testMaster,
			IPAM:   nadConfigIPAM{},
		})
	})

	Context("IPoIBNetwork Network state", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			cr := getIPoIBNetwork(testName, testNamespace, testMaster)
			err := ts.client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, testNamespace, "")
		})
		It("Should Render NetworkAttachmentDefinition with IPAM", func() {
			ipam := nadConfigIPAM{
				Type:    "whereabouts",
				Range:   "192.168.2.225/28",
				Exclude: []string{"192.168.2.229/30", "192.168.2.236/32"},
			}
			cr := getIPoIBNetwork(testName, testNamespace, testMaster)
			cr.Spec.IPAM = getNADConfigIPAMJSON(ipam)
			err := ts.client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = ipam
			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, testNamespace, "")
		})
		It("Should Render NetworkAttachmentDefinition with default namespace", func() {
			cr := getIPoIBNetwork(testName, "", testMaster)
			err := ts.client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, "default", "")
		})
	})
	Context("Verify Sync flows", func() {
		It("Should recreate NetworkAttachmentDefinition with different namespace", func() {
			cr := getIPoIBNetwork(testName, "", testMaster)
			err := ts.client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, "default", "")

			By("Update network namespace")
			cr = &mellanoxv1alpha1.IPoIBNetwork{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Name: testName}, cr)
			Expect(err).NotTo(HaveOccurred())
			cr.Spec.NetworkNamespace = testNamespace
			err = ts.client.Update(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())

			By("Sync")
			_, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: testName}, nad)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, testNamespace, "")
		})
	})
})

// nolint:unparam
func getIPoIBNetwork(testName, testNamespace, testMaster string) *mellanoxv1alpha1.IPoIBNetwork {
	cr := &mellanoxv1alpha1.IPoIBNetwork{
		Spec: mellanoxv1alpha1.IPoIBNetworkSpec{
			NetworkNamespace: testNamespace,
			Master:           testMaster,
		},
	}
	cr.Name = testName
	return cr
}
