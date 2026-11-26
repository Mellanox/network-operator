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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("IPoIB CNI State tests", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateIPoIBCNI, "../../manifests/state-ipoib-cni")
		Expect(ts).NotTo(BeNil())

		ts.catalog.Add(state.InfoTypeStaticConfig,
			staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "custom-cni-bin-directory"}))
	})

	Context("should render", func() {
		It("manifests with IPoIB CNI", func() {
			cr := getNICForIPoIBCNI()
			objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))
			assertNodeCriticalPriorityForRenderedDS(objs, "kube-ipoib-cni-ds")
			GetManifestObjectsTest(ts.context, cr, ts.catalog, cr.Spec.SecondaryNetwork.IPoIB, ts.renderer)
		})
		It("manifests with IPoIB CNI - image as SHA256", func() {
			cr := getNICForIPoIBCNI()
			cr.Spec.SecondaryNetwork.IPoIB.Version = defaultTestVersionSha256
			objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, cr.Spec.SecondaryNetwork.IPoIB, ts.renderer)
		})
		It("manifests with IPoIB CNI for Openshift", func() {
			cr := getNICForIPoIBCNI()
			objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.openshiftCatalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			// Validate the 4 rendered OpenShift objects directly: GetManifestObjectsTest hardcodes
			// custom-cni-bin-directory (which ts.openshiftCatalog does not set) and ignores RBAC/SA kinds.
			assertUnstructuredListHasExactKinds(objs, "ServiceAccount", "Role", "RoleBinding", "DaemonSet")
			for _, obj := range objs {
				Expect(obj.GetNamespace()).To(Equal(ts.namespace))
			}
		})
	})

	Context("Verify GetWatchSources", func() {
		It("should return DaemonSet watch source", func() {
			ws := ts.state.GetWatchSources()
			Expect(ws).To(HaveLen(1))
			Expect(ws).To(HaveKey("DaemonSet"))
			Expect(ws["DaemonSet"]).To(BeAssignableToTypeOf(&appsv1.DaemonSet{}))
		})
	})

	Context("should sync", func() {
		It("without any errors and transition to Ready", func() {
			cr := getNICForIPoIBCNI()
			err := ts.client.Create(ts.context, cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			// We do not expect that the sync state (i.e. the DaemonSet) will be ready.
			// There is no real Kubernetes cluster in the unit tests and thus the Pods cannot be scheduled.
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			ds := &appsv1.DaemonSet{}
			Expect(ts.client.Get(ts.context,
				types.NamespacedName{Namespace: ts.namespace, Name: "kube-ipoib-cni-ds"}, ds)).NotTo(HaveOccurred())

			ds.Status = appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 1,
			}
			Expect(ts.client.Status().Update(ts.context, ds)).NotTo(HaveOccurred())

			status, err = ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should fail NewStateIPoIBCNI when manifestDir does not exist", func() {
			_, _, err := state.NewStateIPoIBCNI(ts.client, "/non-existent-manifest-dir")
			Expect(err).To(HaveOccurred())
		})

		It("should delete objects when IPoIB spec is set to nil", func() {
			cr := getNICForIPoIBCNI()
			status, err := ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			cr.Spec.SecondaryNetwork.IPoIB = nil
			status, err = ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(ts.context,
				types.NamespacedName{Namespace: ts.namespace, Name: "kube-ipoib-cni-ds"}, ds)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			status, err = ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateIgnore))
		})

		It("should delete stale objects labeled with state-ipoib-cni", func() {
			staleCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stale-ipoib-cm",
					Namespace: ts.namespace,
					Labels: map[string]string{
						consts.StateLabel: ts.state.Name(),
					},
				},
			}
			Expect(ts.client.Create(ts.context, staleCM)).NotTo(HaveOccurred())

			cr := getNICForIPoIBCNI()
			status, err := ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			err = ts.client.Get(ts.context,
				types.NamespacedName{Namespace: ts.namespace, Name: "stale-ipoib-cm"}, &corev1.ConfigMap{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return SyncStateError when StaticConfigProvider is missing", func() {
			cr := getNICForIPoIBCNI()
			emptyCatalog := state.NewInfoCatalog()

			status, err := ts.state.Sync(ts.context, cr, emptyCatalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should return SyncStateError when ClusterTypeProvider is missing", func() {
			cr := getNICForIPoIBCNI()
			catalogNoCluster := state.NewInfoCatalog()
			catalogNoCluster.Add(state.InfoTypeStaticConfig, ts.catalog.GetStaticConfigProvider())

			status, err := ts.state.Sync(ts.context, cr, catalogNoCluster)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should return SyncStateNotReady and error when ImageSpec is invalid", func() {
			cr := getNICForIPoIBCNI()
			cr.Spec.SecondaryNetwork.IPoIB.Repository = ""

			status, err := ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})

		It("should return error from GetManifestObjects when spec is nil", func() {
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			_, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).To(HaveOccurred())
		})
	})
})

func getNICForIPoIBCNI() *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()
	imageSpec := addContainerResources(getTestImageSpec(), "ipoib-cni", "1", "9")
	cr.Name = "ipoib-cni-nic-cluster-policy"
	cr.Spec.SecondaryNetwork = &mellanoxv1alpha1.SecondaryNetworkSpec{
		IPoIB: imageSpec,
	}
	return cr
}
