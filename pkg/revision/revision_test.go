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

package revision_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mellanox/network-operator/pkg/revision"
)

var _ = Describe("Revision", func() {
	Context("CalculateRevision", func() {
		It("Should be equal for same objects", func() {
			o := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "obj1", Namespace: "ns1"}}
			rev1, err := revision.CalculateRevision(o)
			Expect(err).NotTo(HaveOccurred())
			rev2, err := revision.CalculateRevision(o)
			Expect(err).NotTo(HaveOccurred())
			Expect(rev1).To(Equal(rev2))
		})
		It("Should not be equal for different objects", func() {
			o1 := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "obj1", Namespace: "ns1"}}
			o2 := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "obj2", Namespace: "ns2"}}
			rev1, err := revision.CalculateRevision(o1)
			Expect(err).NotTo(HaveOccurred())
			rev2, err := revision.CalculateRevision(o2)
			Expect(err).NotTo(HaveOccurred())
			Expect(rev1).NotTo(Equal(rev2))
		})
	})
	Context("Set/Get Revision", func() {
		It("Should get revision set by setter", func() {
			o := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "obj1", Namespace: "ns1"}}
			testRev := revision.ControllerRevision(1000)
			revision.SetRevision(o, testRev)
			Expect(revision.GetRevision(o)).To(Equal(testRev))
		})
		It("Should return zero if revision not set", func() {
			o := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "obj1", Namespace: "ns1"}}
			Expect(revision.GetRevision(o)).To(Equal(revision.ControllerRevision(0)))
		})
	})
})
