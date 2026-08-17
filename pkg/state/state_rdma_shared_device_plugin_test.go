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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("RDMA Shared Device Plugin", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateRDMASharedDevicePlugin, "../../manifests/state-rdma-shared-device-plugin")
		Expect(ts).NotTo(BeNil())
	})

	Context("should render", func() {
		It("Kubernetes manifests", func() {
			cr := getRDMASharedDevicePlugin()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			// We only expect a single manifest here, which should be the DaemonSet.
			// The other manifests are only if RuntimeSpec.IsOpenshift == true.
			Expect(len(objs)).To(Equal(1))
			Expect(objs[0].GetName()).To(Equal("rdma-shared-dp-ds"))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, &cr.Spec.RdmaSharedDevicePlugin.ImageSpec, ts.renderer)
		})
		It("manifests with RdmaSharedDevicePlugin - image as SHA256", func() {
			cr := getRDMASharedDevicePlugin()
			cr.Spec.RdmaSharedDevicePlugin.Version = defaultTestVersionSha256
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(1))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, &cr.Spec.RdmaSharedDevicePlugin.ImageSpec, ts.renderer)
		})
		It("Openshift manifests", func() {
			cr := getRDMASharedDevicePlugin()
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.openshiftCatalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			GetManifestObjectsTest(ts.context, cr, ts.catalog, &cr.Spec.RdmaSharedDevicePlugin.ImageSpec, ts.renderer)
		})
	})
	Context("When rendering a NicNodePolicy", func() {
		DescribeTable("should use a deterministic policy name hash only in the DaemonSet name",
			func(policyName, expectedHash string) {
				expectedName := "rdma-shared-dp-ds-" + expectedHash
				ds := renderRdmaSharedDevicePluginDaemonSet(
					&ts, getMinimalNicNodePolicyWithRdmaSharedDevicePlugin(policyName))

				Expect(ds.Name).To(Equal(expectedName))
				Expect(len(ds.Name)).To(BeNumerically("<=", validation.DNS1123LabelMaxLength))
				Expect(validation.IsDNS1123Subdomain(ds.Name)).To(BeEmpty())
				Expect(ds.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", "rdma-shared-dp-"+policyName))
				Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("app", "rdma-shared-dp-"+policyName))
				Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(And(
					HaveField("Name", "config"),
					HaveField("ConfigMap.Name", "rdma-devices-"+policyName),
				)))
			},
			Entry("for a readable policy name", "pool-a", "779d97f8c7"),
			Entry("for the maximum admitted policy name length", strings.Repeat("a", 30), "84bb999775"),
		)
	})
	Context("should sync", func() {
		It("without any errors", func() {
			cr := getRDMASharedDevicePlugin()
			err := ts.client.Create(ts.context, cr)
			Expect(err).NotTo(HaveOccurred())
			status, err := ts.state.Sync(ts.context, cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			// We do not expect that the sync state (i.e. the DaemonSet) will be ready.
			// There is no real Kubernetes cluster in the unit tests and thus the Pods cannot be scheduled.
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})
	})
	Context("Global config", func() {
		It("should use global repository and version when component values are empty", func() {
			cr := getTestClusterPolicyWithBaseFields()
			cr.Name = "nic-cluster-policy"
			// Set global config
			cr.Spec.Global = &mellanoxv1alpha1.GlobalConfig{
				Repository: "global-repo",
				Version:    "global-version",
			}
			// Component has only image, no repository/version
			cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image: "test-image",
					},
				},
			}
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(BeNumerically(">=", 1))
			// Verify the image uses global values
			ds := objs[0]
			Expect(ds.GetKind()).To(Equal("DaemonSet"))
			containers := getContainers(ds)
			Expect(containers[0].(map[string]interface{})["image"]).To(Equal("global-repo/test-image:global-version"))
		})
		It("should use component values when both global and component values exist", func() {
			cr := getTestClusterPolicyWithBaseFields()
			cr.Name = "nic-cluster-policy"
			// Set global config
			cr.Spec.Global = &mellanoxv1alpha1.GlobalConfig{
				Repository: "global-repo",
				Version:    "global-version",
			}
			// Component has its own repository and version
			cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image:      "test-image",
						Repository: "component-repo",
						Version:    "component-version",
					},
				},
			}
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(BeNumerically(">=", 1))
			// Verify the image uses component values (not global)
			ds := objs[0]
			Expect(ds.GetKind()).To(Equal("DaemonSet"))
			containers := getContainers(ds)
			Expect(containers[0].(map[string]interface{})["image"]).To(Equal("component-repo/test-image:component-version"))
		})
		It("should return error when repository is missing and no global config", func() {
			cr := getTestClusterPolicyWithBaseFields()
			cr.Name = "nic-cluster-policy"
			// No global config, component missing repository
			cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image:   "test-image",
						Version: "v1.0.0",
					},
				},
			}
			_, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("repository is required"))
		})
		It("should return error when version is missing and no global config", func() {
			cr := getTestClusterPolicyWithBaseFields()
			cr.Name = "nic-cluster-policy"
			// No global config, component missing version
			cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image:      "test-image",
						Repository: "test-repo",
					},
				},
			}
			_, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version is required"))
		})
		It("should use global imagePullSecrets when component has none", func() {
			cr := getTestClusterPolicyWithBaseFields()
			cr.Name = "nic-cluster-policy"
			// Set global config with imagePullSecrets
			cr.Spec.Global = &mellanoxv1alpha1.GlobalConfig{
				Repository:       "global-repo",
				Version:          "global-version",
				ImagePullSecrets: []string{"global-secret"},
			}
			// Component has no imagePullSecrets
			cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image: "test-image",
					},
				},
			}
			objs, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(BeNumerically(">=", 1))
			ds := objs[0]
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})
			imagePullSecrets := templateSpec["imagePullSecrets"].([]interface{})
			Expect(len(imagePullSecrets)).To(Equal(1))
			Expect(imagePullSecrets[0].(map[string]interface{})["name"]).To(Equal("global-secret"))
		})
	})
})

