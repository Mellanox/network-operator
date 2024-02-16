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

	"github.com/Mellanox/network-operator/pkg/state"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("NVIPAM Controller", func() {
	ctx := context.Background()

	imageSpec := getTestImageSpec()
	imageSpec = addContainerResources(imageSpec, "nv-ipam-node", "5", "3")
	imageSpec = addContainerResources(imageSpec, "nv-ipam-controller", "5", "3")
	cr := getTestClusterPolicyWithBaseFields()
	cr.Spec.NvIpam = &mellanoxv1alpha1.NVIPAMSpec{
		// TODO(killianmuldoon): Test with the webhook enabled.
		EnableWebhook: false,
		ImageSpec:     *imageSpec,
	}
	catalog := getTestCatalog()
	catalog.Add(state.InfoTypeStaticConfig,
		staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "custom-cni-bin-directory"}))

	_, s, err := state.NewStateNVIPAMCNI(fake.NewClientBuilder().Build(), "../../manifests/state-nv-ipam-cni")
	Expect(err).NotTo(HaveOccurred())
	It("should test that manifests are rendered and fields are set correctly", func() {
		GetManifestObjectsTest(ctx, cr, catalog, imageSpec, s)
	})
})
