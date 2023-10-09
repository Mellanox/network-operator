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
