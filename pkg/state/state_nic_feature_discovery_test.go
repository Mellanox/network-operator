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

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("NicClusterPolicyReconciler Controller", func() {
	ctx := context.Background()

	imageSpec := addContainerResources(getTestImageSpec(), "nic-feature-discovery", "5", "3")
	cr := getTestClusterPolicyWithBaseFields()
	cr.Spec.NicFeatureDiscovery = &mellanoxv1alpha1.NICFeatureDiscoverySpec{ImageSpec: *imageSpec}
	_, s, err := state.NewStateNICFeatureDiscovery(fake.NewClientBuilder().Build(),
		"../../manifests/state-nic-feature-discovery")
	Expect(err).ToNot(HaveOccurred())

	It("should test fields are set correctly", func() {
		GetManifestObjectsTest(ctx, cr, getTestCatalog(), imageSpec, s)
	})
	It("should use SHA256 format", func() {
		withSha256 := cr.DeepCopy()
		withSha256.Spec.NicFeatureDiscovery.Version = defaultTestVersionSha256
		imageSpecWithSha256 := imageSpec.DeepCopy()
		imageSpecWithSha256.Version = defaultTestVersionSha256
		GetManifestObjectsTest(ctx, withSha256, getTestCatalog(), imageSpecWithSha256, s)
	})
})
