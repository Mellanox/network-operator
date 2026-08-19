/*
Copyright 2021 NVIDIA

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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("SR-IOV Device Plugin State tests", func() {
	var (
		ts testScope
	)

	BeforeEach(func() {
		ts = ts.New(state.NewStateSriovDp, "../../manifests/state-sriov-device-plugin")
		Expect(ts).NotTo(BeNil())
	})

	Context("When creating NCP with SRIOV-device-plugin", func() {
		It("should create Daemonset - minimal spec", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(false)
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace,
				Name: "network-operator-sriov-device-plugin"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.SriovDevicePlugin.ImageSpec, cr)
			// expect privileged mode
			Expect(*ds.Spec.Template.Spec.Containers[0].SecurityContext.Privileged).To(BeTrue())
			assertSriovDpPodTemplatesVolumeFields(&ds.Spec.Template, false)
			assertSriovDpPodTemplatesVolumeMountFields(&ds.Spec.Template, false)
		})
		It("should create Daemonset - minimal spec -SHA256 image format", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(false)
			cr.Spec.SriovDevicePlugin.Version = defaultTestVersionSha256
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace,
				Name: "network-operator-sriov-device-plugin"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.SriovDevicePlugin.ImageSpec, cr)
			// expect privileged mode
			Expect(*ds.Spec.Template.Spec.Containers[0].SecurityContext.Privileged).To(BeTrue())
			assertSriovDpPodTemplatesVolumeFields(&ds.Spec.Template, false)
			assertSriovDpPodTemplatesVolumeMountFields(&ds.Spec.Template, false)
		})
		It("should create Daemonset with CDI support when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(true)
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace,
				Name: "network-operator-sriov-device-plugin"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.SriovDevicePlugin.ImageSpec, cr)
			// expect privileged mode
			Expect(*ds.Spec.Template.Spec.Containers[0].SecurityContext.Privileged).To(BeTrue())
			assertSriovDpPodTemplatesVolumeFields(&ds.Spec.Template, true)
			assertSriovDpPodTemplatesVolumeMountFields(&ds.Spec.Template, true)
			Expect(ds.Spec.Template.Spec.Containers[0].Args).Should(ContainElement(ContainSubstring("--use-cdi")))
		})
	})
	Context("When rendering a NicNodePolicy with SRIOV-device-plugin", func() {
		DescribeTable("should use a deterministic policy name hash in the DaemonSet name",
			func(policyName, expectedHash string) {
				expectedName := "network-operator-sriov-device-plugin-" + expectedHash
				ds := renderSriovDpDaemonSet(&ts, getMinimalNicNodePolicyWithSriovDp(policyName))

				Expect(ds.Name).To(Equal(expectedName))
				Expect(len(ds.Name)).To(BeNumerically("<=", validation.DNS1123LabelMaxLength))
				Expect(validation.IsDNS1123Subdomain(ds.Name)).To(BeEmpty())
				Expect(validation.IsValidLabelValue(ds.Name)).To(BeEmpty())
				Expect(ds.Spec.Selector.MatchLabels).To(HaveKeyWithValue("name", expectedName))
				Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("name", expectedName))
				Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(And(
					HaveField("Name", "config-volume"),
					HaveField("ConfigMap.Name", "network-operator-sriovdp-config-"+policyName),
				)))
			},
			Entry("for a readable policy name", "pool-a", "779d97f8c7"),
			Entry("for the maximum admitted policy name length", strings.Repeat("a", 30), "84bb999775"),
		)
	})
	Context("Verify Sync flows", func() {
		It("should create Daemonset, update state to Ready", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(false)
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace,
				Name: "network-operator-sriov-device-plugin"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.SriovDevicePlugin.ImageSpec, cr)
			assertSriovDpPodTemplatesVolumeFields(&ds.Spec.Template, false)
			assertSriovDpPodTemplatesVolumeMountFields(&ds.Spec.Template, false)
			By("Update DaemonSet Status, and re-run Sync")
			ds.Status = appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 1,
			}
			err = ts.client.Status().Update(context.Background(), ds)
			Expect(err).NotTo(HaveOccurred())
			By("Verify State is ready")
			ctx := context.Background()
			objs, err := ts.renderer.GetManifestObjects(ctx, cr, ts.catalog, log.FromContext(ctx))
			Expect(err).NotTo(HaveOccurred())
			status, err = getKindState(ctx, ts.client, objs, "DaemonSet")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should create Daemonset and delete if Spec is nil", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(false)
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace,
				Name: "network-operator-sriov-device-plugin"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.SriovDevicePlugin.ImageSpec, cr)
			assertSriovDpPodTemplatesVolumeFields(&ds.Spec.Template, false)
			assertSriovDpPodTemplatesVolumeMountFields(&ds.Spec.Template, false)
			By("Set spec to nil and Sync")
			cr.Spec.SriovDevicePlugin = nil
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet is deleted")
			ds = &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace,
				Name: "network-operator-sriov-device-plugin"}, ds)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})

func getMinimalNicClusterPolicyWithSriovDp(useCDI bool) *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()

	// add an arbitrary resource, this prevent adding defaut cpu,mem limits
	imageSpec := addContainerResources(getTestImageSpec(), "kube-sriovdp", "5", "3")
	dpSpec := &mellanoxv1alpha1.DevicePluginSpec{
		ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
			ImageSpec: *imageSpec,
			Config:    ptr.To("config"),
		},
		UseCdi: useCDI,
	}
	cr.Spec.SriovDevicePlugin = dpSpec
	return cr
}

func getMinimalNicNodePolicyWithSriovDp(name string) *mellanoxv1alpha1.NicNodePolicy {
	clusterPolicy := getMinimalNicClusterPolicyWithSriovDp(false)
	return &mellanoxv1alpha1.NicNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: mellanoxv1alpha1.NicNodePolicySpec{
			OFEDDriver:             nil,
			RdmaSharedDevicePlugin: nil,
			SriovDevicePlugin:      clusterPolicy.Spec.SriovDevicePlugin.DeepCopy(),
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

func renderSriovDpDaemonSet(ts *testScope, cr mellanoxv1alpha1.NicPolicyCR) *appsv1.DaemonSet {
	ctx := context.Background()
	objects, err := ts.renderer.GetManifestObjects(ctx, cr, ts.catalog, log.FromContext(ctx))
	Expect(err).NotTo(HaveOccurred())

	for _, object := range objects {
		if object.GetKind() != "DaemonSet" {
			continue
		}
		ds := &appsv1.DaemonSet{}
		Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, ds)).To(Succeed())
		return ds
	}

	Fail("SR-IOV device plugin DaemonSet was not rendered")
	return nil
}

func assertSriovDpPodTemplatesVolumeFields(tpl *v1.PodTemplateSpec, useCdi bool) {
	src := []v1.Volume{
		{
			Name: "devicesock",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/device-plugins",
				},
			},
		},
		{
			Name: "plugins-registry",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/plugins_registry",
				},
			},
		},
		{
			Name: "log",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/log",
				},
			},
		},
		{
			Name: "device-info",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/run/k8s.cni.cncf.io/devinfo/dp",
					Type: ptr.To(v1.HostPathDirectoryOrCreate),
				},
			},
		},
		{
			Name: "config-volume",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "network-operator-sriovdp-config",
					},
					Items: []v1.KeyToPath{
						{
							Key:  "config.json",
							Path: "config.json",
						},
					},
				},
			},
		},
	}

	if useCdi {
		src = append(src, []v1.Volume{
			{
				Name: "dynamic-cdi",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/var/run/cdi",
						Type: ptr.To(v1.HostPathDirectoryOrCreate),
					},
				},
			},
			{
				Name: "host-config-volume",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/etc/pcidp",
						Type: ptr.To(v1.HostPathDirectoryOrCreate),
					},
				},
			},
		}...)
	}
	Expect(tpl.Spec.Volumes).To(Equal(src))
}

func assertSriovDpPodTemplatesVolumeMountFields(tpl *v1.PodTemplateSpec, useCdi bool) {
	vlm := []v1.VolumeMount{
		{
			Name:      "devicesock",
			MountPath: "/var/lib/kubelet/device-plugins",
			ReadOnly:  false,
		},
		{
			Name:      "plugins-registry",
			MountPath: "/var/lib/kubelet/plugins_registry",
			ReadOnly:  false,
		},
		{
			Name:      "log",
			MountPath: "/var/log",
		},
		{
			Name:      "config-volume",
			MountPath: "/etc/pcidp",
		},
		{
			Name:      "device-info",
			MountPath: "/var/run/k8s.cni.cncf.io/devinfo/dp",
		},
	}
	if useCdi {
		vlm = append(vlm, []v1.VolumeMount{
			{
				Name:      "dynamic-cdi",
				MountPath: "/var/run/cdi",
			},
			{
				Name:      "host-config-volume",
				MountPath: "/host/etc/pcidp/",
			},
		}...)
	}

	Expect(tpl.Spec.Containers[0].VolumeMounts).To(Equal(vlm))
}
