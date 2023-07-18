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

package controllers //nolint:dupl

import (
	goctx "context"

	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

//nolint:dupl
var _ = Describe("HostDeviceNetwork Controller", func() {

	Context("When HostDeviceNetwork CR is created", func() {
		It("should create and delete hostdevice network", func() {
			cr := mellanoxv1alpha1.HostDeviceNetwork{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HostDeviceNetwork",
					APIVersion: "mellanox.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "",
				},
				Spec: mellanoxv1alpha1.HostDeviceNetworkSpec{
					NetworkNamespace: "default",
					ResourceName:     "test",
				},
			}

			err := k8sClient.Create(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			found := &mellanoxv1alpha1.HostDeviceNetwork{}
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.NetworkNamespace).To(Equal("default"))
			Expect(found.Spec.ResourceName).To(Equal("test"))
			Expect(found.Spec.IPAM).To(Equal(""))

			err = k8sClient.Delete(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create hostdevice network empty state", func() {
			cr := mellanoxv1alpha1.HostDeviceNetwork{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HostDeviceNetwork",
					APIVersion: "mellanox.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-with-error",
					Namespace: "default",
				},
				Spec: mellanoxv1alpha1.HostDeviceNetworkSpec{
					NetworkNamespace: "",
					ResourceName:     "",
					IPAM:             " {\"type\": whereabouts}",
				},
			}

			err := k8sClient.Create(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			found := &mellanoxv1alpha1.HostDeviceNetwork{}
			state := mellanoxv1alpha1.State("")
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Status.State).To(Equal(state))
			Expect(found.Status.Reason).To(Equal(""))

		})
	})
})
