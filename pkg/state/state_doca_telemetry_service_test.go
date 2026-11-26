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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/state"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("DOCATelemetryService Controller", func() {
	ctx := context.Background()

	imageSpec := addContainerResources(getTestImageSpec(), "doca-telemetry-service", "1", "9")
	cr := getTestClusterPolicyWithBaseFields()
	cr.Spec.DOCATelemetryService = &mellanoxv1alpha1.DOCATelemetryServiceSpec{ImageSpec: *imageSpec}
	_, s, err := state.NewStateDOCATelemetryService(fake.NewClientBuilder().Build(),
		"../../manifests/state-doca-telemetry-service")
	Expect(err).ToNot(HaveOccurred())

	It("should test the configmap is rendered with the default name", func() {
		defaultConfigMapName := "doca-telemetry-service"
		got, err := s.GetManifestObjects(ctx, cr, getTestCatalog(), log.FromContext(ctx))
		Expect(err).ToNot(HaveOccurred())
		expectedKinds := []string{"ServiceAccount", "DaemonSet", "ConfigMap"}
		assertUnstructuredListHasExactKinds(got, expectedKinds...)

		for _, obj := range got {
			// The configMap should be rendered with the default name.
			if obj.GetKind() == "ConfigMap" {
				Expect(obj.GetName()).To(Equal("doca-telemetry-service"))
			}
			// The configMap volume should point to the default value in the NICClusterPolicy.
			if obj.GetKind() == "DaemonSet" {
				ds := &appsv1.DaemonSet{}
				Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), ds)).To(Succeed())
				for _, v := range ds.Spec.Template.Spec.Volumes {
					if v.Name == "doca-telemetry-service-configmap" {
						Expect(v.ConfigMap).To(Not(BeNil()))
						Expect(v.ConfigMap.Name).To(Equal(defaultConfigMapName))
					}
				}
			}

		}
	})
	It("should test configmap not rendered if nicClusterPolicy `config.fromConfigMap` is set", func() {
		customConfigMapName := "custom-cm-name"
		withConfig := cr.DeepCopy()
		withConfig.Spec.DOCATelemetryService.Config = &mellanoxv1alpha1.DOCATelemetryServiceConfig{
			FromConfigMap: customConfigMapName,
		}
		got, err := s.GetManifestObjects(ctx, withConfig, getTestCatalog(), log.FromContext(ctx))
		Expect(err).ToNot(HaveOccurred())
		expectedKinds := []string{"ServiceAccount", "DaemonSet"}
		assertUnstructuredListHasExactKinds(got, expectedKinds...)

		for _, obj := range got {
			// The configMap should not be rendered.
			if obj.GetKind() == "ConfigMap" {
				Fail("configMap should not be rendered if `config.fromConfigMap` is set")
			}
			// The configMap volume should point to the custom value in the NICClusterPolicy.
			if obj.GetKind() == "DaemonSet" {
				ds := &appsv1.DaemonSet{}
				Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), ds)).To(Succeed())
				for _, v := range ds.Spec.Template.Spec.Volumes {
					if v.Name == "doca-telemetry-service-configmap" {
						Expect(v.ConfigMap).To(Not(BeNil()))
						Expect(v.ConfigMap.Name).To(Equal(customConfigMapName))
					}
				}
			}
		}
	})
	It("should confine the IPC shared memory to the telemetry directory by default", func() {
		got, err := s.GetManifestObjects(ctx, cr, getTestCatalog(), log.FromContext(ctx))
		Expect(err).ToNot(HaveOccurred())
		Expect(getIPCSharedMemoryHostPath(got)).To(Equal("/dev/shm/telemetry"))
	})
	It("should share the host shared memory when `shareHostSharedMemory` is set", func() {
		withHostShm := cr.DeepCopy()
		withHostShm.Spec.DOCATelemetryService.ShareHostSharedMemory = true
		got, err := s.GetManifestObjects(ctx, withHostShm, getTestCatalog(), log.FromContext(ctx))
		Expect(err).ToNot(HaveOccurred())
		Expect(getIPCSharedMemoryHostPath(got)).To(Equal("/dev/shm"))
	})
	It("should mount the host fwctl device directory", func() {
		got, err := s.GetManifestObjects(ctx, cr, getTestCatalog(), log.FromContext(ctx))
		Expect(err).ToNot(HaveOccurred())

		foundDaemonSet := false
		for _, obj := range got {
			if obj.GetKind() != "DaemonSet" {
				continue
			}
			foundDaemonSet = true
			ds := &appsv1.DaemonSet{}
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), ds)).To(Succeed())

			container := ds.Spec.Template.Spec.Containers[0]
			Expect(container.VolumeMounts).To(ContainElement(And(
				HaveField("Name", "fwctl-devices"),
				HaveField("MountPath", "/dev/fwctl"),
				HaveField("ReadOnly", false),
			)))

			pathType := corev1.HostPathDirectoryOrCreate
			Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(And(
				HaveField("Name", "fwctl-devices"),
				HaveField("HostPath.Path", "/dev/fwctl"),
				HaveField("HostPath.Type", &pathType),
			)))
		}
		Expect(foundDaemonSet).To(BeTrue())
	})
	It("should test OpenShift specific role and rolebinding rendered when the cluster is OpenShift", func() {
		withConfig := cr.DeepCopy()
		got, err := s.GetManifestObjects(ctx, withConfig, getTestCatalogForOpenshift(true), log.FromContext(ctx))
		Expect(err).ToNot(HaveOccurred())
		expectedKinds := []string{"ServiceAccount", "DaemonSet", "ConfigMap", "Role", "RoleBinding"}
		assertUnstructuredListHasExactKinds(got, expectedKinds...)
	})
	It("should use SHA256 format", func() {
		withSha256 := cr.DeepCopy()
		withSha256.Spec.DOCATelemetryService.Version = defaultTestVersionSha256
		got, err := s.GetManifestObjects(ctx, withSha256, getTestCatalog(), log.FromContext(ctx))
		Expect(err).ToNot(HaveOccurred())
		expectedKinds := []string{"ServiceAccount", "DaemonSet", "ConfigMap"}
		assertUnstructuredListHasExactKinds(got, expectedKinds...)

		for _, obj := range got {
			// The image should be in SHA256 format
			if obj.GetKind() == "DaemonSet" {
				ds := &appsv1.DaemonSet{}
				Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), ds)).To(Succeed())
				Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(getImagePathWithSha256()))
			}

		}
	})
})

