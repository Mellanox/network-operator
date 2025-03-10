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
