/*
2023 NVIDIA CORPORATION & AFFILIATES

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Mellanox/network-operator/pkg/consts"
)

const (
	testState = "test"
)

var testSa = &corev1.ServiceAccount{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test",
		Labels: map[string]string{
			consts.StateLabel: testState,
		}},
}

var _ = Describe("stateSkel", func() {
	var (
		s   stateSkel
		ctx context.Context
	)
	BeforeEach(func() {
		s = stateSkel{name: testState}
		ctx = context.Background()
	})
	Context("handleStaleStateObjects", func() {
		It("No obj", func() {
			s.client = fake.NewClientBuilder().Build()
			wait, err := s.handleStaleStateObjects(ctx, []*unstructured.Unstructured{})
			Expect(err).To(BeNil())
			Expect(wait).To(BeFalse())
		})
		It("In sync", func() {
			s.client = fake.NewClientBuilder().WithObjects(testSa).Build()
			unstrSa, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testSa)
			Expect(err).NotTo(HaveOccurred())
			wait, err := s.handleStaleStateObjects(ctx, []*unstructured.Unstructured{{Object: unstrSa}})
			Expect(err).To(BeNil())
			Expect(wait).To(BeFalse())
		})
		It("Stale object", func() {
			s.client = fake.NewClientBuilder().WithObjects(testSa).Build()
			wait, err := s.handleStaleStateObjects(ctx, []*unstructured.Unstructured{})
			Expect(err).To(BeNil())
			Expect(wait).To(BeTrue())
		})
	})
})

var _ = Describe("SetConfigHashAnnotation", func() {
	Context("when configHash is empty", func() {
		It("should not modify any objects", func() {
			ds := createTestDaemonSet("test-ds", "test-ns")
			objs := []*unstructured.Unstructured{ds}
			err := SetConfigHashAnnotation(objs, "")
			Expect(err).NotTo(HaveOccurred())

			annotations, found, err := unstructured.NestedStringMap(ds.Object,
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
			Expect(annotations).To(BeNil())
		})
	})

	Context("when configHash is provided", func() {
		It("should add annotation to DaemonSet pod template", func() {
			ds := createTestDaemonSet("test-ds", "test-ns")
			objs := []*unstructured.Unstructured{ds}
			configHash := "abc123hash"

			err := SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			annotations, found, err := unstructured.NestedStringMap(ds.Object,
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(annotations[consts.ConfigHashAnnotation]).To(Equal(configHash))
		})

		It("should preserve existing annotations", func() {
			ds := createTestDaemonSet("test-ds", "test-ns")
			// Add existing annotation
			err := unstructured.SetNestedStringMap(ds.Object,
				map[string]string{"existing": "annotation"},
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())

			objs := []*unstructured.Unstructured{ds}
			configHash := "abc123hash"

			err = SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			annotations, found, err := unstructured.NestedStringMap(ds.Object,
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(annotations["existing"]).To(Equal("annotation"))
			Expect(annotations[consts.ConfigHashAnnotation]).To(Equal(configHash))
		})

		It("should not modify non-DaemonSet objects", func() {
			configMap := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-cm",
						"namespace": "test-ns",
					},
				},
			}
			objs := []*unstructured.Unstructured{configMap}
			configHash := "abc123hash"

			err := SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			// ConfigMap should not have the annotation path
			_, found, _ := unstructured.NestedStringMap(configMap.Object,
				"spec", "template", "metadata", "annotations")
			Expect(found).To(BeFalse())
		})

		It("should handle multiple DaemonSets", func() {
			ds1 := createTestDaemonSet("test-ds-1", "test-ns")
			ds2 := createTestDaemonSet("test-ds-2", "test-ns")
			objs := []*unstructured.Unstructured{ds1, ds2}
			configHash := "abc123hash"

			err := SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			for _, ds := range objs {
				annotations, found, err := unstructured.NestedStringMap(ds.Object,
					"spec", "template", "metadata", "annotations")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(annotations[consts.ConfigHashAnnotation]).To(Equal(configHash))
			}
		})
	})
})

func createTestDaemonSet(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": name,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": name,
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test-container",
								"image": "test-image",
							},
						},
					},
				},
			},
		},
	}
}
