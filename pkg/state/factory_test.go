/*
2026 NVIDIA CORPORATION & AFFILIATES

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
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

var _ = Describe("State Factory tests", func() {
	var origManifestBaseDir string

	BeforeEach(func() {
		origManifestBaseDir = envConfig.State.ManifestBaseDir
		envConfig.State.ManifestBaseDir = "../../manifests"
		DeferCleanup(func() {
			envConfig.State.ManifestBaseDir = origManifestBaseDir
		})
	})

	It("should create Manager for NicClusterPolicy", func() {
		mgr, err := NewManager(mellanoxv1alpha1.NicClusterPolicyCRDName, fake.NewClientBuilder().Build(), logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr).NotTo(BeNil())
		Expect(mgr.(*stateManager).states).To(HaveLen(12))
	})

	It("should create Manager for NicNodePolicy", func() {
		mgr, err := NewManager(mellanoxv1alpha1.NicNodePolicyCRDName, fake.NewClientBuilder().Build(), logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr).NotTo(BeNil())
		Expect(mgr.(*stateManager).states).To(HaveLen(3))
	})

	It("should create Manager for MacvlanNetwork, HostDeviceNetwork, and IPoIBNetwork", func() {
		for _, crd := range []string{
			mellanoxv1alpha1.MacvlanNetworkCRDName,
			mellanoxv1alpha1.HostDeviceNetworkCRDName,
			mellanoxv1alpha1.IPoIBNetworkCRDName,
		} {
			mgr, err := NewManager(crd, fake.NewClientBuilder().Build(), logr.Discard())
			Expect(err).NotTo(HaveOccurred())
			Expect(mgr).NotTo(BeNil())
			Expect(mgr.(*stateManager).states).To(HaveLen(1))
		}
	})

	It("should fail for unsupported CRD kind", func() {
		_, err := NewManager("UnsupportedCRD", fake.NewClientBuilder().Build(), logr.Discard())
		Expect(err).To(HaveOccurred())
	})

	It("should fail when ManifestBaseDir does not exist", func() {
		envConfig.State.ManifestBaseDir = "/non-existent-manifests"
		_, err := NewManager(mellanoxv1alpha1.NicClusterPolicyCRDName, fake.NewClientBuilder().Build(), logr.Discard())
		Expect(err).To(HaveOccurred())
	})
})
