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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

var _ = Describe("ManifestRenderer tests", func() {
	It("should parse container names from Deployment manifests", func() {
		_, renderer, err := NewStateIBKubernetes(fake.NewClientBuilder().Build(), "../../manifests/state-ib-kubernetes")
		Expect(err).NotTo(HaveOccurred())

		cr := &mellanoxv1alpha1.NicClusterPolicy{
			Spec: mellanoxv1alpha1.NicClusterPolicySpec{
				IBKubernetes: &mellanoxv1alpha1.IBKubernetesSpec{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image:      "ib-kubernetes",
						Repository: "nvcr.io/nvidia/mellanox",
						Version:    "v1.0.0",
					},
				},
			},
		}
		names, err := ParseContainerNames(renderer, cr, testLogger)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(Equal([]string{"ib-kubernetes"}))
	})

	It("should return error when GetManifestObjects fails", func() {
		_, renderer, err := NewStateIBKubernetes(fake.NewClientBuilder().Build(), "../../manifests/state-ib-kubernetes")
		Expect(err).NotTo(HaveOccurred())

		_, err = ParseContainerNames(renderer, &mellanoxv1alpha1.NicClusterPolicy{}, testLogger)
		Expect(err).To(HaveOccurred())
	})
})
