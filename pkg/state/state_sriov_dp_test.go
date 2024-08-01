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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("SR-IOV Device Plugin State tests", func() {
	var (
		sriovDpState state.State
		catalog      state.InfoCatalog
		client       client.Client
		namespace    string
		renderer     state.ManifestRenderer
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(v1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(appsv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir := "../../manifests/state-sriov-device-plugin"
		s, r, err := state.NewStateSriovDp(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		sriovDpState = s
		renderer = r
		catalog = getTestCatalog()
		namespace = config.FromEnv().State.NetworkOperatorResourceNamespace
	})

	Context("When creating NCP with SRIOV-device-plugin", func() {
		It("should create Daemonset - minimal spec", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(false)
			status, err := sriovDpState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace,
				Name: "sriov-device-plugin"}, ds)
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
			status, err := sriovDpState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace,
				Name: "sriov-device-plugin"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.SriovDevicePlugin.ImageSpec, cr)
			// expect privileged mode
			Expect(*ds.Spec.Template.Spec.Containers[0].SecurityContext.Privileged).To(BeTrue())
			assertSriovDpPodTemplatesVolumeFields(&ds.Spec.Template, true)
			assertSriovDpPodTemplatesVolumeMountFields(&ds.Spec.Template, true)
			Expect(ds.Spec.Template.Spec.Containers[0].Args).Should(ContainElement(ContainSubstring("--use-cdi")))
		})
	})
	Context("Verify Sync flows", func() {
		It("should create Daemonset, update state to Ready", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(false)
			status, err := sriovDpState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace,
				Name: "sriov-device-plugin"}, ds)
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
			err = client.Status().Update(context.Background(), ds)
			Expect(err).NotTo(HaveOccurred())
			By("Verify State is ready")
			ctx := context.Background()
			objs, err := renderer.GetManifestObjects(ctx, cr, catalog, log.FromContext(ctx))
			Expect(err).NotTo(HaveOccurred())
			status, err = getKindState(ctx, client, objs, "DaemonSet")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should create Daemonset and delete if Spec is nil", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithSriovDp(false)
			status, err := sriovDpState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace,
				Name: "sriov-device-plugin"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.SriovDevicePlugin.ImageSpec, cr)
			assertSriovDpPodTemplatesVolumeFields(&ds.Spec.Template, false)
			assertSriovDpPodTemplatesVolumeMountFields(&ds.Spec.Template, false)
			By("Set spec to nil and Sync")
			cr.Spec.SriovDevicePlugin = nil
			status, err = sriovDpState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet is deleted")
			ds = &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace,
				Name: "sriov-device-plugin"}, ds)
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
						Name: "sriovdp-config",
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