// getIPCSharedMemoryHostPath returns the host path backing the /dev/shm mount of the DTS container,
// failing if the mount and the volume it refers to do not line up.
func getIPCSharedMemoryHostPath(objects []*unstructured.Unstructured) string {
	for _, obj := range objects {
		if obj.GetKind() != "DaemonSet" {
			continue
		}
		ds := &appsv1.DaemonSet{}
		Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), ds)).To(Succeed())
		podSpec := ds.Spec.Template.Spec

		var volumeName string
		for _, m := range podSpec.Containers[0].VolumeMounts {
			if m.MountPath == "/dev/shm" {
				volumeName = m.Name
			}
		}
		Expect(volumeName).ToNot(BeEmpty(), "no volume is mounted at /dev/shm")

		for i := range podSpec.Volumes {
			if podSpec.Volumes[i].Name == volumeName {
				Expect(podSpec.Volumes[i].HostPath).ToNot(BeNil())
				return podSpec.Volumes[i].HostPath.Path
			}
		}
		Fail("the volume mounted at /dev/shm is not declared")
	}
	Fail("no DaemonSet was rendered")
	return ""
}

var _ = Describe("DOCATelemetryService state sync tests", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateDOCATelemetryService, "../../manifests/state-doca-telemetry-service")
		Expect(ts).NotTo(BeNil())
	})

	Context("Verify metadata and GetWatchSources", func() {
		It("should return expected Name, Description, and GetWatchSources", func() {
			Expect(ts.state.Name()).To(Equal("state-doca-telemetry-service"))
			Expect(ts.state.Description()).To(Equal("DOCA Telemetry Service deployed in the cluster"))

			ws := ts.state.GetWatchSources()
			Expect(ws).To(HaveLen(1))
			Expect(ws).To(HaveKey("DaemonSet"))
			Expect(ws["DaemonSet"]).To(BeAssignableToTypeOf(&appsv1.DaemonSet{}))
		})
	})

	Context("Verify Sync lifecycle and error paths", func() {
		It("should fail NewStateDOCATelemetryService when manifestDir does not exist", func() {
			_, _, err := state.NewStateDOCATelemetryService(ts.client, "/non-existent-manifest-dir")
			Expect(err).To(HaveOccurred())
		})

		It("should create objects and transition from NotReady to Ready", func() {
			cr := getMinimalNicClusterPolicyWithDTS()

			By("Initial Sync creates ServiceAccount, ConfigMap, and DaemonSet")
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			sa := &corev1.ServiceAccount{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "doca-telemetry-service"}, sa)).NotTo(HaveOccurred())

			cm := &corev1.ConfigMap{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "doca-telemetry-service"}, cm)).NotTo(HaveOccurred())

			ds := &appsv1.DaemonSet{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "doca-telemetry-service"}, ds)).NotTo(HaveOccurred())

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

		It("should delete objects when DOCATelemetryService spec is set to nil", func() {
			cr := getMinimalNicClusterPolicyWithDTS()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			By("Set DOCATelemetryService spec to nil and Sync")
			cr.Spec.DOCATelemetryService = nil
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "doca-telemetry-service"}, ds)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			By("Sync again when no objects remain")
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateIgnore))
		})

		It("should delete stale objects labeled with state-doca-telemetry-service", func() {
			staleCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stale-dts-cm",
					Namespace: ts.namespace,
					Labels: map[string]string{
						consts.StateLabel: ts.state.Name(),
					},
				},
			}
			Expect(ts.client.Create(context.Background(), staleCM)).NotTo(HaveOccurred())

			cr := getMinimalNicClusterPolicyWithDTS()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			err = ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "stale-dts-cm"}, &corev1.ConfigMap{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return SyncStateNotReady and error when ClusterTypeProvider is missing", func() {
			cr := getMinimalNicClusterPolicyWithDTS()
			emptyCatalog := state.NewInfoCatalog()

			status, err := ts.state.Sync(context.Background(), cr, emptyCatalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})

		It("should return SyncStateNotReady and error when ImageSpec is invalid", func() {
			cr := getMinimalNicClusterPolicyWithDTS()
			cr.Spec.DOCATelemetryService.Repository = ""

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

func getMinimalNicClusterPolicyWithDTS() *mellanoxv1alpha1.NicClusterPolicy {
	cr := &mellanoxv1alpha1.NicClusterPolicy{}
	cr.Name = "nic-cluster-policy"
	cr.Spec.DOCATelemetryService = &mellanoxv1alpha1.DOCATelemetryServiceSpec{
		ImageSpec: *getTestImageSpec(),
	}
	return cr
}
