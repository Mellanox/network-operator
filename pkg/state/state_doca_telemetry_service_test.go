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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
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
