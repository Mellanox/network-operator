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

package migrate

import (
	goctx "context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/network-operator/pkg/consts"
)

//nolint:dupl
var _ = Describe("Migrate", func() {
	AfterEach(func() {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: nvIPAMcmName}}
		_ = k8sClient.Delete(goctx.Background(), cm)
	})
	It("should remove annotation on NVIPAM CM", func() {
		createConfigMap(true)
		err := Migrate(goctx.Background(), testLog, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		cm := &corev1.ConfigMap{}
		err = k8sClient.Get(goctx.Background(),
			types.NamespacedName{Namespace: namespaceName, Name: nvIPAMcmName}, cm)
		Expect(err).NotTo(HaveOccurred())
		_, ok := cm.Labels[consts.StateLabel]
		Expect(ok).To(BeFalse())
	})
	It("should not fail if state label does not exists", func() {
		createConfigMap(false)
		err := Migrate(goctx.Background(), testLog, k8sClient)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should not fail if NVIPAM CM does not exists", func() {
		err := Migrate(goctx.Background(), testLog, k8sClient)
		Expect(err).NotTo(HaveOccurred())
	})
})

func createConfigMap(addLabel bool) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: nvIPAMcmName}}
	if addLabel {
		cm.Labels = make(map[string]string)
		cm.Labels[consts.StateLabel] = "state-nv-ipam-cni"
	}
	err := k8sClient.Create(goctx.Background(), cm)
	Expect(err).NotTo(HaveOccurred())
}
