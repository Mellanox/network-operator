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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
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

var _ = Describe("NIC Feature Discovery state sync tests", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateNICFeatureDiscovery, "../../manifests/state-nic-feature-discovery")
		Expect(ts).NotTo(BeNil())
	})

	Context("Verify GetWatchSources", func() {
		It("should return DaemonSet watch source", func() {
			ws := ts.state.GetWatchSources()
			Expect(ws).To(HaveLen(1))
			Expect(ws).To(HaveKey("DaemonSet"))
			Expect(ws["DaemonSet"]).To(BeAssignableToTypeOf(&appsv1.DaemonSet{}))
		})
	})

	Context("Verify Sync lifecycle, OpenShift, and error paths", func() {
		It("should fail NewStateNICFeatureDiscovery when manifestDir does not exist", func() {
			_, _, err := state.NewStateNICFeatureDiscovery(ts.client, "/non-existent-manifest-dir")
			Expect(err).To(HaveOccurred())
		})

		It("should create DaemonSet and transition from NotReady to Ready", func() {
			cr := getMinimalNicClusterPolicyWithNFD()

			By("Initial Sync creates DaemonSet and returns SyncStateNotReady")
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			ds := &appsv1.DaemonSet{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "nic-feature-discovery-ds"}, ds)).NotTo(HaveOccurred())
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].Image).To(
				Equal(defaultTestRepository + "/" + defaultTestImage + ":" + defaultTestVersion))

			By("Mark DaemonSet as ready and Sync again")
			ds.Status = appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 1,
			}
			Expect(ts.client.Status().Update(context.Background(), ds)).NotTo(HaveOccurred())

			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should create OpenShift ServiceAccount, Role, RoleBinding, and DaemonSet", func() {
			cr := getMinimalNicClusterPolicyWithNFD()

			status, err := ts.state.Sync(context.Background(), cr, ts.openshiftCatalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			sa := &corev1.ServiceAccount{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "nic-feature-discovery"}, sa)).NotTo(HaveOccurred())

			role := &rbacv1.Role{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "nic-feature-discovery"}, role)).NotTo(HaveOccurred())

			roleBinding := &rbacv1.RoleBinding{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "nic-feature-discovery"}, roleBinding)).NotTo(HaveOccurred())

			ds := &appsv1.DaemonSet{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "nic-feature-discovery-ds"}, ds)).NotTo(HaveOccurred())
		})

		It("should delete objects when NicFeatureDiscovery spec is set to nil", func() {
			cr := getMinimalNicClusterPolicyWithNFD()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			By("Set NicFeatureDiscovery spec to nil and Sync")
			cr.Spec.NicFeatureDiscovery = nil
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "nic-feature-discovery-ds"}, ds)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			By("Sync again when no objects remain")
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateIgnore))
		})

		It("should delete stale objects labeled with state-nic-feature-discovery", func() {
			staleCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stale-nfd-cm",
					Namespace: ts.namespace,
					Labels: map[string]string{
						consts.StateLabel: ts.state.Name(),
					},
				},
			}
			Expect(ts.client.Create(context.Background(), staleCM)).NotTo(HaveOccurred())

			cr := getMinimalNicClusterPolicyWithNFD()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			err = ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "stale-nfd-cm"}, &corev1.ConfigMap{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return SyncStateError when ClusterTypeProvider is missing", func() {
			cr := getMinimalNicClusterPolicyWithNFD()
			emptyCatalog := state.NewInfoCatalog()

			status, err := ts.state.Sync(context.Background(), cr, emptyCatalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should return SyncStateNotReady and error when ImageSpec is invalid", func() {
			cr := getMinimalNicClusterPolicyWithNFD()
			cr.Spec.NicFeatureDiscovery.Repository = ""

			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})

		It("should return error from GetManifestObjects when spec is nil", func() {
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			_, err := ts.renderer.GetManifestObjects(context.Background(), cr, ts.catalog, testLogger)
			Expect(err).To(HaveOccurred())
		})
	})
})

func getMinimalNicClusterPolicyWithNFD() *mellanoxv1alpha1.NicClusterPolicy {
	cr := &mellanoxv1alpha1.NicClusterPolicy{}
	cr.Name = "nic-cluster-policy"
	cr.Spec.NicFeatureDiscovery = &mellanoxv1alpha1.NICFeatureDiscoverySpec{
		ImageSpec: *getTestImageSpec(),
	}
	return cr
}
