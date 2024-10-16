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

package controllers //nolint:dupl

import (
	goctx "context"
	"fmt"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

const (
	lastNetworkNamespaceAnnot = "operator.macvlannetwork.mellanox.com/last-network-namespace"
	testNetworkNamespace      = "default"
	crName                    = "test"
	crNamespace               = "default"
)

//nolint:dupl
var _ = Describe("MacVlanNetwork Controller", func() {

	Context("When MacvlanNetwork CR is created", func() {
		It("should create Network Attachment Definition", func() {
			cr := mellanoxv1alpha1.MacvlanNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: crNamespace,
				},
				Spec: mellanoxv1alpha1.MacvlanNetworkSpec{
					NetworkNamespace: testNetworkNamespace,
					Master:           "ibs3",
					Mode:             "bridge",
					Mtu:              150,
				},
			}

			By("Create MacvlanNetwork")
			err := k8sClient.Create(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			found := &mellanoxv1alpha1.MacvlanNetwork{}
			err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Spec.NetworkNamespace).To(Equal(testNetworkNamespace))
			Expect(found.Spec.Master).To(Equal("ibs3"))
			Expect(found.Spec.IPAM).To(Equal(""))

			By("Verify NAD is created")
			verifyNADCreated(cr.GetName(), testNetworkNamespace)

			By("Verify network namespace annotation")
			verifyNetworkNamespaceAnnot(testNetworkNamespace)

			By("Verify MacvlanNetwork Status")
			verifyMacvlanStatus(testNetworkNamespace)

			By("Update NetworkNamespace")
			Eventually(func() error {
				found = &mellanoxv1alpha1.MacvlanNetwork{}
				err = k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
				Expect(err).NotTo(HaveOccurred())
				found.Spec.NetworkNamespace = namespaceName
				err = k8sClient.Update(goctx.TODO(), found)
				return err
			}, timeout*30, interval).ShouldNot(HaveOccurred())

			By("Verify new network namespace annotation")
			verifyNetworkNamespaceAnnot(namespaceName)

			By("Verify MacvlanNetwork Status with new Namespace")
			verifyMacvlanStatus(namespaceName)

			By("Verify NAD is deleted")
			Eventually(func() bool {
				netAttachDef := &netattdefv1.NetworkAttachmentDefinition{}
				err := k8sClient.Get(goctx.TODO(),
					types.NamespacedName{Namespace: testNetworkNamespace, Name: cr.GetName()},
					netAttachDef)
				return errors.IsNotFound(err)
			}, timeout*30, interval).Should(BeTrue())

			By("Verify NAD is created in new namespace")
			verifyNADCreated(cr.GetName(), namespaceName)

			By("Delete MacvlanNetwork")
			err = k8sClient.Delete(goctx.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			By("Delete NAD")
			netAttachDef := &netattdefv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: cr.GetName(), Namespace: namespaceName}}
			err = k8sClient.Delete(goctx.TODO(), netAttachDef)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should not create Network Attachment Definition", func() {
		cr := mellanoxv1alpha1.MacvlanNetwork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      crName,
				Namespace: crNamespace,
			},
			Spec: mellanoxv1alpha1.MacvlanNetworkSpec{
				NetworkNamespace: "no-exists",
				Master:           "ibs3",
				Mode:             "bridge",
				Mtu:              150,
			},
		}

		By("Create MacvlanNetwork")
		err := k8sClient.Create(goctx.TODO(), &cr)
		Expect(err).NotTo(HaveOccurred())

		By("Verify status is not ready")
		Eventually(func() bool {
			mvn := &mellanoxv1alpha1.MacvlanNetwork{}
			err := k8sClient.Get(goctx.TODO(), types.NamespacedName{Name: crName}, mvn)
			Expect(err).NotTo(HaveOccurred())
			return mellanoxv1alpha1.StateNotReady == mvn.Status.State && mvn.Status.MacvlanNetworkAttachmentDef == ""
		}, timeout*30, interval).Should(BeTrue())
	})
})

func verifyNADCreated(name, namespace string) {
	netAttachDef := &netattdefv1.NetworkAttachmentDefinition{}
	Eventually(func() error {
		return k8sClient.Get(goctx.TODO(),
			types.NamespacedName{Namespace: namespace, Name: name},
			netAttachDef)
	}, timeout*30, interval).ShouldNot(HaveOccurred())
}

func verifyNetworkNamespaceAnnot(namespace string) {
	Eventually(func() bool {
		mvn := &mellanoxv1alpha1.MacvlanNetwork{}
		err := k8sClient.Get(goctx.TODO(), types.NamespacedName{Namespace: crNamespace, Name: crName}, mvn)
		Expect(err).NotTo(HaveOccurred())
		annot, ok := mvn.Annotations[lastNetworkNamespaceAnnot]
		return ok && annot == namespace
	}, timeout*30, interval).Should(BeTrue())
}

func verifyMacvlanStatus(namespace string) {
	Eventually(func() bool {
		mvn := &mellanoxv1alpha1.MacvlanNetwork{}
		err := k8sClient.Get(goctx.TODO(), types.NamespacedName{Name: crName}, mvn)
		Expect(err).NotTo(HaveOccurred())
		nadState := fmt.Sprintf("k8s.cni.cncf.io/v1/namespaces/%s/NetworkAttachmentDefinition/test", namespace)
		return mellanoxv1alpha1.StateReady == mvn.Status.State && nadState == mvn.Status.MacvlanNetworkAttachmentDef
	}, timeout*30, interval).Should(BeTrue())
}
