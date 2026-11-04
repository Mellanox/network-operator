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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	osconfigv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("NIC Configuration Operator OCI deployment", func() {
	var ts testScope
	BeforeEach(func() {
		ts = ts.New(state.NewStateNicConfigurationOperator, "../../manifests/state-nic-configuration-operator")
	})

	DescribeTable("renders an ephemeral cache with optional legacy storage", func(storage bool) {
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		if storage {
			cr.Spec.NicConfigurationOperator.NicFirmwareStorage = &mellanoxv1alpha1.NicFirmwareStorageSpec{
				Create: true, PVCName: "legacy", AvailableStorageSize: "1Gi", AccessMode: "ReadWriteMany"}
		}
		ds, manager := renderOCIDeployment(&ts, cr)
		Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(ds.Spec.Template.Spec.InitContainers).To(BeEmpty())
		Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
			Name: "FIRMWARE_IMAGE_CACHE_DIR", Value: "/nic-firmware-images"}))
		Expect(ds.Spec.Template.Spec.Containers[0].Env).NotTo(ContainElement(HaveField("Name", "FIRMWARE_IMAGE_AUTH_DIR")))
		Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
			Name: "firmware-images", MountPath: "/nic-firmware-images"}))
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			if volume.Name == "firmware-images" {
				Expect(volume.EmptyDir).NotTo(BeNil())
				Expect(volume.EmptyDir.SizeLimit.String()).To(Equal("4Gi"))
			}
			if !storage {
				Expect(volume.PersistentVolumeClaim).To(BeNil())
			}
		}
		Expect(manager.Spec.Template.Spec.Containers[0].Env).NotTo(ContainElement(
			HaveField("Name", "FIRMWARE_IMAGE_CACHE_DIR")))
	}, Entry("without a PVC", false), Entry("with the existing firmware PVC", true))

	DescribeTable("uses the same validated size for cache budget and storage", func(size string, valid bool) {
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		cr.Spec.NicConfigurationOperator.FirmwareImageCacheSize = size
		if !valid {
			_, err := ts.renderer.GetManifestObjects(context.Background(), cr, ts.catalog, log.Log)
			Expect(err).To(MatchError(ContainSubstring("nicConfigurationOperator.firmwareImageCacheSize")))
			return
		}
		ds, _ := renderOCIDeployment(&ts, cr)
		var expected string
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			if volume.Name == "firmware-images" {
				expected = volume.EmptyDir.SizeLimit.String()
				requested := size
				if requested == "" {
					requested = "4Gi"
				}
				Expect(volume.EmptyDir.SizeLimit.Cmp(resource.MustParse(requested))).To(BeZero())
			}
		}
		Expect(expected).NotTo(BeEmpty())
		Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
			Name: "FIRMWARE_IMAGE_CACHE_SIZE", Value: expected}))
		Expect(cr.Spec.NicConfigurationOperator.FirmwareImageCacheSize).To(Equal(size))
	}, Entry("default", "", true), Entry("minimum", "2Gi", true), Entry("larger cache", "18Gi", true),
		Entry("fractional quantity", "2.5Gi", true), Entry("decimal bytes", "3000000000", true),
		Entry("saturated binary quantity", "10Ei", true), Entry("decimal exponent overflow", "1e30", false),
		Entry("small cache", "1Gi", false), Entry("zero", "0", false), Entry("negative", "-4Gi", false),
		Entry("malformed", "invalid", false), Entry("int64 overflow", "9223372036854775808", false))

	It("exposes only explicitly selected firmware credentials to the daemon", func() {
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		cr.Spec.NicConfigurationOperator.FirmwareImagePullSecrets = []string{"firmware.one", "firmware-two"}
		cr.Spec.NicConfigurationOperator.ConfigurationDaemon.ImagePullSecrets = []string{"component-only"}
		ds, manager := renderOCIDeployment(&ts, cr)
		Expect(ds.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "component-only"}))
		Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
			Name: "FIRMWARE_IMAGE_AUTH_DIR", Value: "/var/run/nic-firmware-auth"}))
		for index, name := range cr.Spec.NicConfigurationOperator.FirmwareImagePullSecrets {
			Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      []string{"firmware-auth-0", "firmware-auth-1"}[index],
				MountPath: "/var/run/nic-firmware-auth/" + name, ReadOnly: true}))
		}
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			if volume.Secret == nil {
				continue
			}
			Expect(volume.Secret.Optional).NotTo(BeNil())
			Expect(*volume.Secret.Optional).To(BeTrue())
			Expect(cr.Spec.NicConfigurationOperator.FirmwareImagePullSecrets).To(ContainElement(volume.Secret.SecretName))
		}
		for _, volume := range manager.Spec.Template.Spec.Volumes {
			Expect(volume.Secret).To(BeNil())
		}
	})

	It("does not project component or global image pull credentials implicitly", func() {
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		cr.Spec.NicConfigurationOperator.ConfigurationDaemon.ImagePullSecrets = nil
		cr.Spec.Global = &mellanoxv1alpha1.GlobalConfig{ImagePullSecrets: []string{"global-only"}}
		ds, _ := renderOCIDeployment(&ts, cr)
		Expect(ds.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "global-only"}))
		for _, volume := range ds.Spec.Template.Spec.Volumes {
			Expect(volume.Secret).To(BeNil())
		}
	})

	DescribeTable("preserves YAML-ambiguous firmware Secret names as strings", func(name string) {
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		cr.Spec.NicConfigurationOperator.FirmwareImagePullSecrets = []string{name}
		ds, _ := renderOCIDeployment(&ts, cr)
		Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(And(
			HaveField("Name", "firmware-auth-0"),
			HaveField("Secret.SecretName", name),
		)))
		Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
			Name: "firmware-auth-0", MountPath: "/var/run/nic-firmware-auth/" + name, ReadOnly: true,
		}))
	}, Entry("integer", "123"), Entry("boolean true", "true"), Entry("boolean false", "false"),
		Entry("null", "null"), Entry("exponent", "1e3"), Entry("decimal", "1.2"),
		Entry("leading zero", "0123"), Entry("YAML boolean", "on"))

	DescribeTable("preserves administrator TLS settings without duplicate env entries", func(customCert bool) {
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		cr.Spec.NicConfigurationOperator.CertConfig = &mellanoxv1alpha1.ConfigMapNameReference{Name: "registry-ca"}
		cr.Spec.NicConfigurationOperator.Env = []corev1.EnvVar{{Name: "HTTPS_PROXY", Value: "http://proxy:3128"}}
		expected := "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
		if customCert {
			expected = "/custom/ca.pem"
			cr.Spec.NicConfigurationOperator.Env = append(cr.Spec.NicConfigurationOperator.Env,
				corev1.EnvVar{Name: "SSL_CERT_FILE", Value: expected})
		}
		original := cr.Spec.NicConfigurationOperator.DeepCopy().Env
		ds, _ := renderOCIDeployment(&ts, cr)
		Expect(cr.Spec.NicConfigurationOperator.Env).To(Equal(original))
		var certificates []corev1.EnvVar
		for _, variable := range ds.Spec.Template.Spec.Containers[0].Env {
			if variable.Name == "SSL_CERT_FILE" {
				certificates = append(certificates, variable)
			}
		}
		if customCert {
			Expect(certificates).To(ConsistOf(corev1.EnvVar{Name: "SSL_CERT_FILE", Value: expected}))
		} else {
			Expect(certificates).To(BeEmpty())
		}
		Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
			Name: "FIRMWARE_IMAGE_CA_FILE", Value: "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"}))
		Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ContainElement(original[0]))
		Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
			Name: "trusted-ca", MountPath: "/etc/pki/ca-trust/extracted/pem", ReadOnly: true}))
	}, Entry("component CA bundle", false), Entry("explicit TLS path", true))

	DescribeTable("rejects conflicting deployment-owned paths", func(name string) {
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		cr.Spec.NicConfigurationOperator.Env = []corev1.EnvVar{{Name: name, Value: "/override"}}
		_, err := ts.renderer.GetManifestObjects(context.Background(), cr, ts.catalog, log.Log)
		Expect(err).To(MatchError(ContainSubstring(name + " is managed by the operator")))
	}, Entry("cache directory", "FIRMWARE_IMAGE_CACHE_DIR"), Entry("credential directory", "FIRMWARE_IMAGE_AUTH_DIR"),
		Entry("CA file", "FIRMWARE_IMAGE_CA_FILE"), Entry("cache budget", "FIRMWARE_IMAGE_CACHE_SIZE"))

	It("propagates OpenShift proxy and trusted CA configuration into the daemon", func() {
		Expect(osconfigv1.AddToScheme(ts.client.Scheme())).To(Succeed())
		Expect(ts.client.Create(context.Background(), newNodeMaintenanceCRD())).To(Succeed())
		proxy := &osconfigv1.Proxy{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}, Spec: osconfigv1.ProxySpec{
			HTTPSProxy: "http://proxy:3128", NoProxy: "harbor.internal",
			TrustedCA: osconfigv1.ConfigMapNameReference{Name: "cluster-ca"}}}
		Expect(ts.client.Create(context.Background(), proxy)).To(Succeed())
		cr := getMinimalNicClusterPolicyWithNicConfigurationOperator("operator", "daemon")
		_, err := ts.state.Sync(context.Background(), cr, ts.openshiftCatalog)
		Expect(err).NotTo(HaveOccurred())
		ds := &appsv1.DaemonSet{}
		Expect(ts.client.Get(context.Background(), types.NamespacedName{
			Name: "nic-configuration-daemon", Namespace: ts.namespace}, ds)).To(Succeed())
		Expect(ds.Spec.Template.Spec.Containers[0].Env).To(ContainElements(
			corev1.EnvVar{Name: "HTTPS_PROXY", Value: "http://proxy:3128"},
			corev1.EnvVar{Name: "NO_PROXY", Value: "harbor.internal"},
			corev1.EnvVar{Name: "FIRMWARE_IMAGE_CA_FILE", Value: "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"}))
		Expect(cr.Spec.NicConfigurationOperator.CertConfig).NotTo(BeNil())
	})
})

func renderOCIDeployment(ts *testScope, cr *mellanoxv1alpha1.NicClusterPolicy) (*appsv1.DaemonSet, *appsv1.Deployment) {
	objs, err := ts.renderer.GetManifestObjects(context.Background(), cr, ts.catalog, log.Log)
	Expect(err).NotTo(HaveOccurred())
	ds := &appsv1.DaemonSet{}
	manager := &appsv1.Deployment{}
	for _, obj := range objs {
		Expect(obj.GetKind()).NotTo(BeElementOf("MutatingWebhookConfiguration", "Certificate", "Issuer"))
		if obj.GetKind() == "DaemonSet" {
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ds)).To(Succeed())
		}
		if obj.GetKind() == "Deployment" {
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, manager)).To(Succeed())
		}
	}
	return ds, manager
}
