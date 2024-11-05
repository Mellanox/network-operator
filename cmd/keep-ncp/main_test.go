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

package main

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
)

var _ = Describe("NCP keep annotation", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		ncp := &mellanoxv1alpha1.NicClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "", Name: consts.NicClusterPolicyResourceName}}
		_ = k8sClient.Delete(ctx, ncp)
	})

	Describe("annotate keep NCP", func() {
		It("should succeed - NCP does not exists", func() {
			Expect(annotateNCP(ctx, k8sClient)).To(Succeed())
		})
		It("should succeed - NCP managed by Helm", func() {
			createNcp(true, false)
			Expect(annotateNCP(ctx, k8sClient)).To(Succeed())
			Eventually(func() bool {
				found := &mellanoxv1alpha1.NicClusterPolicy{}
				err := k8sClient.Get(context.TODO(),
					types.NamespacedName{Name: consts.NicClusterPolicyResourceName}, found)
				Expect(err).NotTo(HaveOccurred())
				val, ok := found.Annotations[helmResourcePolicyKey]
				return ok && val == helmKeepValue
			}, timeout*3, interval).Should(BeTrue())
		})
		It("should succeed - NCP not managed by Helm", func() {
			createNcp(false, false)
			Expect(annotateNCP(ctx, k8sClient)).To(Succeed())
			Eventually(func() bool {
				found := &mellanoxv1alpha1.NicClusterPolicy{}
				err := k8sClient.Get(context.TODO(),
					types.NamespacedName{Name: consts.NicClusterPolicyResourceName}, found)
				Expect(err).NotTo(HaveOccurred())
				_, ok := found.Annotations[helmResourcePolicyKey]
				return ok
			}, timeout*3, interval).Should(BeFalse())
		})
		It("should succeed - NCP managed by Helm, keep already set", func() {
			createNcp(true, true)
			Expect(annotateNCP(ctx, k8sClient)).To(Succeed())
			Consistently(func() bool {
				found := &mellanoxv1alpha1.NicClusterPolicy{}
				err := k8sClient.Get(context.TODO(),
					types.NamespacedName{Name: consts.NicClusterPolicyResourceName}, found)
				Expect(err).NotTo(HaveOccurred())
				val, ok := found.Annotations[helmResourcePolicyKey]
				return ok && val == helmKeepValue
			}, timeout, interval).Should(BeTrue())
		})
	})
})

func createNcp(helmManaged, keepAnnotation bool) {
	cr := mellanoxv1alpha1.NicClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.NicClusterPolicyResourceName,
			Namespace: "",
		},
	}
	if helmManaged {
		cr.Labels = map[string]string{"app.kubernetes.io/managed-by": "Helm"}
	}
	if keepAnnotation {
		cr.Annotations = map[string]string{helmResourcePolicyKey: helmKeepValue}
	}
	err := k8sClient.Create(context.TODO(), &cr)
	Expect(err).NotTo(HaveOccurred())
}
