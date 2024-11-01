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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CRD Application", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		Expect(testClient.ApiextensionsV1().CustomResourceDefinitions().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).NotTo(HaveOccurred())
	})

	Describe("applyCRDsFromFile", func() {
		It("should apply CRDs multiple times from a valid YAML file", func() {
			By("applying CRDs")
			Expect(applyCRDsFromFile(ctx, testClient, "test-files/test-crds.yaml")).To(Succeed())
			Expect(applyCRDsFromFile(ctx, testClient, "test-files/test-crds.yaml")).To(Succeed())
			Expect(applyCRDsFromFile(ctx, testClient, "test-files/test-crds.yaml")).To(Succeed())
			Expect(applyCRDsFromFile(ctx, testClient, "test-files/test-crds.yaml")).To(Succeed())
			
			By("verifying CRDs are applied")
			crds, err := testClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(crds.Items).To(HaveLen(2))
		})

		It("should update CRDs", func() {
			By("applying CRDs")
			Expect(applyCRDsFromFile(ctx, testClient, "test-files/test-crds.yaml")).To(Succeed())

			By("verifying CRDs do not have spec.foobar")
			for _, crdName := range []string{"bars.example.com", "foos.example.com"} {
				crd, err := testClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				props := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties
				Expect(props).To(HaveKey("spec"))
				Expect(props["spec"].Properties).NotTo(HaveKey("foobar"))
			}

			By("updating CRDs")
			Expect(applyCRDsFromFile(ctx, testClient, "test-files/updated-test-crds.yaml")).To(Succeed())

			By("verifying CRDs are updated")
			for _, crdName := range []string{"bars.example.com", "foos.example.com"} {
				crd, err := testClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				props := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties
				Expect(props["spec"].Properties).To(HaveKey("foobar"))
			}
		})
	})
})
