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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const resourceNamePrefix = "nvidia.com/"

var _ = Describe("HostDevice Network State rendering tests", func() {
	const (
		testNamespace = "namespace"
		testType      = "host-device"
	)

	var hostDeviceNetState state.State
	var catalog state.InfoCatalog
	var client client.Client

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(netattdefv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir := "../../manifests/state-hostdevice-network"
		s, err := state.NewStateHostDeviceNetwork(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		hostDeviceNetState = s
		catalog = getTestCatalog()
	})

	Context("HostDevice Network State", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			testName := "host-device"
			testResourceName := "test"
			cr := getHostDeviceNetwork(testName, testNamespace, testResourceName)
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := hostDeviceNetState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			nadConfig, err := getNADConfig(nad.Spec.Config)
			Expect(err).NotTo(HaveOccurred())
			Expect(nadConfig.Type).To(Equal("host-device"))
			expectedNad := getExpectedHostDeviceNetNAD(testName, "{}")
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal(testNamespace))
			Expect(err).NotTo(HaveOccurred())
			resourceName, ok := nad.Annotations["k8s.v1.cni.cncf.io/resourceName"]
			Expect(ok).To(BeTrue())
			Expect(resourceName).To(Equal(resourceNamePrefix + testResourceName))
		})
		It("Should Render NetworkAttachmentDefinition with resource with prefix", func() {
			testName := "host-device"
			testResourceName := resourceNamePrefix + "test"
			cr := getHostDeviceNetwork(testName, testNamespace, testResourceName)
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := hostDeviceNetState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			nadConfig, err := getNADConfig(nad.Spec.Config)
			Expect(err).NotTo(HaveOccurred())
			Expect(nadConfig.Type).To(Equal("host-device"))
			expectedNad := getExpectedHostDeviceNetNAD(testName, "{}")
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal(testNamespace))
			Expect(err).NotTo(HaveOccurred())
			resourceName, ok := nad.Annotations["k8s.v1.cni.cncf.io/resourceName"]
			Expect(ok).To(BeTrue())
			Expect(resourceName).To(Equal(testResourceName))
		})
		It("Should Render NetworkAttachmentDefinition with IPAM", func() {
			ipam := "{\"type\":\"whereabouts\",\"range\":\"192.168.2.225/28\"," +
				"\"exclude\":[\"192.168.2.229/30\",\"192.168.2.236/32\"]}"
			testName := "host-device"
			testResourceName := "test"
			cr := getHostDeviceNetwork(testName, testNamespace, testResourceName)
			cr.Spec.IPAM = ipam
			err := client.Create(context.Background(), cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := hostDeviceNetState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			By("Verify NetworkAttachmentDefinition")
			nad := &netattdefv1.NetworkAttachmentDefinition{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: testName}, nad)
			Expect(err).NotTo(HaveOccurred())
			nadConfig, err := getNADConfig(nad.Spec.Config)
			Expect(err).NotTo(HaveOccurred())
			Expect(nadConfig.Type).To(Equal("host-device"))
			Expect(nad.Name).To(Equal(testName))
			Expect(nad.Namespace).To(Equal(testNamespace))

			expectedNad := getExpectedHostDeviceNetNAD(testName, ipam)
			Expect(nad.Spec).To(BeEquivalentTo(expectedNad.Spec))
		})
	})
})

func getExpectedHostDeviceNetNAD(testName, ipam string) *netattdefv1.NetworkAttachmentDefinition {
	nad := &netattdefv1.NetworkAttachmentDefinition{}
	cfg := fmt.Sprintf("{ \"cniVersion\":\"0.3.1\", \"name\":%q, \"type\":\"host-device\", "+
		"\"ipam\":%s }",
		testName, ipam)
	nad.Spec.Config = cfg
	return nad
}

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
