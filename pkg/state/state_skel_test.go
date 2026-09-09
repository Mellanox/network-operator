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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	osconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
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

	Context("OpenShift cluster-wide proxy", func() {
		var clusterProxy *osconfigv1.Proxy

		BeforeEach(func() {
			clusterProxy = &osconfigv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: osconfigv1.ProxySpec{
					HTTPProxy:  testClusterWideHTTPProxy,
					HTTPSProxy: testClusterWideHTTPSProxy,
					NoProxy:    testClusterWideNoProxy,
				},
			}
		})

		DescribeTable("setEnvFromClusterWideProxy applies to OFED and NIC Configuration Operator env",
			func(getEnv func(*mellanoxv1alpha1.NicClusterPolicy) []corev1.EnvVar,
				setEnv func(*mellanoxv1alpha1.NicClusterPolicy, []corev1.EnvVar)) {
				cr := &mellanoxv1alpha1.NicClusterPolicy{
					Spec: mellanoxv1alpha1.NicClusterPolicySpec{
						OFEDDriver:               &mellanoxv1alpha1.OFEDDriverSpec{},
						NicConfigurationOperator: &mellanoxv1alpha1.NicConfigurationOperatorSpec{},
					},
				}
				setEnv(cr, s.setEnvFromClusterWideProxy(getEnv(cr), clusterProxy))
				Expect(getEnv(cr)).To(HaveLen(6))
				for _, expected := range expectedClusterWideProxyEnv() {
					Expect(getEnv(cr)).To(ContainElement(expected))
				}
			},
			Entry("OFED driver env",
				func(cr *mellanoxv1alpha1.NicClusterPolicy) []corev1.EnvVar {
					return cr.Spec.OFEDDriver.Env
				},
				func(cr *mellanoxv1alpha1.NicClusterPolicy, env []corev1.EnvVar) {
					cr.Spec.OFEDDriver.Env = env
				},
			),
			Entry("NIC Configuration Operator env",
				func(cr *mellanoxv1alpha1.NicClusterPolicy) []corev1.EnvVar {
					return cr.Spec.NicConfigurationOperator.Env
				},
				func(cr *mellanoxv1alpha1.NicClusterPolicy, env []corev1.EnvVar) {
					cr.Spec.NicConfigurationOperator.Env = env
				},
			),
		)

		It("setEnvFromClusterWideProxy keeps existing env and fills only missing proxy vars", func() {
			env := []corev1.EnvVar{
				{Name: envVarNameNoProxy, Value: testNicPolicyNoProxy},
				{Name: strings.ToLower(envVarNameHTTPProxy), Value: testNicPolicyHTTPProxy},
			}
			env = s.setEnvFromClusterWideProxy(env, clusterProxy)
			Expect(env).To(ContainElements(
				corev1.EnvVar{Name: envVarNameNoProxy, Value: testNicPolicyNoProxy},
				corev1.EnvVar{Name: strings.ToLower(envVarNameHTTPProxy), Value: testNicPolicyHTTPProxy},
				corev1.EnvVar{Name: envVarNameHTTPSProxy, Value: testClusterWideHTTPSProxy},
				corev1.EnvVar{Name: strings.ToLower(envVarNameHTTPSProxy), Value: testClusterWideHTTPSProxy},
			))
			Expect(env).NotTo(ContainElement(corev1.EnvVar{
				Name: envVarNameHTTPProxy, Value: testClusterWideHTTPProxy,
			}))
		})

		It("readOpenshiftProxyConfig returns the cluster Proxy object", func() {
			s.client = fake.NewClientBuilder().WithScheme(openshiftProxyScheme()).WithObjects(clusterProxy).Build()
			got, err := s.readOpenshiftProxyConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).NotTo(BeNil())
			Expect(got.Spec.HTTPProxy).To(Equal(testClusterWideHTTPProxy))
			Expect(got.Spec.HTTPSProxy).To(Equal(testClusterWideHTTPSProxy))
			Expect(got.Spec.NoProxy).To(Equal(testClusterWideNoProxy))
		})

		It("readOpenshiftProxyConfig returns nil when the Proxy object is missing", func() {
			s.client = fake.NewClientBuilder().WithScheme(openshiftProxyScheme()).Build()
			got, err := s.readOpenshiftProxyConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeNil())
		})

		It("readOpenshiftProxyConfig returns nil when the Proxy API is not registered", func() {
			s.client = noKindMatchGetClient{}
			got, err := s.readOpenshiftProxyConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeNil())
		})

		DescribeTable("handleOpenshiftClusterWideProxyConfig injects env for OFED and NIC Configuration Operator",
			func(getEnv func(*mellanoxv1alpha1.NicClusterPolicy) []corev1.EnvVar,
				setEnv func(*mellanoxv1alpha1.NicClusterPolicy, []corev1.EnvVar)) {
				s.client = fake.NewClientBuilder().WithScheme(openshiftProxyScheme()).WithObjects(clusterProxy).Build()
				cr := &mellanoxv1alpha1.NicClusterPolicy{
					Spec: mellanoxv1alpha1.NicClusterPolicySpec{
						OFEDDriver:               &mellanoxv1alpha1.OFEDDriverSpec{},
						NicConfigurationOperator: &mellanoxv1alpha1.NicConfigurationOperatorSpec{},
					},
				}
				env, _, err := s.handleOpenshiftClusterWideProxyConfig(ctx, cr, getEnv(cr), nil)
				Expect(err).NotTo(HaveOccurred())
				setEnv(cr, env)
				for _, expected := range expectedClusterWideProxyEnv() {
					Expect(getEnv(cr)).To(ContainElement(expected))
				}
			},
			Entry("OFED driver env",
				func(cr *mellanoxv1alpha1.NicClusterPolicy) []corev1.EnvVar {
					return cr.Spec.OFEDDriver.Env
				},
				func(cr *mellanoxv1alpha1.NicClusterPolicy, env []corev1.EnvVar) {
					cr.Spec.OFEDDriver.Env = env
				},
			),
			Entry("NIC Configuration Operator env",
				func(cr *mellanoxv1alpha1.NicClusterPolicy) []corev1.EnvVar {
					return cr.Spec.NicConfigurationOperator.Env
				},
				func(cr *mellanoxv1alpha1.NicClusterPolicy, env []corev1.EnvVar) {
					cr.Spec.NicConfigurationOperator.Env = env
				},
			),
		)

		It("handleOpenshiftClusterWideProxyConfig leaves env unchanged when Proxy is absent", func() {
			s.client = fake.NewClientBuilder().WithScheme(openshiftProxyScheme()).Build()
			existing := []corev1.EnvVar{{Name: "KEEP", Value: "me"}}
			env, certConfig, err := s.handleOpenshiftClusterWideProxyConfig(ctx, nil, existing, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(env).To(Equal(existing))
			Expect(certConfig).To(BeNil())
		})

		It("handleOpenshiftTrustedCA keeps admin certConfig", func() {
			existing := &mellanoxv1alpha1.ConfigMapNameReference{Name: "admin-ca"}
			got, err := s.handleOpenshiftTrustedCA(ctx, nil, clusterProxy, existing)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(existing))
		})

		It("handleOpenshiftTrustedCA skips when cluster TrustedCA is unset", func() {
			clusterProxy.Spec.TrustedCA.Name = ""
			got, err := s.handleOpenshiftTrustedCA(ctx, nil, clusterProxy, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeNil())
		})
	})
})

