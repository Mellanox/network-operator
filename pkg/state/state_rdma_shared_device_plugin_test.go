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

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("RDMA Shared Device Plugin", func() {

	var ctx context.Context
	var rdmaDPState state.State
	var rdmaDPRenderer state.ManifestRenderer
	var testLogger logr.Logger
	var catalog state.InfoCatalog
	var client client.Client
	var manifestDir string

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(netattdefv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir = "../../manifests/state-rdma-shared-device-plugin"
		s, r, err := state.NewStateRDMASharedDevicePlugin(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		rdmaDPState = s
		rdmaDPRenderer = r
		catalog = getTestCatalog()
		testLogger = log.Log.WithName("testLog")
	})

	Context("should render", func() {
		It("Kubernetes manifests", func() {
			cr := getRDMASharedDevicePlugin()
			objs, err := rdmaDPRenderer.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			// We only expect a single manifest here, which should be the DaemonSet.
			// The other manifests are only if RuntimeSpec.IsOpenshift == true.
			Expect(len(objs)).To(Equal(1))
			GetManifestObjectsTest(ctx, cr, catalog, &cr.Spec.RdmaSharedDevicePlugin.ImageSpec, rdmaDPRenderer)
		})
		It("Openshift manifests", func() {
			cr := getRDMASharedDevicePlugin()
			openshiftCatalog := getOpenshiftTestCatalog()
			objs, err := rdmaDPRenderer.GetManifestObjects(ctx, cr, openshiftCatalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			GetManifestObjectsTest(ctx, cr, catalog, &cr.Spec.RdmaSharedDevicePlugin.ImageSpec, rdmaDPRenderer)
		})
	})
	Context("should sync", func() {
		It("wihtout any errors", func() {
			cr := getRDMASharedDevicePlugin()
			err := client.Create(ctx, cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := rdmaDPState.Sync(ctx, cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			// We do not expect that the sync state (i.e. the DaemonSet) will be ready.
			// There is no real Kubernetes cluster in the unit tests and thus the Pods cannot be scheduled.
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})
	})
})

func getRDMASharedDevicePlugin() *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()
	imageSpec := getTestImageSpec()
	cr.Name = "nic-cluster-policy"
	cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{
		ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
			ImageSpec: *imageSpec,
		},
	}
	cr.Spec.RdmaSharedDevicePlugin.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{
		{
			Name: "rdma-shared-dp",
			Requests: v1.ResourceList{
				"cpu":    resource.MustParse("150m"),
				"memory": resource.MustParse("150Mi"),
			},
			Limits: v1.ResourceList{
				"cpu":    resource.MustParse("150m"),
				"memory": resource.MustParse("200Mi"),
			},
		},
	}
	return cr
}
