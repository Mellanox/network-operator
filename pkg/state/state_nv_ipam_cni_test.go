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

package state_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/state"
	staticconfig_mocks "github.com/Mellanox/network-operator/pkg/staticconfig/mocks"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("NVIPAM Controller", func() {
	var (
		nvIpamState state.State
		catalog     state.InfoCatalog
		client      client.Client
		namespace   string
		renderer    state.ManifestRenderer
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir := "../../manifests/state-nv-ipam-cni"
		s, r, err := state.NewStateNVIPAMCNI(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		nvIpamState = s
		renderer = r
		catalog = getTestCatalog()
		catalog.Add(state.InfoTypeStaticConfig,
			staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "custom-cni-bin-directory"}))
		namespace = config.FromEnv().State.NetworkOperatorResourceNamespace
	})

	Context("Verify objects rendering", func() {
		It("should create Daemonset - minimal spec", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-node")
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-node"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.NvIpam.ImageSpec, cr)
			assertCNIBinDirForDS(ds)
		})

		It("should create Deployment - minimal spec", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
		})

		It("should create Daemonset - SHA256 image format", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-node")
			cr.Spec.NvIpam.Version = defaultTestVersionSha256
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-node"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.NvIpam.ImageSpec, cr)
		})

		It("should create Deployment - SHA256 image format", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			cr.Spec.NvIpam.Version = defaultTestVersionSha256
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
		})

		It("should create Deployment with Webhook when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			cr.Spec.NvIpam.EnableWebhook = true
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
			By("Verify Webhook")
			assertPodTemplateWebhookFields(&d.Spec.Template)
		})

		It("should create ValidatingWebhookConfiguration when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			cr.Spec.NvIpam.EnableWebhook = true
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify ValidatingWebhookConfiguration")
			adm := &admv1.ValidatingWebhookConfiguration{}
			err = client.Get(context.Background(), types.NamespacedName{Name: "nv-ipam-validating-webhook-configuration"}, adm)
			Expect(err).NotTo(HaveOccurred())
			assertValidatingWebhookFields(adm)
			By("Verify ValidatingWebhookConfiguration Service")
			sv := &v1.Service{}
			err = client.Get(context.Background(),
				types.NamespacedName{Namespace: namespace, Name: "nv-ipam-webhook-service"}, sv)
			Expect(err).NotTo(HaveOccurred())
			Expect(sv.Spec.Selector).Should(ContainElement("nv-ipam-controller"))
		})

		It("should render DeploymentNodeAffinity", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			cr.Spec.DeploymentNodeAffinity = &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "custom-label",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"value1", "value2"},
								},
							},
						},
					},
				},
			}
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
			By("Verify NodeAffinity in Deployment")
			Expect(d.Spec.Template.Spec.Affinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
			// Check that default preferredDuringSchedulingIgnoredDuringExecution is present
			preferred := d.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			Expect(len(preferred)).To(Equal(2)) // 2 default preferred terms
			// Check that custom requiredDuringSchedulingIgnoredDuringExecution is present
			required := d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			Expect(required).NotTo(BeNil())
			nodeSelectorTerms := required.NodeSelectorTerms
			Expect(len(nodeSelectorTerms)).To(Equal(1))
			Expect(len(nodeSelectorTerms[0].MatchExpressions)).To(Equal(1))
			expr := nodeSelectorTerms[0].MatchExpressions[0]
			Expect(expr.Key).To(Equal("custom-label"))
			Expect(expr.Operator).To(Equal(v1.NodeSelectorOpIn))
			Expect(expr.Values).To(ContainElements("value1", "value2"))
		})
		It("should render DeploymentTolerations", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			cr.Spec.DeploymentTolerations = []v1.Toleration{
				{
					Key:      "custom-taint",
					Operator: v1.TolerationOpEqual,
					Value:    "custom-value",
					Effect:   v1.TaintEffectNoSchedule,
				},
				{
					Key:      "another-taint",
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectPreferNoSchedule,
				},
			}
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
			By("Verify Tolerations in Deployment")
			tolerations := d.Spec.Template.Spec.Tolerations
			Expect(len(tolerations)).To(Equal(5)) // 3 default + 2 custom
			// Check that custom tolerations are present
			foundCustomTaint := false
			foundAnotherTaint := false
			for _, tol := range tolerations {
				if tol.Key == "custom-taint" {
					foundCustomTaint = true
					Expect(tol.Operator).To(Equal(v1.TolerationOpEqual))
					Expect(tol.Value).To(Equal("custom-value"))
					Expect(tol.Effect).To(Equal(v1.TaintEffectNoSchedule))
				}
				if tol.Key == "another-taint" {
					foundAnotherTaint = true
					Expect(tol.Operator).To(Equal(v1.TolerationOpExists))
					Expect(tol.Effect).To(Equal(v1.TaintEffectPreferNoSchedule))
				}
			}
			Expect(foundCustomTaint).To(BeTrue())
			Expect(foundAnotherTaint).To(BeTrue())
		})
		It("should render both DeploymentNodeAffinity and DeploymentTolerations together", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			cr.Spec.DeploymentNodeAffinity = &v1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
					{
						Weight: 10,
						Preference: v1.NodeSelectorTerm{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "preferred-label",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"preferred-value"},
								},
							},
						},
					},
				},
			}
			cr.Spec.DeploymentTolerations = []v1.Toleration{
				{
					Key:      "combined-taint",
					Operator: v1.TolerationOpEqual,
					Value:    "combined-value",
					Effect:   v1.TaintEffectNoExecute,
				},
			}
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
			By("Verify NodeAffinity in Deployment")
			Expect(d.Spec.Template.Spec.Affinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
			// Check that both default and custom preferredDuringSchedulingIgnoredDuringExecution are present
			preferred := d.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			Expect(len(preferred)).To(Equal(3)) // 2 default + 1 custom
			// Verify custom preferred term is present
			foundPreferredLabel := false
			for _, term := range preferred {
				if len(term.Preference.MatchExpressions) > 0 && term.Preference.MatchExpressions[0].Key == "preferred-label" {
					foundPreferredLabel = true
					Expect(term.Weight).To(Equal(int32(10)))
					Expect(term.Preference.MatchExpressions[0].Operator).To(Equal(v1.NodeSelectorOpIn))
					Expect(term.Preference.MatchExpressions[0].Values).To(ContainElements("preferred-value"))
				}
			}
			Expect(foundPreferredLabel).To(BeTrue())
			By("Verify Tolerations in Deployment")
			tolerations := d.Spec.Template.Spec.Tolerations
			Expect(len(tolerations)).To(Equal(4)) // 3 default + 1 custom
			// Verify custom toleration is present
			foundCombinedTaint := false
			for _, tol := range tolerations {
				if tol.Key == "combined-taint" {
					foundCombinedTaint = true
					Expect(tol.Operator).To(Equal(v1.TolerationOpEqual))
					Expect(tol.Value).To(Equal("combined-value"))
					Expect(tol.Effect).To(Equal(v1.TaintEffectNoExecute))
				}
			}
			Expect(foundCombinedTaint).To(BeTrue())
		})
		It("should prioritize DeploymentNodeAffinity over NodeAffinity", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			cr.Spec.NodeAffinity = &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "general-label",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"general-value"},
								},
							},
						},
					},
				},
			}
			cr.Spec.DeploymentNodeAffinity = &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "deployment-specific-label",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"deployment-value"},
								},
							},
						},
					},
				},
			}
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
			By("Verify DeploymentNodeAffinity takes precedence")
			Expect(d.Spec.Template.Spec.Affinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
			// Check that both default preferred and custom required terms are present
			preferred := d.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			Expect(len(preferred)).To(Equal(2)) // 2 default preferred terms
			required := d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			Expect(required).NotTo(BeNil())
			nodeSelectorTerms := required.NodeSelectorTerms
			Expect(len(nodeSelectorTerms)).To(Equal(1))
			Expect(len(nodeSelectorTerms[0].MatchExpressions)).To(Equal(1))
			expr := nodeSelectorTerms[0].MatchExpressions[0]
			Expect(expr.Key).To(Equal("deployment-specific-label"))
			Expect(expr.Values).To(ContainElements("deployment-value"))
		})
	})
	Context("Verify Sync flows", func() {
		It("should create Daemonset, update state to Ready", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-node")
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-node"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.NvIpam.ImageSpec, cr)
			assertCNIBinDirForDS(ds)
			ds.Status = appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 1,
			}
			By("Update DaemonSet Status, and re-run Sync")
			err = client.Status().Update(context.Background(), ds)
			Expect(err).NotTo(HaveOccurred())
			By("Verify State is ready")
			ctx := context.Background()
			objs, err := renderer.GetManifestObjects(ctx, cr, catalog, log.FromContext(ctx))
			Expect(err).NotTo(HaveOccurred())
			status, err = getKindState(ctx, client, objs, "DaemonSet")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should create Daemonset and delete if Spec is nil", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-node")
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-node"}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, &cr.Spec.NvIpam.ImageSpec, cr)
			assertCNIBinDirForDS(ds)
			By("Set spec to nil and Sync")
			cr.Spec.NvIpam = nil
			status, err = nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet is deleted")
			ds = &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-node"}, ds)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should create Deployment, update state to Ready", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-controller")
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "nv-ipam-controller"}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, &cr.Spec.NvIpam.ImageSpec)
			d.Status = appsv1.DeploymentStatus{
				ObservedGeneration: 1,
				Replicas:           1,
				UpdatedReplicas:    1,
				AvailableReplicas:  1,
				ReadyReplicas:      1,
			}
			By("Update Deployment Status, and re-run Sync")
			err = client.Status().Update(context.Background(), d)
			Expect(err).NotTo(HaveOccurred())
			By("Verify State is ready")
			ctx := context.Background()
			objs, err := renderer.GetManifestObjects(ctx, cr, catalog, log.FromContext(ctx))
			Expect(err).NotTo(HaveOccurred())
			status, err = getKindState(ctx, client, objs, "Deployment")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should fail if static config provider not set in catalog", func() {
			By("Sync")
			catalog := state.NewInfoCatalog()
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-node")
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should fail if clustertype provider not set in catalog", func() {
			By("Sync")
			catalog := state.NewInfoCatalog()
			staticConfigProvider := staticconfig_mocks.Provider{}
			staticConfigProvider.On("GetStaticConfig").Return(staticconfig.StaticConfig{CniBinDirectory: ""})
			catalog.Add(state.InfoTypeStaticConfig, &staticConfigProvider)
			cr := getMinimalNicClusterPolicyWithNVIpam("nv-ipam-node")
			status, err := nvIpamState.Sync(context.Background(), cr, catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})
	})
})

