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

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
	staticconfig_mocks "github.com/Mellanox/network-operator/pkg/staticconfig/mocks"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("CNI plugins state", func() {
	var ts testScope

	BeforeEach(func() {
		ts = ts.New(state.NewStateCNIPlugins, "../../manifests/state-container-networking-plugins")
		Expect(ts).NotTo(BeNil())
	})

	Context("Verify objects rendering", func() {
		It("should create Daemonset - minimal spec", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
		})

		It("should create Daemonset - SHA256 version", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			cr.Spec.SecondaryNetwork.CniPlugins.Version = "sha256:1699d23027ea30c9fa"
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			expectedDs.Spec.Template.Spec.Containers[0].Image = "myrepo/myimage@sha256:1699d23027ea30c9fa"
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
		})

		It("should create Daemonset with ImagePullSecrets when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			cr.Spec.SecondaryNetwork.CniPlugins.ImagePullSecrets = []string{"myimagepullsecret"}
			_, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			By("Verify ImagePullSecret")
			expectedDs.Spec.Template.Spec.ImagePullSecrets = []v1.LocalObjectReference{
				{
					Name: "myimagepullsecret",
				},
			}
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
		})

		It("should create Daemonset with user specified CNI bin dir", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			testCniBinDir := "/opt/mydir/cni"
			staticConfigProvider := staticconfig_mocks.Provider{}
			staticConfigProvider.On("GetStaticConfig").Return(staticconfig.StaticConfig{CniBinDirectory: testCniBinDir})
			ts.catalog.Add(state.InfoTypeStaticConfig, &staticConfigProvider)
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			expectedDs.Spec.Template.Spec.Volumes[0].VolumeSource.HostPath.Path = testCniBinDir
			By("Verify CNI bin dir")
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
		})

		It("should create Daemonset with Openshift CNI bin dir", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			staticConfigProvider := staticconfig_mocks.Provider{}
			staticConfigProvider.On("GetStaticConfig").Return(staticconfig.StaticConfig{CniBinDirectory: ""})
			ts.openshiftCatalog.Add(state.InfoTypeStaticConfig, &staticConfigProvider)
			status, err := ts.state.Sync(context.Background(), cr, ts.openshiftCatalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			expectedDs.Spec.Template.Spec.Volumes[0].VolumeSource.HostPath.Path = "/var/lib/cni/bin"
			By("Verify CNI bin dir")
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
		})

		It("should render Daemonset with Resources when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			cpu, _ := resource.ParseQuantity("1")
			mem, _ := resource.ParseQuantity("1Gi")
			cr.Spec.SecondaryNetwork.CniPlugins.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{
				{
					Name: "cni-plugins",
					Requests: v1.ResourceList{
						v1.ResourceCPU:    cpu,
						v1.ResourceMemory: mem,
					},
					Limits: v1.ResourceList{
						v1.ResourceCPU:    cpu,
						v1.ResourceMemory: mem,
					},
				},
			}
			_, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			rq := v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    cpu,
					v1.ResourceMemory: mem,
				},
				Limits: v1.ResourceList{
					v1.ResourceCPU:    cpu,
					v1.ResourceMemory: mem,
				},
			}
			expectedDs := getExpectedMinimalCniPluginDS()
			expectedDs.Spec.Template.Spec.Containers[0].Resources = rq
			By("Verify container resources")
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
		})

		It("should render Daemonset with NodeAffinity when specified in CR", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			nodeAffinity := &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "mykey",
									Operator: v1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			}
			cr.Spec.NodeAffinity = nodeAffinity
			_, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			expectedDs.Spec.Template.Spec.Affinity = &v1.Affinity{NodeAffinity: nodeAffinity}
			By("Verify NodeAffinity")
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))

		})
	})
	Context("Verify Sync flows", func() {
		It("should create Daemonset, update state to Ready", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
			ds.Status = appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 1,
			}
			By("Update DaemonSet Status, and re-run Sync")
			err = ts.client.Status().Update(context.Background(), ds)
			Expect(err).NotTo(HaveOccurred())
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			By("Verify State is ready")
			Expect(status).To(BeEquivalentTo(state.SyncStateReady))
		})

		It("should create Daemonset and delete if Spec is nil", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			status, err := ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet")
			ds := &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(err).NotTo(HaveOccurred())
			expectedDs := getExpectedMinimalCniPluginDS()
			Expect(ds.Spec).To(BeEquivalentTo(expectedDs.Spec))
			By("Set spec to nil and Sync")
			cr.Spec.SecondaryNetwork.CniPlugins = nil
			status, err = ts.state.Sync(context.Background(), cr, ts.catalog)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
			By("Verify DaemonSet is deleted")
			ds = &appsv1.DaemonSet{}
			err = ts.client.Get(context.Background(), types.NamespacedName{Namespace: ts.namespace, Name: "cni-plugins-ds"}, ds)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should fail if static config provider not set in catalog", func() {
			By("Sync")
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			noCatalog := state.NewInfoCatalog()
			status, err := ts.state.Sync(context.Background(), cr, noCatalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateError))
		})

		It("should fail if clustertype provider not set in catalog", func() {
			By("Sync")
			catalog := state.NewInfoCatalog()
			staticConfigProvider := staticconfig_mocks.Provider{}
			staticConfigProvider.On("GetStaticConfig").Return(staticconfig.StaticConfig{CniBinDirectory: ""})
			catalog.Add(state.InfoTypeStaticConfig, &staticConfigProvider)
			cr := getMinimalNicClusterPolicyWithCNIPlugins()
			status, err := ts.state.Sync(context.Background(), cr, catalog)
			Expect(err).To(HaveOccurred())
			Expect(status).To(BeEquivalentTo(state.SyncStateNotReady))
		})
	})
})

func getMinimalNicClusterPolicyWithCNIPlugins() *mellanoxv1alpha1.NicClusterPolicy {
	cr := &mellanoxv1alpha1.NicClusterPolicy{}
	cr.Name = "nic-cluster-policy"

	secondaryNetworkSpec := &mellanoxv1alpha1.SecondaryNetworkSpec{}
	secondaryNetworkSpec.CniPlugins = &mellanoxv1alpha1.ImageSpec{}
	secondaryNetworkSpec.CniPlugins.Image = "myimage"
	secondaryNetworkSpec.CniPlugins.Repository = "myrepo"
	secondaryNetworkSpec.CniPlugins.Version = "myversion"
	cr.Spec.SecondaryNetwork = secondaryNetworkSpec
	return cr
}

func getExpectedMinimalCniPluginDS() *appsv1.DaemonSet {
	trueVar := true
	cpu, _ := resource.ParseQuantity("100m")
	mem, _ := resource.ParseQuantity("50Mi")

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cni-plugins-ds",
			Namespace: "nvidia-network-operator",
			Labels: map[string]string{
				"tier": "node",
				"app":  "cni-plugins",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "cni-plugins",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "cni-plugins",
						"tier": "node",
						"app":  "cni-plugins",
					},
				},
				Spec: v1.PodSpec{
					HostNetwork: true,
					Containers: []v1.Container{
						{
							Name:            "cni-plugins",
							Image:           "myrepo/myimage:myversion",
							ImagePullPolicy: v1.PullIfNotPresent,
							SecurityContext: &v1.SecurityContext{
								Privileged: &trueVar,
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    cpu,
									v1.ResourceMemory: mem,
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    cpu,
									v1.ResourceMemory: mem,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "cnibin",
									MountPath: "/host/opt/cni/bin",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "cnibin",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/opt/cni/bin",
								},
							},
						},
					},
					Tolerations: []v1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}
}
