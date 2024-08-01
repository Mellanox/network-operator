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

package state_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("HostDevice Network State rendering tests", func() {
	const (
		testName      = "host-device"
		testNamespace = "hostdevice"
		testType      = "host-device"
	)

	var (
		ts                testScope
		expectedNadConfig nadConfig
	)

	BeforeEach(func() {
		ts = ts.New(state.NewStateHostDeviceNetwork, "../../manifests/state-hostdevice-network")
		Expect(ts).NotTo(BeNil())
		expectedNadConfig = defaultNADConfig(&nadConfig{
			Name: testName,
			Type: testType,
			IPAM: nadConfigIPAM{},
		})
	})

	Context("HostDevice Network State", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			testResourceName := "test"
			cr := getHostDeviceNetwork(testName, testNamespace, testResourceName)
			err := ts.client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, testNamespace, testResourceName)
		})
		// We should be able to create the HostDeviceNetwork with a prefixed resource name,
		// but the CR should be mutated to NOT have the prefix.
		// The annotations resource name MUST have to prefix anyway.
		It("Should Render NetworkAttachmentDefinition with resource with prefix", func() {
			testResourceName := hostDeviceNetworkResourceNamePrefix + testName
			cr := getHostDeviceNetwork(testName, testNamespace, testResourceName)
			err := ts.client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = nadConfigIPAM{}
			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, testNamespace, testName)
		})
		It("Should Render NetworkAttachmentDefinition with IPAM", func() {
			ipam := nadConfigIPAM{
				Type:    "whereabouts",
				Range:   "192.168.2.225/28",
				Exclude: []string{"192.168.2.229/30", "192.168.2.236/32"},
			}
			testResourceName := "test"
			cr := getHostDeviceNetwork(testName, testNamespace, testResourceName)
			cr.Spec.IPAM = getNADConfigIPAMJSON(ipam)
			err := ts.client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			expectedNadConfig.IPAM = ipam
			assertNetworkAttachmentDefinition(ts.client, &expectedNadConfig, testName, testNamespace, testResourceName)
		})
	})
})

func getHostDeviceNetwork(testName, testNamespace, resourceName string) *mellanoxv1alpha1.HostDeviceNetwork {
	cr := &mellanoxv1alpha1.HostDeviceNetwork{
		Spec: mellanoxv1alpha1.HostDeviceNetworkSpec{
			NetworkNamespace: testNamespace,
			ResourceName:     resourceName,
		},
	}
	cr.Name = testName
	return cr
}
