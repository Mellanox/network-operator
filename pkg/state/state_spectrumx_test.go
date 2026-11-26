/*
2025 NVIDIA CORPORATION & AFFILIATES

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

//nolint:dupl
var _ = Describe("Spectrum X Operator", func() {
	var (
		ts      testScope
		catalog state.InfoCatalog
	)
	BeforeEach(func() {
		ts = ts.New(state.NewStateSpectrumXOperator, "../../manifests/state-spectrum-x-operator")
		Expect(ts).NotTo(BeNil())
		catalog = getTestCatalog()
		catalog.Add(state.InfoTypeStaticConfig,
			staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "custom-cni-bin-directory"}))
	})

	Context("should render", func() {
		It("the flowcontroller pod identity for xplane toleration inheritance", func() {
			cr := getSpectrumXOperator()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			var ds appsv1.DaemonSet
			for _, obj := range objs {
				if obj.GetKind() == "DaemonSet" {
					Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)).To(Succeed())
				}
			}
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			for name, fieldPath := range map[string]string{"POD_NAME": "metadata.name", "POD_NAMESPACE": "metadata.namespace"} {
				Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
					Name: name,
					ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fieldPath,
					}},
				}))
			}
		})
		It("permissions to watch the parent and reconcile the xplane DaemonSet", func() {
			cr := getSpectrumXOperator()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			var role rbacv1.ClusterRole
			for _, obj := range objs {
				if obj.GetKind() == "ClusterRole" {
					Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &role)).To(Succeed())
				}
			}
			Expect(role.Rules).To(ContainElement(rbacv1.PolicyRule{
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets"},
				Verbs:     []string{"create", "get", "list", "update", "watch"},
			}))
		})
		It("Kubernetes manifests", func() {
			cr := getSpectrumXOperator()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(5))
			assertNodeCriticalPriorityForRenderedDS(objs, "spectrum-x-flowcontroller")
			GetManifestObjectsTest(ts.context, cr, catalog, &cr.Spec.SpectrumXOperator.ImageSpec, ts.renderer)
		})
		It("Openshift manifests", func() {
			cr := getSpectrumXOperator()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.openshiftCatalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(7))
			GetManifestObjectsTest(ts.context, cr, ts.openshiftCatalog, &cr.Spec.SpectrumXOperator.ImageSpec, ts.renderer)
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
			cr := getSpectrumXOperator()
			err := ts.client.Create(ts.context, cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(ts.context, cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			// We do not expect that the sync state (i.e. the DaemonSet) will be ready.
			// There is no real Kubernetes cluster in the unit tests and thus the Pods cannot be scheduled.
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			ds := &appsv1.DaemonSet{}
			Expect(ts.client.Get(ts.context,
				types.NamespacedName{Namespace: ts.namespace, Name: "spectrum-x-flowcontroller"}, ds)).NotTo(HaveOccurred())

			ds.Status = appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 1,
			}
			Expect(ts.client.Status().Update(ts.context, ds)).NotTo(HaveOccurred())

			status, err = ts.state.Sync(ts.context, cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should fail NewStateSpectrumXOperator when manifestDir does not exist", func() {
			_, _, err := state.NewStateSpectrumXOperator(ts.client, "/non-existent-manifest-dir")
			Expect(err).To(HaveOccurred())
		})

		It("should delete objects when SpectrumXOperator spec is set to nil", func() {
			cr := getSpectrumXOperator()
			status, err := ts.state.Sync(ts.context, cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			cr.Spec.SpectrumXOperator = nil
			status, err = ts.state.Sync(ts.context, cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(ts.context,
				types.NamespacedName{Namespace: ts.namespace, Name: "spectrum-x-flowcontroller"}, ds)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			status, err = ts.state.Sync(ts.context, cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateIgnore))
		})

		It("should delete stale objects labeled with state-spectrum-x-operator", func() {
			staleCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stale-spectrumx-cm",
					Namespace: ts.namespace,
					Labels: map[string]string{
						consts.StateLabel: ts.state.Name(),
					},
				},
			}
			Expect(ts.client.Create(ts.context, staleCM)).NotTo(HaveOccurred())

			cr := getSpectrumXOperator()
			status, err := ts.state.Sync(ts.context, cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			err = ts.client.Get(ts.context,
				types.NamespacedName{Namespace: ts.namespace, Name: "stale-spectrumx-cm"}, &corev1.ConfigMap{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return SyncStateError when ClusterTypeProvider is missing", func() {
			cr := getSpectrumXOperator()
			emptyCatalog := state.NewInfoCatalog()

			status, err := ts.state.Sync(ts.context, cr, emptyCatalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should return SyncStateNotReady and error when StaticConfigProvider is missing", func() {
			cr := getSpectrumXOperator()
			catalogNoStatic := state.NewInfoCatalog()
			catalogNoStatic.Add(state.InfoTypeClusterType, catalog.GetClusterTypeProvider())

			status, err := ts.state.Sync(ts.context, cr, catalogNoStatic)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})

		It("should return SyncStateNotReady and error when ImageSpec is invalid", func() {
			cr := getSpectrumXOperator()
			cr.Spec.SpectrumXOperator.Repository = ""

			status, err := ts.state.Sync(ts.context, cr, catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})

		It("should return error from GetManifestObjects when spec is nil", func() {
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			_, err := ts.renderer.GetManifestObjects(ts.context, cr, catalog, testLogger)
			Expect(err).To(HaveOccurred())
		})
	})
})

func getSpectrumXOperator() *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()
	imageSpec := getTestImageSpec()
	imageSpec = addContainerResources(imageSpec, "spectrum-x-flowcontroller", "1", "9")
	cr.Name = "nic-cluster-policy"
	cr.Spec.SpectrumXOperator = &mellanoxv1alpha1.SpectrumXOperatorSpec{
		ImageSpec: *imageSpec,
	}
	return cr
}
