/*
2022 NVIDIA CORPORATION & AFFILIATES

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

package controllers //nolint:dupl

import (
	goctx "context"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

//nolint:dupl
var _ = Describe("IPoIBNetwork Controller", func() {

	Context("When IPoIBNetwork CR is created", func() {
		It("should create and delete ipoib network", func() {
			cr := mellanoxv1alpha1.IPoIBNetwork{
				TypeMeta: metav1.TypeMeta{
					Kind:       "IPoIBNetwork",
					APIVersion: "mellanox.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mellanoxv1alpha1.IPoIBNetworkSpec{
					NetworkNamespace: "default",
					Master:           "ibs3",
				},
			}

			err := k8sClient.Create(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			found := &mellanoxv1alpha1.IPoIBNetwork{}
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.NetworkNamespace).To(Equal("default"))
			Expect(found.Spec.Master).To(Equal("ibs3"))
			Expect(found.Spec.IPAM).To(Equal(""))

			Eventually(func() error {
				netAttachDef := &netattdefv1.NetworkAttachmentDefinition{}
				return k8sClient.Get(goctx.TODO(),
					types.NamespacedName{Namespace: "default", Name: cr.GetName()},
					netAttachDef)
			}, timeout*3, interval).ShouldNot(HaveOccurred())

			err = k8sClient.Delete(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create ipoib network empty state", func() {
			cr := mellanoxv1alpha1.IPoIBNetwork{
				TypeMeta: metav1.TypeMeta{
					Kind:       "IPoIBNetwork",
					APIVersion: "mellanox.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-with-error",
					Namespace: "default",
				},
				Spec: mellanoxv1alpha1.IPoIBNetworkSpec{
					NetworkNamespace: "default",
					Master:           "",
					IPAM:             " {\"type\": whereabouts}",
				},
			}

			err := k8sClient.Create(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			found := &mellanoxv1alpha1.IPoIBNetwork{}
			state := mellanoxv1alpha1.State("")
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Status.State).To(Equal(state))
			Expect(found.Status.Reason).To(Equal(""))

			Eventually(func() error {
				netAttachDef := &netattdefv1.NetworkAttachmentDefinition{}
				return k8sClient.Get(goctx.TODO(),
					types.NamespacedName{Namespace: "default", Name: cr.GetName()},
					netAttachDef)
			}, timeout*3, interval).ShouldNot(HaveOccurred())
		})
	})
})