func assertValidatingWebhookFields(adm *admv1.ValidatingWebhookConfiguration) {
	Expect(adm.Webhooks[0].Name).To(Equal("validate-ippool.nv-ipam.nvidia.com"))
	Expect(adm.Webhooks[1].Name).To(Equal("validate-cidrpool.nv-ipam.nvidia.com"))
}

func assertPodTemplateWebhookFields(template *v1.PodTemplateSpec) {
	Expect(template.Spec.Containers[0].Args).Should(ContainElement(ContainSubstring("--webhook=true")))
	Expect(template.Spec.Volumes).To(Equal(
		[]v1.Volume{
			{
				Name: "cert",
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName:  "nv-ipam-webhook-server-cert",
						DefaultMode: ptr.To(int32(420)),
					},
				},
			},
		},
	))

	Expect(template.Spec.Containers[0].VolumeMounts).To(Equal(
		[]v1.VolumeMount{
			{
				Name:      "cert",
				MountPath: "/tmp/k8s-webhook-server/serving-certs",
				ReadOnly:  true,
			},
		},
	))
}

func getMinimalNicClusterPolicyWithNVIpam(name string) *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()

	// add an arbitrary resource, this prevent adding defaut cpu,mem limits
	imageSpec := addContainerResources(getTestImageSpec(), name, "5", "3")
	nvIpamSpec := &mellanoxv1alpha1.NVIPAMSpec{
		EnableWebhook: false,
		ImageSpec:     *imageSpec,
	}
	cr.Spec.NvIpam = nvIpamSpec
	return cr
}
