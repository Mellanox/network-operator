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
	"encoding/json"
	"fmt"

	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	clustertype_mocks "github.com/Mellanox/network-operator/pkg/clustertype/mocks"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
	staticconfig_mocks "github.com/Mellanox/network-operator/pkg/staticconfig/mocks"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func getTestCatalog() state.InfoCatalog {
	catalog := state.NewInfoCatalog()
	clusterTypeProvider := clustertype_mocks.Provider{}
	clusterTypeProvider.On("IsOpenshift").Return(false)
	staticConfigProvider := staticconfig_mocks.Provider{}
	staticConfigProvider.On("GetStaticConfig").Return(staticconfig.StaticConfig{CniBinDirectory: ""})
	catalog.Add(state.InfoTypeStaticConfig, &staticConfigProvider)
	catalog.Add(state.InfoTypeClusterType, &clusterTypeProvider)
	return catalog
}

type ipam struct {
	Type    string   `json:"type"`
	Range   string   `json:"range"`
	Exclude []string `json:"exclude"`
}

type nadConfig struct {
	CNIVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Master     string `json:"master"`
	Mode       string `json:"mode"`
	MTU        int    `json:"mtu"`
	IPAM       ipam   `json:"ipam"`
}

func getNADConfig(jsonData string) (*nadConfig, error) {
	config := &nadConfig{}
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func assertCommonPodTemplateFields(template *corev1.PodTemplateSpec, image *mellanoxv1alpha1.ImageSpec) {
	// Image name
	Expect(template.Spec.Containers[0].Image).To(Equal(
		fmt.Sprintf("%v/%v:%v", image.Repository, image.Image, image.Version)),
	)

	// ImagePullSecrets
	Expect(template.Spec.ImagePullSecrets).To(ConsistOf(
		corev1.LocalObjectReference{Name: "secret-one"},
		corev1.LocalObjectReference{Name: "secret-two"},
	))

	// Container Resources
	Expect(template.Spec.Containers[0].Resources.Limits).To(Equal(image.ContainerResources[0].Limits))
	Expect(template.Spec.Containers[0].Resources.Requests).To(Equal(image.ContainerResources[0].Requests))
}

func assertCommonDeploymentFields(u *unstructured.Unstructured, image *mellanoxv1alpha1.ImageSpec) {
	d := &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), d)
	Expect(err).ToNot(HaveOccurred())
	assertCommonPodTemplateFields(&d.Spec.Template, image)
}

func assertCommonDaemonSetFields(u *unstructured.Unstructured,
	image *mellanoxv1alpha1.ImageSpec, policy *mellanoxv1alpha1.NicClusterPolicy) {
	ds := &appsv1.DaemonSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), ds)
	Expect(err).ToNot(HaveOccurred())
	assertCommonPodTemplateFields(&ds.Spec.Template, image)
	// Tolerations
	Expect(ds.Spec.Template.Spec.Tolerations).To(ContainElements(
		corev1.Toleration{Key: "first-taint"},
		corev1.Toleration{
			Key:               "nvidia.com/gpu",
			Operator:          "Exists",
			Value:             "",
			Effect:            "NoSchedule",
			TolerationSeconds: nil,
		},
	))

	// NodeAffinity
	Expect(ds.Spec.Template.Spec.Affinity.NodeAffinity).To(Equal(policy.Spec.NodeAffinity))
}

func getTestImageSpec() *mellanoxv1alpha1.ImageSpec {
	return &mellanoxv1alpha1.ImageSpec{
		Image:            "image-one",
		Repository:       "repository",
		Version:          "five",
		ImagePullSecrets: []string{"secret-one", "secret-two"},
	}
}

func addContainerResources(imageSpec *mellanoxv1alpha1.ImageSpec,
	containerName, requestValue, limitValue string) *mellanoxv1alpha1.ImageSpec {
	i := imageSpec.DeepCopy()
	i.ContainerResources = append(i.ContainerResources, []mellanoxv1alpha1.ResourceRequirements{
		{
			Name:     containerName,
			Limits:   map[corev1.ResourceName]resource.Quantity{"resource-one": resource.MustParse(limitValue)},
			Requests: map[corev1.ResourceName]resource.Quantity{"resource-one": resource.MustParse(requestValue)},
		},
	}...)
	return i
}

func isNamespaced(obj *unstructured.Unstructured) bool {
	return obj.GetKind() != "CustomResourceDefinition" &&
		obj.GetKind() != "ClusterRole" &&
		obj.GetKind() != "ClusterRoleBinding" &&
		obj.GetKind() != "ValidatingWebhookConfiguration"
}

func assertCNIBinDirForDS(u *unstructured.Unstructured) {
	ds := &appsv1.DaemonSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), ds)
	Expect(err).ToNot(HaveOccurred())
	for i := range ds.Spec.Template.Spec.Volumes {
		vol := ds.Spec.Template.Spec.Volumes[i]
		if vol.Name == "cnibin" {
			Expect(vol.HostPath).NotTo(BeNil())
			Expect(vol.HostPath.Path).To(Equal("custom-cni-bin-directory"))
		}
	}
}

func GetManifestObjectsTest(ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy, catalog state.InfoCatalog,
	imageSpec *mellanoxv1alpha1.ImageSpec, renderer state.ManifestRenderer) {
	got, err := renderer.GetManifestObjects(ctx, cr, catalog, log.FromContext(ctx))
	Expect(err).ToNot(HaveOccurred())
	for i := range got {
		if isNamespaced(got[i]) {
			Expect(got[i].GetNamespace()).To(Equal("nvidia-network-operator"))
		}
		switch got[i].GetKind() {
		case "DaemonSet":
			assertCommonDaemonSetFields(got[i], imageSpec, cr)
			assertCNIBinDirForDS(got[i])
		case "Deployment":
			assertCommonDeploymentFields(got[i], imageSpec)
		}
	}
}

func getTestClusterPolicyWithBaseFields() *mellanoxv1alpha1.NicClusterPolicy {
	return &mellanoxv1alpha1.NicClusterPolicy{
		Spec: mellanoxv1alpha1.NicClusterPolicySpec{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      "node-label",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"labels"},
							},
							},
						},
					},
				},
				PreferredDuringSchedulingIgnoredDuringExecution: nil,
			},
			Tolerations: []corev1.Toleration{{Key: "first-taint"}},
		},
	}
}
