/*
2026 NVIDIA CORPORATION & AFFILIATES

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/state"
)

var _ = Describe("IB Kubernetes state sync tests", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateIBKubernetes, "../../manifests/state-ib-kubernetes")
		Expect(ts).NotTo(BeNil())
	})

	Context("Verify objects rendering and creation via Sync", func() {
		It("should create base 4 objects with env vars and UFM secret", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()

			By("Sync")
			dep := syncAndGetIBKubernetesDeployment(&ts, cr)

			By("Verify ServiceAccount, ClusterRole, ClusterRoleBinding are created")
			sa := &corev1.ServiceAccount{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "ib-kubernetes"}, sa)).NotTo(HaveOccurred())

			clusterRole := &rbacv1.ClusterRole{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Name: "ib-kubernetes"}, clusterRole)).NotTo(HaveOccurred())

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Name: "ib-kubernetes"}, clusterRoleBinding)).NotTo(HaveOccurred())

			By("Verify Deployment container image, env vars, and UFM secret")
			Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := dep.Spec.Template.Spec.Containers[0]
			Expect(container.Image).To(Equal(defaultTestRepository + "/" + defaultTestImage + ":" + defaultTestVersion))

			expectedEnvValues := map[string]string{
				"DAEMON_SM_PLUGIN":       "ufm",
				"DAEMON_PERIODIC_UPDATE": "6",
				"GUID_POOL_RANGE_START":  "rangestart",
				"GUID_POOL_RANGE_END":    "rangeend",
			}
			expectedSecretVars := map[string]struct{}{
				"UFM_USERNAME":    {},
				"UFM_PASSWORD":    {},
				"UFM_ADDRESS":     {},
				"UFM_HTTP_SCHEMA": {},
				"UFM_PORT":        {},
				"UFM_CERTIFICATE": {},
			}
			for _, envVar := range container.Env {
				if expectedVal, ok := expectedEnvValues[envVar.Name]; ok {
					Expect(envVar.Value).To(Equal(expectedVal))
					delete(expectedEnvValues, envVar.Name)
				}
				if _, ok := expectedSecretVars[envVar.Name]; ok {
					Expect(envVar.ValueFrom).NotTo(BeNil())
					Expect(envVar.ValueFrom.SecretKeyRef).NotTo(BeNil())
					Expect(envVar.ValueFrom.SecretKeyRef.Name).To(Equal("secret"))
					delete(expectedSecretVars, envVar.Name)
				}
			}
			Expect(expectedEnvValues).To(BeEmpty(), "missing env vars: %v", expectedEnvValues)
			Expect(expectedSecretVars).To(BeEmpty(), "missing UFM secret env vars: %v", expectedSecretVars)
		})

		It("should render fieldRef env vars", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))

			expectedFieldRefs := map[string]string{
				"K8S_NODE":      "spec.nodeName",
				"POD_NAMESPACE": "metadata.namespace",
				"POD_UID":       "metadata.uid",
			}
			for _, envVar := range dep.Spec.Template.Spec.Containers[0].Env {
				if expectedPath, ok := expectedFieldRefs[envVar.Name]; ok {
					Expect(envVar.ValueFrom).NotTo(BeNil())
					Expect(envVar.ValueFrom.FieldRef).NotTo(BeNil())
					Expect(envVar.ValueFrom.FieldRef.FieldPath).To(Equal(expectedPath), "fieldPath for %s", envVar.Name)
					delete(expectedFieldRefs, envVar.Name)
				}
			}
			Expect(expectedFieldRefs).To(BeEmpty(), "missing fieldRef env vars: %v", expectedFieldRefs)
		})

		It("should render ContainerResources", func() {
			quantity := resource.MustParse("150Mi")
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{{
				Name:     "ib-kubernetes",
				Requests: corev1.ResourceList{corev1.ResourceCPU: quantity},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: quantity},
			}}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			resources := dep.Spec.Template.Spec.Containers[0].Resources
			Expect(resources.Requests.Cpu().String()).To(Equal(quantity.String()))
			Expect(resources.Limits.Cpu().String()).To(Equal(quantity.String()))
		})

		It("should NOT render ContainerResources with the wrong container name", func() {
			quantity := resource.MustParse("150Mi")
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{{
				Name:     "ib-kubernetes-wrong",
				Requests: corev1.ResourceList{corev1.ResourceCPU: quantity},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: quantity},
			}}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			resources := dep.Spec.Template.Spec.Containers[0].Resources
			Expect(resources.Requests).To(BeEmpty())
			Expect(resources.Limits).To(BeEmpty())
		})

		It("should not render limits if nil", func() {
			quantity := resource.MustParse("150Mi")
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{{
				Name:     "ib-kubernetes",
				Requests: corev1.ResourceList{corev1.ResourceCPU: quantity},
			}}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			resources := dep.Spec.Template.Spec.Containers[0].Resources
			Expect(resources.Requests.Cpu().String()).To(Equal(quantity.String()))
			Expect(resources.Limits).To(BeEmpty())
		})

		It("should have image in SHA256 format", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.Version = defaultTestVersionSha256

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal(getImagePathWithSha256()))
		})

		It("should render DeploymentNodeAffinity", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.DeploymentNodeAffinity = &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "custom-label",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"value1", "value2"},
								},
							},
						},
					},
				},
			}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			nodeAffinity := dep.Spec.Template.Spec.Affinity.NodeAffinity
			Expect(nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(HaveLen(2))
			Expect(nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).NotTo(BeNil())

			terms := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
			Expect(terms).To(HaveLen(1))
			Expect(terms[0].MatchExpressions).To(HaveLen(1))
			expr := terms[0].MatchExpressions[0]
			Expect(expr.Key).To(Equal("custom-label"))
			Expect(expr.Operator).To(Equal(corev1.NodeSelectorOpIn))
			Expect(expr.Values).To(ContainElements("value1", "value2"))
		})

		It("should render DeploymentTolerations", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.DeploymentTolerations = []corev1.Toleration{
				{
					Key:      "custom-taint",
					Operator: corev1.TolerationOpEqual,
					Value:    "custom-value",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "another-taint",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectPreferNoSchedule,
				},
			}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			tolerations := dep.Spec.Template.Spec.Tolerations
			Expect(tolerations).To(HaveLen(5)) // 3 default + 2 custom
			Expect(tolerations).To(ContainElements(
				corev1.Toleration{
					Key:      "custom-taint",
					Operator: corev1.TolerationOpEqual,
					Value:    "custom-value",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				corev1.Toleration{
					Key:      "another-taint",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectPreferNoSchedule,
				},
			))
		})

		It("should render both DeploymentNodeAffinity and DeploymentTolerations together", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.DeploymentNodeAffinity = &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: 10,
						Preference: corev1.NodeSelectorTerm{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "preferred-label",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"preferred-value"},
								},
							},
						},
					},
				},
			}
			cr.Spec.DeploymentTolerations = []corev1.Toleration{
				{
					Key:      "combined-taint",
					Operator: corev1.TolerationOpEqual,
					Value:    "combined-value",
					Effect:   corev1.TaintEffectNoExecute,
				},
			}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			nodeAffinity := dep.Spec.Template.Spec.Affinity.NodeAffinity
			Expect(nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(HaveLen(3)) // 2 default + 1 custom

			tolerations := dep.Spec.Template.Spec.Tolerations
			Expect(tolerations).To(HaveLen(4)) // 3 default + 1 custom
			Expect(tolerations).To(ContainElement(corev1.Toleration{
				Key:      "combined-taint",
				Operator: corev1.TolerationOpEqual,
				Value:    "combined-value",
				Effect:   corev1.TaintEffectNoExecute,
			}))
		})

		It("should render ImagePullSecrets", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.ImagePullSecrets = []string{"secret1", "secret2", "secret3"}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
				corev1.LocalObjectReference{Name: "secret1"},
				corev1.LocalObjectReference{Name: "secret2"},
				corev1.LocalObjectReference{Name: "secret3"},
			))
		})

		It("should NOT render ImagePullSecrets when empty", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.ImagePullSecrets = []string{}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())
		})

		It("should render ImagePullSecrets with other configurations", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.ImagePullSecrets = []string{"my-secret"}
			cr.Spec.DeploymentTolerations = []corev1.Toleration{
				{
					Key:      "test-taint",
					Operator: corev1.TolerationOpEqual,
					Value:    "test-value",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			dep := syncAndGetIBKubernetesDeployment(&ts, cr)
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).To(Equal([]corev1.LocalObjectReference{
				{Name: "my-secret"},
			}))

			tolerations := dep.Spec.Template.Spec.Tolerations
			Expect(tolerations).To(HaveLen(4)) // 3 default + 1 custom
			Expect(tolerations).To(ContainElement(corev1.Toleration{
				Key:      "test-taint",
				Operator: corev1.TolerationOpEqual,
				Value:    "test-value",
				Effect:   corev1.TaintEffectNoSchedule,
			}))
		})
	})

	Context("Verify GetWatchSources", func() {
		It("should return Deployment watch source", func() {
			ws := ts.state.GetWatchSources()
			Expect(ws).To(HaveLen(1))
			Expect(ws).To(HaveKey("Deployment"))
			Expect(ws["Deployment"]).To(BeAssignableToTypeOf(&appsv1.Deployment{}))
		})
	})

	Context("Verify Sync lifecycle, OpenShift, and error paths", func() {
		It("should fail NewStateIBKubernetes when manifestDir does not exist", func() {
			_, _, err := state.NewStateIBKubernetes(ts.client, "/non-existent-manifest-dir")
			Expect(err).To(HaveOccurred())
		})

		It("should create OpenShift Role and RoleBinding when IsOpenshift is true", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()

			status, err := ts.state.Sync(context.Background(), cr, ts.openshiftCatalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))

			role := &rbacv1.Role{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "ib-kubernetes"}, role)).NotTo(HaveOccurred())

			roleBinding := &rbacv1.RoleBinding{}
			Expect(ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "ib-kubernetes"}, roleBinding)).NotTo(HaveOccurred())
		})

		It("should delete objects when IBKubernetes spec is set to nil", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			syncAndGetIBKubernetesDeployment(&ts, cr)

			By("Set IBKubernetes spec to nil and Sync")
			cr.Spec.IBKubernetes = nil
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			By("Verify Deployment is deleted")
			dep := &appsv1.Deployment{}
			err = ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "ib-kubernetes"}, dep)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			By("Sync again when no objects remain")
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateIgnore))
		})

		It("should delete stale objects labeled with state-ib-kubernetes", func() {
			staleCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stale-ib-cm",
					Namespace: ts.namespace,
					Labels: map[string]string{
						consts.StateLabel: ts.state.Name(),
					},
				},
			}
			Expect(ts.client.Create(context.Background(), staleCM)).NotTo(HaveOccurred())

			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))

			err = ts.client.Get(context.Background(),
				types.NamespacedName{Namespace: ts.namespace, Name: "stale-ib-cm"}, &corev1.ConfigMap{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return SyncStateError when ClusterTypeProvider is missing", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			emptyCatalog := state.NewInfoCatalog()

			status, err := ts.state.Sync(context.Background(), cr, emptyCatalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should return SyncStateNotReady and error when ImageSpec is invalid", func() {
			cr := getMinimalNicClusterPolicyWithIBKubernetes()
			cr.Spec.IBKubernetes.Repository = ""

			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})

		It("should return error from GetManifestObjects when spec is nil", func() {
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			_, err := ts.renderer.GetManifestObjects(context.Background(), cr, ts.catalog, testLogger)
			Expect(err).To(HaveOccurred())
		})
	})
})

func syncAndGetIBKubernetesDeployment(ts *testScope, cr *mellanoxv1alpha1.NicClusterPolicy) *appsv1.Deployment {
	status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
	Expect(err).NotTo(HaveOccurred())
	Expect(status).To(BeEquivalentTo(state.SyncStateReady))

	dep := &appsv1.Deployment{}
	err = ts.client.Get(context.Background(),
		types.NamespacedName{Namespace: ts.namespace, Name: "ib-kubernetes"}, dep)
	Expect(err).NotTo(HaveOccurred())
	return dep
}

func getMinimalNicClusterPolicyWithIBKubernetes() *mellanoxv1alpha1.NicClusterPolicy {
	cr := &mellanoxv1alpha1.NicClusterPolicy{}
	cr.Name = "nic-cluster-policy"
	cr.Spec.IBKubernetes = &mellanoxv1alpha1.IBKubernetesSpec{
		ImageSpec: mellanoxv1alpha1.ImageSpec{
			Image:            defaultTestImage,
			Repository:       defaultTestRepository,
			Version:          defaultTestVersion,
			ImagePullSecrets: []string{},
		},
		PeriodicUpdateSeconds:  6,
		PKeyGUIDPoolRangeStart: "rangestart",
		PKeyGUIDPoolRangeEnd:   "rangeend",
		UfmSecret:              "secret",
	}
	return cr
}
