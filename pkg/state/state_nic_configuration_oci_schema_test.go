/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xeipuuv/gojsonschema"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("NIC firmware OCI schema integration", func() {
	DescribeTable("ships matching OCI settings in every installation schema", func(filename string) {
		canonical := nicConfigurationSchema("../../config/crd/bases/mellanox.com_nicclusterpolicies.yaml")
		shipped := nicConfigurationSchema(filename)
		for _, field := range []string{"firmwareImageCacheSize", "firmwareImagePullSecrets"} {
			Expect(canonical.Properties).To(HaveKey(field))
			Expect(shipped.Properties).To(HaveKey(field))
			Expect(shipped.Properties[field]).To(Equal(canonical.Properties[field]), "field=%s", field)
		}
	}, Entry("Helm", "../../deployment/network-operator/crds/mellanox.com_nicclusterpolicies.yaml"),
		Entry("OLM", "../../bundle/manifests/mellanox.com_nicclusterpolicies.yaml"))

	It("ships matching firmware schemas for NicDevice and NicFirmwareTemplate", func() {
		device := firmwareSchema("002-configuration.net.nvidia.com_nicdevices.yaml", "firmware")
		template := firmwareSchema("004-configuration.net.nvidia.com_nicfirmwaretemplates.yaml", "template")
		Expect(device.Properties).To(Equal(template.Properties))
		Expect(device.Required).To(Equal(template.Required))
		Expect(device.XValidations).To(Equal(template.XValidations))
		Expect(device.Required).To(ConsistOf("updatePolicy"))
		Expect(device.XValidations).To(ConsistOf(apiextensionsv1.ValidationRule{
			Rule: "(has(self.nicFirmwareSourceRef) && size(self.nicFirmwareSourceRef) > 0) != " +
				"(has(self.firmwareImages) && size(self.firmwareImages) > 0)",
			Message: "Exactly one of nicFirmwareSourceRef or firmwareImages must be specified",
		}))
	})

	DescribeTable("validates explicit firmware credential Secret names", func(name string, valid bool) {
		configuration := nicConfigurationSchema("../../config/crd/bases/mellanox.com_nicclusterpolicies.yaml")
		schema := configuration.Properties["firmwareImagePullSecrets"]
		Expect(*schema.XListType).To(Equal("set"))
		Expect(*schema.MaxItems).To(Equal(int64(32)))
		result, err := gojsonschema.Validate(gojsonschema.NewGoLoader(schema),
			gojsonschema.NewGoLoader([]string{name}))
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid()).To(Equal(valid), "errors=%v", result.Errors())
	}, Entry("DNS subdomain", "harbor.credentials", true), Entry("traversal", "../credentials", false),
		Entry("empty", "", false), Entry("invalid separator", "harbor_credentials", false))

	DescribeTable("validates the shipped image reference schema", func(images []interface{}, valid bool) {
		for _, location := range []struct{ file, field string }{
			{"002-configuration.net.nvidia.com_nicdevices.yaml", "firmware"},
			{"004-configuration.net.nvidia.com_nicfirmwaretemplates.yaml", "template"},
		} {
			schema := firmwareSchema(location.file, location.field)
			result, err := gojsonschema.Validate(gojsonschema.NewGoLoader(schema),
				gojsonschema.NewGoLoader(map[string]interface{}{"firmwareImages": images, "updatePolicy": "Validate"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Valid()).To(Equal(valid), "schema=%s errors=%v", location.file, result.Errors())
		}
	}, Entry("digest-pinned image", []interface{}{"registry.example/fw/cx7@sha256:" + strings.Repeat("a", 64)}, true),
		Entry("readable tag plus digest",
			[]interface{}{"registry.example/fw/cx7:PSID_1-28.40@sha256:" + strings.Repeat("a", 64)}, true),
		Entry("mutable tag", []interface{}{"registry.example/fw/cx7:latest"}, false),
		Entry("empty list", []interface{}{}, false),
		Entry("invalid digest", []interface{}{"registry.example/fw/cx7@sha256:bad"}, false))
})

func nicConfigurationSchema(filename string) apiextensionsv1.JSONSchemaProps {
	data, err := os.ReadFile(filename)
	Expect(err).NotTo(HaveOccurred())
	crd := &apiextensionsv1.CustomResourceDefinition{}
	Expect(yaml.UnmarshalStrict(data, crd)).To(Succeed())
	for _, version := range crd.Spec.Versions {
		if version.Name != "v1alpha1" || !version.Served {
			continue
		}
		Expect(version.Schema).NotTo(BeNil())
		Expect(version.Schema.OpenAPIV3Schema).NotTo(BeNil())
		spec := version.Schema.OpenAPIV3Schema.Properties["spec"]
		Expect(spec.Properties).To(HaveKey("nicConfigurationOperator"))
		return spec.Properties["nicConfigurationOperator"]
	}
	Fail("CRD has no served v1alpha1 schema: " + filename)
	return apiextensionsv1.JSONSchemaProps{}
}

func firmwareSchema(filename, name string) apiextensionsv1.JSONSchemaProps {
	data, err := os.ReadFile("../../manifests/state-nic-configuration-operator/" + filename)
	Expect(err).NotTo(HaveOccurred())
	crd := &apiextensionsv1.CustomResourceDefinition{}
	Expect(yaml.UnmarshalStrict(data, crd)).To(Succeed())
	Expect(crd.Spec.Versions).To(HaveLen(1))
	return crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties[name]
}
