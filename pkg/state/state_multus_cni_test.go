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

package state

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

var _ = Describe("Multus CNI state", func() {
	var state stateMultusCNI
	var catalog InfoCatalog
	var networkOperatorResourceNamespace string

	BeforeEach(func() {
		manifestBaseDir := "../../manifests/state-multus-cni"
		files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
		Expect(err).NotTo(HaveOccurred())

		client := mocks.ControllerRuntimeClient{}
		renderer := render.NewRenderer(files)
		state = stateMultusCNI{
			stateSkel: stateSkel{
				name:        "state-multus-cni",
				description: "multus CNI deployed in the cluster",
				client:      &client,
				renderer:    renderer,
			}}
		catalog = NewInfoCatalog()
		catalog.Add(InfoTypeStaticConfig, &dummyProvider{})
		catalog.Add(InfoTypeClusterType, &dummyProvider{})
		networkOperatorResourceNamespace = config.FromEnv().State.NetworkOperatorResourceNamespace
	})

	It("should render ServiceAccount", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ServiceAccount", func(obj *unstructured.Unstructured) {
			Expect(obj.GetNamespace()).To(Equal(networkOperatorResourceNamespace))
		})).To(BeTrue())
	})

	It("should render ClusterRoleBinding", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ClusterRoleBinding", func(obj *unstructured.Unstructured) {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &clusterRoleBinding)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(clusterRoleBinding.Subjects)).To(Equal(1))
			Expect(clusterRoleBinding.Subjects[0].Namespace).To(Equal(networkOperatorResourceNamespace))
		})).To(BeTrue())
	})

	It("should render Daemonset", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "DaemonSet", func(obj *unstructured.Unstructured) {
			var daemonSet appsv1.DaemonSet
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &daemonSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(daemonSet.Namespace).To(Equal(networkOperatorResourceNamespace))
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

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
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

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
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

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
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

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
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

	It("should render config map with expected key", func() {
		cr := getMinimalNicClusterPolicyWithMultus()
		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ConfigMap", func(obj *unstructured.Unstructured) {
			var configMap corev1.ConfigMap
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &configMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(configMap.Namespace).To(Equal(networkOperatorResourceNamespace))
			defaultConfig := make(map[string]interface{})
			Expect(configMap.Data).To(HaveKey("daemon-config.json"))
			// just make sure data is valid json
			Expect(json.Unmarshal([]byte(configMap.Data["daemon-config.json"]), &defaultConfig)).ToNot(HaveOccurred())
			Expect(defaultConfig).ToNot(BeEmpty())
		})).To(BeTrue())
	})

	It("should render config map with config as specified in CR", func() {
		cr := getMinimalNicClusterPolicyWithMultus()

		configString := "myconfig"
		cr.Spec.SecondaryNetwork.Multus.Config = &configString

		objs, err := state.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())

		Expect(runFuncForObjectInSlice(objs, "ConfigMap", func(obj *unstructured.Unstructured) {
			var configMap corev1.ConfigMap
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &configMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(configMap.Namespace).To(Equal(networkOperatorResourceNamespace))
			Expect(configMap.Data).To(HaveKey("daemon-config.json"))
			Expect(configMap.Data["daemon-config.json"]).To(Equal(configString))
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
