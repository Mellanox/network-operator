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
		Expect(mellanoxv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(v1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(appsv1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(admv1.AddToScheme(scheme)).NotTo(HaveOccurred())
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
