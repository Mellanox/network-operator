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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("Multus CNI state", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateMultusCNI, "../../manifests/state-multus-cni")
		Expect(ts).NotTo(BeNil())

		ts.catalog.Add(state.InfoTypeStaticConfig,
			staticconfig.NewProvider(staticconfig.StaticConfig{
				CniBinDirectory:     "custom-cni-bin-directory",
				CniNetworkDirectory: "custom-cni-network-directory",
			}))
	})

	It("should render ServiceAccount", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ServiceAccount", func(obj *unstructured.Unstructured) {
			Expect(obj.GetNamespace()).To(Equal(ts.namespace))
		})).To(BeTrue())
	})

	It("should render ClusterRoleBinding", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ClusterRoleBinding", func(obj *unstructured.Unstructured) {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &clusterRoleBinding)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(clusterRoleBinding.Subjects)).To(Equal(1))
			Expect(clusterRoleBinding.Subjects[0].Namespace).To(Equal(ts.namespace))
		})).To(BeTrue())
	})

	It("should render Daemonset", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(daemonSet.Namespace).To(Equal(ts.namespace))
			Expect(daemonSet.Spec.Template.Spec.Affinity).To(BeNil())
			Expect(daemonSet.Spec.Template.Spec.ImagePullSecrets).To(BeNil())
			Expect(daemonSet.Spec.Template.Spec.Tolerations).To(Equal(
				[]corev1.Toleration{
					{
						Key:      "nvidia.com/gpu",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			))

			cpu, _ := resource.ParseQuantity("100m")
			mem, _ := resource.ParseQuantity("50Mi")
			Expect(daemonSet.Spec.Template.Spec.Containers[0].Resources).To(Equal(
				corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    cpu,
						corev1.ResourceMemory: mem,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    cpu,
						corev1.ResourceMemory: mem,
					},
				},
			))
			Expect(daemonSet.Spec.Template.Spec.Containers[0].Image).To(Equal("myrepo/myimage:myversion"))
		})).To(BeTrue())
	})

	It("should render Daemonset image with SHA256 format", func() {
		cr := getMinimalNicClusterPolicyWithMultus()
		cr.Spec.SecondaryNetwork.Multus.Version = "sha256:1699d23027ea30c9fa"

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(daemonSet.Spec.Template.Spec.Containers[0].Image).To(Equal("myrepo/myimage@sha256:1699d23027ea30c9fa"))
		})).To(BeTrue())
	})

	It("should render Daemonset with NodeAffinity when specified in CR", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		nodeAffinity := corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "mykey",
								Operator: corev1.NodeSelectorOpExists,
							},
						},
					},
				},
			},
		}
		nodeAffinityCopy := nodeAffinity.DeepCopy()
		cr.Spec.NodeAffinity = nodeAffinityCopy

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(daemonSet.Spec.Template.Spec.Affinity.NodeAffinity).To(Equal(&nodeAffinity))
		})).To(BeTrue())
	})

	It("should render Daemonset with ImagePullSecrets when specified in CR", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		cr.Spec.SecondaryNetwork.Multus.ImagePullSecrets = []string{"myimagepullsecret"}

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(daemonSet.Spec.Template.Spec.ImagePullSecrets)).To(Equal(1))
			Expect(daemonSet.Spec.Template.Spec.ImagePullSecrets[0]).To(Equal(
				corev1.LocalObjectReference{
					Name: "myimagepullsecret",
				},
			))
		})).To(BeTrue())
	})

	It("should render Daemonset with Tolerations when specified in CR", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		toleration := corev1.Toleration{
			Key:      "mykey",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		}
		cr.Spec.Tolerations = []corev1.Toleration{toleration}

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(daemonSet.Spec.Template.Spec.Tolerations).To(Equal(
				[]corev1.Toleration{
					toleration,
					{
						Key:      "nvidia.com/gpu",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			))
		})).To(BeTrue())
	})

	It("should render Daemonset with Resources when specified in CR", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		cpu, _ := resource.ParseQuantity("1")
		mem, _ := resource.ParseQuantity("1Gi")
		cr.Spec.SecondaryNetwork.Multus.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{
			{
				Name: "kube-multus",
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    cpu,
					corev1.ResourceMemory: mem,
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    cpu,
					corev1.ResourceMemory: mem,
				},
			},
		}

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(daemonSet.Spec.Template.Spec.Containers[0].Resources).To(Equal(
				corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    cpu,
						corev1.ResourceMemory: mem,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    cpu,
						corev1.ResourceMemory: mem,
					},
				},
			))
		})).To(BeTrue())
	})

	It("should not render ConfigMap for thin mode", func() {
		cr := getMinimalNicClusterPolicyWithMultus()
		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ConfigMap", func(_ *unstructured.Unstructured) {})).To(BeFalse())
	})

	It("should return an error when Config is set in thin mode", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		configString := `{"chrootDir": "/hostroot", "logLevel": "debug"}`
		cr.Spec.SecondaryNetwork.Multus.Config = &configString

		_, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("deploymentType: thick"))
	})

	It("should render ConfigMap with default daemon config for thick mode without config", func() {
		cr := getMinimalNicClusterPolicyWithMultus()
		cr.Spec.SecondaryNetwork.Multus.DeploymentType = mellanoxv1alpha1.MultusDeploymentTypeThick

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ConfigMap", func(obj *unstructured.Unstructured) {
			var configMap corev1.ConfigMap
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &configMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(configMap.Name).To(Equal("multus-daemon-config"))
			Expect(configMap.Namespace).To(Equal(ts.namespace))
			Expect(configMap.Data).To(HaveKey("daemon-config.json"))
			Expect(configMap.Data["daemon-config.json"]).To(ContainSubstring("chrootDir"))
		})).To(BeTrue())
	})

	It("should render ConfigMap with custom daemon config for thick mode when config is specified in CR", func() {
		cr := getMinimalNicClusterPolicyWithMultus()
		cr.Spec.SecondaryNetwork.Multus.DeploymentType = mellanoxv1alpha1.MultusDeploymentTypeThick

		configString := `{"chrootDir": "/hostroot", "logLevel": "debug"}`
		cr.Spec.SecondaryNetwork.Multus.Config = &configString

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ConfigMap", func(obj *unstructured.Unstructured) {
			var configMap corev1.ConfigMap
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &configMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(configMap.Name).To(Equal("multus-daemon-config"))
			Expect(configMap.Data["daemon-config.json"]).To(Equal(configString))
		})).To(BeTrue())
	})

	It("should render thick DaemonSet with correct structure when DeploymentType is thick", func() {
		cr := getMinimalNicClusterPolicyWithMultus()
		cr.Spec.SecondaryNetwork.Multus.DeploymentType = mellanoxv1alpha1.MultusDeploymentTypeThick

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			// Init container uses /install_multus --type thick
			initContainers := daemonSet.Spec.Template.Spec.InitContainers
			Expect(initContainers).To(HaveLen(1))
			Expect(initContainers[0].Command).To(ContainElement("/install_multus"))
			Expect(initContainers[0].Args).To(ContainElement("thick"))

			mainContainer := daemonSet.Spec.Template.Spec.Containers[0]
			Expect(mainContainer.Command).To(ContainElement("/usr/src/multus-cni/bin/multus-daemon"))
			Expect(mainContainer.Args).To(ContainElement("--config=/etc/cni/net.d/multus.d/daemon-config.json"))

			// MULTUS_NODE_NAME env var
			Expect(mainContainer.Env).To(ContainElement(
				corev1.EnvVar{
					Name: "MULTUS_NODE_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
					},
				},
			))

			// Volume mounts
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "cni", MountPath: "/host/etc/cni/net.d"},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "cnibin", MountPath: "custom-cni-bin-directory"},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "hostroot",
					MountPath:        "/hostroot",
					MountPropagation: mountPropagationPtr(corev1.MountPropagationHostToContainer),
				},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "multus-cni-config", MountPath: "/etc/cni/net.d/multus.d", ReadOnly: true},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "host-run", MountPath: "/host/run"},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "host-var-lib-cni-multus", MountPath: "/var/lib/cni/multus"},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "host-var-lib-kubelet", MountPath: "/var/lib/kubelet",
					MountPropagation: mountPropagationPtr(corev1.MountPropagationHostToContainer)},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "host-run-k8s-cni-cncf-io", MountPath: "/run/k8s.cni.cncf.io"},
			))
			Expect(mainContainer.VolumeMounts).To(ContainElement(
				corev1.VolumeMount{Name: "host-run-netns", MountPath: "/run/netns",
					MountPropagation: mountPropagationPtr(corev1.MountPropagationHostToContainer)},
			))

			// Volumes
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{Name: "cni", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "custom-cni-network-directory"},
				}},
			))
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{Name: "hostroot", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/"},
				}},
			))
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{
					Name: "multus-cni-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "multus-daemon-config"},
							Items: []corev1.KeyToPath{
								{Key: "daemon-config.json", Path: "daemon-config.json"},
							},
						},
					},
				},
			))
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{Name: "host-run", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/run"},
				}},
			))
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{Name: "host-var-lib-cni-multus", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/cni/multus"},
				}},
			))
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{Name: "host-var-lib-kubelet", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet"},
				}},
			))
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{Name: "host-run-k8s-cni-cncf-io", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/run/k8s.cni.cncf.io"},
				}},
			))
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{Name: "host-run-netns", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/run/netns"},
				}},
			))
		})).To(BeTrue())
	})

	It("should render thin DaemonSet with correct structure when DeploymentType is thin", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			initContainers := daemonSet.Spec.Template.Spec.InitContainers
			Expect(initContainers).To(HaveLen(1))
			Expect(initContainers[0].Command).To(ContainElement("/install_multus"))
			Expect(initContainers[0].Args).To(ContainElement("thin"))

			mainContainer := daemonSet.Spec.Template.Spec.Containers[0]
			Expect(mainContainer.Command).To(ContainElement("/thin_entrypoint"))
			Expect(mainContainer.Args).To(ContainElement("--multus-conf-file=auto"))
			Expect(mainContainer.Env).To(BeEmpty())

			volumeMountNames := make([]string, 0, len(mainContainer.VolumeMounts))
			for _, vm := range mainContainer.VolumeMounts {
				volumeMountNames = append(volumeMountNames, vm.Name)
			}
			Expect(volumeMountNames).To(ContainElement("cninetwork"))
			Expect(volumeMountNames).To(ContainElement("cnibin"))
			Expect(volumeMountNames).NotTo(ContainElement("hostroot"))
			Expect(volumeMountNames).NotTo(ContainElement("multus-cni-config"))
			Expect(volumeMountNames).NotTo(ContainElement("host-run"))
		})).To(BeTrue())
	})

	It("should render Daemonset with custom CNI directories", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := ts.renderer.GetManifestObjects(context.TODO(), cr, ts.catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			// Verify CNI bin directory
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{
					Name: "cnibin",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "custom-cni-bin-directory",
						},
					},
				},
			))

			// Verify CNI network directory
			Expect(daemonSet.Spec.Template.Spec.Volumes).To(ContainElement(
				corev1.Volume{
					Name: "cninetwork",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "custom-cni-network-directory",
						},
					},
				},
			))

			// Verify volume mounts
			Expect(daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:      "cnibin",
					MountPath: "/host/opt/cni/bin",
				},
			))
			Expect(daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:      "cninetwork",
					MountPath: "/host/etc/cni/net.d",
				},
			))

			// Verify multus kubeconfig path uses the custom CNI network directory
			Expect(daemonSet.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
				"--multus-kubeconfig-file-host=custom-cni-network-directory/multus.d/multus.kubeconfig",
			))

			// Verify CNI conf dir argument
			Expect(daemonSet.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
				"--cni-conf-dir=/host/etc/cni/net.d",
			))
		})).To(BeTrue())
	})

})

func getMinimalNicClusterPolicyWithMultus() *mellanoxv1alpha1.NicClusterPolicy {
	cr := &mellanoxv1alpha1.NicClusterPolicy{}
	cr.Name = "nic-cluster-policy"

	secondaryNetworkSpec := &mellanoxv1alpha1.SecondaryNetworkSpec{}
	secondaryNetworkSpec.Multus = &mellanoxv1alpha1.MultusSpec{}
	secondaryNetworkSpec.Multus.ImageSpec.Image = "myimage"
	secondaryNetworkSpec.Multus.ImageSpec.Repository = "myrepo"
	secondaryNetworkSpec.Multus.ImageSpec.Version = "myversion"
	cr.Spec.SecondaryNetwork = secondaryNetworkSpec

	return cr
}

func mountPropagationPtr(p corev1.MountPropagationMode) *corev1.MountPropagationMode {
	return &p
}
