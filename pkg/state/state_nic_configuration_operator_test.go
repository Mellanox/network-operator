/*
2025 NVIDIA CORPORATION & AFFILIATES

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/state"
	staticconfig_mocks "github.com/Mellanox/network-operator/pkg/staticconfig/mocks"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("NIC Configuration Operator Controller", func() {
	var (
		nicConfigurationOperatorState state.State
		catalog                       state.InfoCatalog
		client                        client.Client
		namespace                     string
		renderer                      state.ManifestRenderer
		deploymentName                string
		daemonSetName                 string
	)

	BeforeEach(func() {
		deploymentName = "nic-configuration-operator"
		daemonSetName = "nic-configuration-daemon"

		scheme := runtime.NewScheme()
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(v1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(appsv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(admv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()
		manifestDir := "../../manifests/state-nic-configuration-operator"
		s, r, err := state.NewStateNicConfigurationOperator(client, manifestDir)
		Expect(err).NotTo(HaveOccurred())
		nicConfigurationOperatorState = s
		renderer = r
		catalog = getTestCatalog()
		catalog.Add(state.InfoTypeStaticConfig,
			staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: "custom-cni-bin-directory"}))
		namespace = config.FromEnv().State.NetworkOperatorResourceNamespace
	})

	Context("Verify objects rendering", func() {
		It("should create Daemonset - minimal spec", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
			assertCNIBinDirForDS(ds)
		})

		It("should create Deployment - minimal spec", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
		})

		It("should create Daemonset - SHA256 image format", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			cr.Spec.NicConfigurationOperator.ConfigurationDaemon.Version = defaultTestVersionSha256
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
		})

		It("should create Deployment - SHA256 image format", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			cr.Spec.NicConfigurationOperator.Operator.Version = defaultTestVersionSha256
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
		})

		It("should create Deployment and DaemonSet with log level when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			cr.Spec.NicConfigurationOperator.LogLevel = "debug"
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
			By("Verify Log Level in Deployment")
			Expect(d.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(v1.EnvVar{Name: "LOG_LEVEL", Value: "debug"}))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
			By("Verify Log Level in DaemonSet")
			Expect(ds.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(v1.EnvVar{Name: "LOG_LEVEL", Value: "debug"}))
		})

		It("should create PVC with storage class and size when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			cr.Spec.NicConfigurationOperator.NicFirmwareStorage = &mellanoxv1alpha1.NicFirmwareStorageSpec{
				Create:               true,
				PVCName:              "pvc",
				StorageClassName:     "class",
				AvailableStorageSize: "2Gi",
			}
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify PVC")
			pvc := &v1.PersistentVolumeClaim{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "pvc"}, pvc)
			Expect(err).NotTo(HaveOccurred())
			Expect(*pvc.Spec.StorageClassName).To(Equal("class"))
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("2Gi"))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
			By("Verify PVC in Deployment")
			Expect(d.Spec.Template.Spec.Volumes).Should(ContainElement(v1.Volume{
				Name: "firmware-cache",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvc",
					},
				},
			}))
			Expect(d.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(v1.VolumeMount{
				Name:      "firmware-cache",
				ReadOnly:  false,
				MountPath: "/nic-firmware",
			}))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
			By("Verify PVC in Deployment")
			Expect(ds.Spec.Template.Spec.Volumes).Should(ContainElement(v1.Volume{
				Name: "firmware-cache",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvc",
					},
				},
			}))
			Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(v1.VolumeMount{
				Name:      "firmware-cache",
				ReadOnly:  true,
				MountPath: "/nic-firmware",
			}))
		})

		It("should not create PVC when not specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			cr.Spec.NicConfigurationOperator.NicFirmwareStorage = nil
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify PVC")
			pvc := &v1.PersistentVolumeClaim{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "pvc"}, pvc)
			Expect(errors.IsNotFound(err)).To(BeTrue())
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
			By("Verify PVC in Deployment")
			Expect(d.Spec.Template.Spec.Volumes).Should(BeEmpty())
			Expect(d.Spec.Template.Spec.Containers[0].VolumeMounts).Should(BeEmpty())
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
			By("Verify PVC in Deployment")
			Expect(ds.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(v1.Volume{
				Name: "firmware-cache",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvc",
					},
				},
			}))
			Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(v1.VolumeMount{
				Name:      "firmware-cache",
				ReadOnly:  true,
				MountPath: "/nic-firmware",
			}))
		})

		It("should not create PVC but mount the existing one when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			cr.Spec.NicConfigurationOperator.NicFirmwareStorage = &mellanoxv1alpha1.NicFirmwareStorageSpec{
				Create:  false,
				PVCName: "pvc",
			}
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify PVC")
			pvc := &v1.PersistentVolumeClaim{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "pvc"}, pvc)
			Expect(errors.IsNotFound(err)).To(BeTrue())
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
			By("Verify PVC in Deployment")
			Expect(d.Spec.Template.Spec.Volumes).Should(ContainElement(v1.Volume{
				Name: "firmware-cache",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvc",
					},
				},
			}))
			Expect(d.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(v1.VolumeMount{
				Name:      "firmware-cache",
				ReadOnly:  false,
				MountPath: "/nic-firmware",
			}))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
			By("Verify PVC in Deployment")
			Expect(ds.Spec.Template.Spec.Volumes).Should(ContainElement(v1.Volume{
				Name: "firmware-cache",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvc",
					},
				},
			}))
			Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(v1.VolumeMount{
				Name:      "firmware-cache",
				ReadOnly:  true,
				MountPath: "/nic-firmware",
			}))
		})

		It("should render DeploymentNodeAffinity", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
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
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
			By("Verify NodeAffinity in Deployment")
			Expect(d.Spec.Template.Spec.Affinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).NotTo(BeNil())
			required := d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
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
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
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
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
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
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
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
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
			By("Verify NodeAffinity in Deployment")
			Expect(d.Spec.Template.Spec.Affinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
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
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
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
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.Operator)
			By("Verify DeploymentNodeAffinity takes precedence")
			Expect(d.Spec.Template.Spec.Affinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
			Expect(d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).NotTo(BeNil())
			required := d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			nodeSelectorTerms := required.NodeSelectorTerms
			Expect(len(nodeSelectorTerms)).To(Equal(1))
			Expect(len(nodeSelectorTerms[0].MatchExpressions)).To(Equal(1))
			expr := nodeSelectorTerms[0].MatchExpressions[0]
			Expect(expr.Key).To(Equal("deployment-specific-label"))
			Expect(expr.Values).To(ContainElements("deployment-value"))
		})
	})
	Context("Verify Sync flows", func() {
		It("should create DaemonSet, update state to Ready", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
			ds.Status = appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 1,
			}
			By("Update status of the daemon set, re-run sync")
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

		It("should create DaemonSet and delete if Spec is nil", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDaemonSetFields(ds, cr.Spec.NicConfigurationOperator.ConfigurationDaemon, cr)
			By("Set spec to nil and Sync")
			cr.Spec.NicConfigurationOperator = nil
			status, err = nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet is deleted")
			ds = &appsv1.DaemonSet{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: daemonSetName}, ds)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should create Deployment, update state to Ready", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify Deployment")
			d := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: deploymentName}, d)
			Expect(err).NotTo(HaveOccurred())
			assertCommonDeploymentFields(d, cr.Spec.NicConfigurationOperator.ConfigurationDaemon)
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
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should fail if clustertype provider not set in catalog", func() {
			By("Sync")
			catalog := state.NewInfoCatalog()
			staticConfigProvider := staticconfig_mocks.Provider{}
			staticConfigProvider.On("GetStaticConfig").Return(staticconfig.StaticConfig{CniBinDirectory: ""})
			catalog.Add(state.InfoTypeStaticConfig, &staticConfigProvider)
			cr := getMinimalNicClusterPolicyWithNicConfigurationOperator(deploymentName, daemonSetName)
			status, err := nicConfigurationOperatorState.Sync(context.Background(), cr, catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})
	})
})

func getMinimalNicClusterPolicyWithNicConfigurationOperator(
	operatorName, configDaemonName string) *mellanoxv1alpha1.NicClusterPolicy {
	cr := getTestClusterPolicyWithBaseFields()

	// add an arbitrary resource, this prevents adding default cpu and memory limits
	imageSpec := addContainerResources(getTestImageSpec(), operatorName, "5", "3")
	imageSpec = addContainerResources(imageSpec, configDaemonName, "5", "3")
	spec := &mellanoxv1alpha1.NicConfigurationOperatorSpec{
		Operator:            imageSpec,
		ConfigurationDaemon: imageSpec,
	}
	cr.Spec.NicConfigurationOperator = spec
	return cr
}