func getContainers(obj *unstructured.Unstructured) []interface{} {
	spec := obj.Object["spec"].(map[string]interface{})
	template := spec["template"].(map[string]interface{})
	templateSpec := template["spec"].(map[string]interface{})
	return templateSpec["containers"].([]interface{})
}

func getRDMASharedDevicePlugin() *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()
	imageSpec := addContainerResources(getTestImageSpec(), "rdma-shared-dp", "1", "9")
	cr.Name = "nic-cluster-policy"
	cr.Spec.RdmaSharedDevicePlugin = &mellanoxv1alpha1.DevicePluginSpec{
		ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
			ImageSpec: *imageSpec,
		},
	}
	return cr
}

func getMinimalNicNodePolicyWithRdmaSharedDevicePlugin(name string) *mellanoxv1alpha1.NicNodePolicy {
	clusterPolicy := getRDMASharedDevicePlugin()
	return &mellanoxv1alpha1.NicNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: mellanoxv1alpha1.NicNodePolicySpec{
			OFEDDriver:             nil,
			RdmaSharedDevicePlugin: clusterPolicy.Spec.RdmaSharedDevicePlugin.DeepCopy(),
			SriovDevicePlugin:      nil,
			NodeSelector:           map[string]string{"node-role.kubernetes.io/worker": ""},
			Labels:                 map[string]string{},
			Annotations:            map[string]string{},
			Tolerations:            []v1.Toleration{},
		},
		Status: mellanoxv1alpha1.NicNodePolicyStatus{
			State:         mellanoxv1alpha1.StateIgnore,
			Reason:        "",
			AppliedStates: []mellanoxv1alpha1.AppliedState{},
			Conditions:    []metav1.Condition{},
		},
	}
}

func renderRdmaSharedDevicePluginDaemonSet(
	ts *testScope, cr mellanoxv1alpha1.NicPolicyCR) *appsv1.DaemonSet {
	objects, err := ts.renderer.GetManifestObjects(ts.context, cr, ts.catalog, testLogger)
	Expect(err).NotTo(HaveOccurred())

	for _, object := range objects {
		if object.GetKind() != "DaemonSet" {
			continue
		}
		ds := &appsv1.DaemonSet{}
		Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, ds)).To(Succeed())
		return ds
	}

	Fail("RDMA shared device plugin DaemonSet was not rendered")
	return nil
}