var _ = Describe("SetConfigHashAnnotation", func() {
	Context("when configHash is empty", func() {
		It("should not modify any objects", func() {
			ds := createTestDaemonSet("test-ds")
			objs := []*unstructured.Unstructured{ds}
			err := SetConfigHashAnnotation(objs, "")
			Expect(err).NotTo(HaveOccurred())

			annotations, found, err := unstructured.NestedStringMap(ds.Object,
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
			Expect(annotations).To(BeNil())
		})
	})

	Context("when configHash is provided", func() {
		It("should add annotation to DaemonSet pod template", func() {
			ds := createTestDaemonSet("test-ds")
			objs := []*unstructured.Unstructured{ds}
			configHash := "abc123hash"

			err := SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			annotations, found, err := unstructured.NestedStringMap(ds.Object,
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(annotations[consts.ConfigHashAnnotation]).To(Equal(configHash))
		})

		It("should preserve existing annotations", func() {
			ds := createTestDaemonSet("test-ds")
			// Add existing annotation
			err := unstructured.SetNestedStringMap(ds.Object,
				map[string]string{"existing": "annotation"},
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())

			objs := []*unstructured.Unstructured{ds}
			configHash := "abc123hash"

			err = SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			annotations, found, err := unstructured.NestedStringMap(ds.Object,
				"spec", "template", "metadata", "annotations")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(annotations["existing"]).To(Equal("annotation"))
			Expect(annotations[consts.ConfigHashAnnotation]).To(Equal(configHash))
		})

		It("should not modify non-DaemonSet objects", func() {
			configMap := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-cm",
						"namespace": "test-ns",
					},
				},
			}
			objs := []*unstructured.Unstructured{configMap}
			configHash := "abc123hash"

			err := SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			// ConfigMap should not have the annotation path
			_, found, _ := unstructured.NestedStringMap(configMap.Object,
				"spec", "template", "metadata", "annotations")
			Expect(found).To(BeFalse())
		})

		It("should handle multiple DaemonSets", func() {
			ds1 := createTestDaemonSet("test-ds-1")
			ds2 := createTestDaemonSet("test-ds-2")
			objs := []*unstructured.Unstructured{ds1, ds2}
			configHash := "abc123hash"

			err := SetConfigHashAnnotation(objs, configHash)
			Expect(err).NotTo(HaveOccurred())

			for _, ds := range objs {
				annotations, found, err := unstructured.NestedStringMap(ds.Object,
					"spec", "template", "metadata", "annotations")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(annotations[consts.ConfigHashAnnotation]).To(Equal(configHash))
			}
		})
	})
})

func expectedClusterWideProxyEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: envVarNameNoProxy, Value: testClusterWideNoProxy},
		{Name: envVarNameHTTPProxy, Value: testClusterWideHTTPProxy},
		{Name: envVarNameHTTPSProxy, Value: testClusterWideHTTPSProxy},
		{Name: strings.ToLower(envVarNameNoProxy), Value: testClusterWideNoProxy},
		{Name: strings.ToLower(envVarNameHTTPProxy), Value: testClusterWideHTTPProxy},
		{Name: strings.ToLower(envVarNameHTTPSProxy), Value: testClusterWideHTTPSProxy},
	}
}

func openshiftProxyScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(osconfigv1.AddToScheme(scheme)).NotTo(HaveOccurred())
	return scheme
}

type noKindMatchGetClient struct {
	client.Client
}

func (c noKindMatchGetClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return &meta.NoKindMatchError{
		GroupKind: schema.GroupKind{Group: "config.openshift.io", Kind: "Proxy"},
	}
}

func createTestDaemonSet(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": name,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": name,
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test-container",
								"image": "test-image",
							},
						},
					},
				},
			},
		},
	}
}
