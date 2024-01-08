/*
  2022 NVIDIA CORPORATION & AFFILIATES

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package state

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	osconfigv1 "github.com/openshift/api/config/v1"
	apiimagev1 "github.com/openshift/api/image/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

const (
	testClusterWideHTTPProxy  = "http-cluster-wide"
	testClusterWideHTTPSProxy = "https-cluster-wide"
	testClusterWideNoProxy    = "no-proxy-cluster-wide"
	testNicPolicyHTTPProxy    = "http-policy"
	testNicPolicyNoProxy      = "no-proxy-policy"
	osName                    = "ubuntu"
	osVer                     = "22.04"
	rhcosOsTree               = "414.92.202311061957-0"
)

type openShiftClusterProvider struct {
}

func (d *openShiftClusterProvider) GetClusterType() clustertype.Type {
	return clustertype.Openshift
}

func (d *openShiftClusterProvider) IsKubernetes() bool {
	return false
}

func (d *openShiftClusterProvider) IsOpenshift() bool {
	return true
}

var _ = Describe("MOFED state test", func() {
	var stateOfed stateOFED
	var ctx context.Context

	BeforeEach(func() {
		stateOfed = stateOFED{}
		ctx = context.Background()
	})

	Context("getMofedDriverImageName", func() {
		nodeAttr := make(map[nodeinfo.AttributeType]string)
		nodeAttr[nodeinfo.AttrTypeCPUArch] = "amd64"
		nodeAttr[nodeinfo.AttrTypeOSName] = "ubuntu"
		nodeAttr[nodeinfo.AttrTypeOSVer] = "20.04"

		cr := &v1alpha1.NicClusterPolicy{
			Spec: v1alpha1.NicClusterPolicySpec{
				OFEDDriver: &v1alpha1.OFEDDriverSpec{
					ImageSpec: v1alpha1.ImageSpec{
						Image:      "mofed",
						Repository: "nvcr.io/mellanox",
					},
				},
			},
		}

		It("generates new image format", func() {
			cr.Spec.OFEDDriver.Version = "5.7-1.0.0.0"
			imageName := stateOfed.getMofedDriverImageName(cr, nodeAttr, testLogger)
			Expect(imageName).To(Equal("nvcr.io/mellanox/mofed:5.7-1.0.0.0-ubuntu20.04-amd64"))
		})
		It("generates new image format double digit minor", func() {
			cr.Spec.OFEDDriver.Version = "5.10-0.0.0.1"
			imageName := stateOfed.getMofedDriverImageName(cr, nodeAttr, testLogger)
			Expect(imageName).To(Equal("nvcr.io/mellanox/mofed:5.10-0.0.0.1-ubuntu20.04-amd64"))
		})
		It("return new image format in case of a bad version", func() {
			cr.Spec.OFEDDriver.Version = "1.1.1.1.1"
			imageName := stateOfed.getMofedDriverImageName(cr, nodeAttr, testLogger)
			Expect(imageName).To(Equal("nvcr.io/mellanox/mofed:1.1.1.1.1-ubuntu20.04-amd64"))
		})
	})

	Context("Init container", func() {
		It("getInitContainerConfig", func() {
			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						OfedUpgradePolicy: &v1alpha1.DriverUpgradePolicySpec{
							AutoUpgrade: true,
							SafeLoad:    true,
						},
					},
				},
			}
			cfg := stateOfed.getInitContainerConfig(cr, testLogger, "repository/image:version")
			Expect(cfg.SafeLoadAnnotation).NotTo(BeEmpty())
			Expect(cfg.SafeLoadEnable).To(BeTrue())
			Expect(cfg.InitContainerEnable).To(BeTrue())
			Expect(cfg.InitContainerImageName).To(Equal("repository/image:version"))
		})
		It("getInitContainerConfig - no image", func() {
			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						OfedUpgradePolicy: &v1alpha1.DriverUpgradePolicySpec{
							AutoUpgrade: true,
							SafeLoad:    true,
						},
					},
				},
			}
			cfg := stateOfed.getInitContainerConfig(cr, testLogger, "")
			Expect(cfg.SafeLoadEnable).To(BeFalse())
			Expect(cfg.InitContainerEnable).To(BeFalse())
		})
		It("getInitContainerConfig - SafeLoad disabled if AutoUpgrade is false ", func() {
			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						OfedUpgradePolicy: &v1alpha1.DriverUpgradePolicySpec{
							AutoUpgrade: false,
							SafeLoad:    true,
						},
					},
				},
			}
			cfg := stateOfed.getInitContainerConfig(cr, testLogger, "repository/image:version")
			Expect(cfg.SafeLoadEnable).To(BeFalse())
			Expect(cfg.InitContainerEnable).To(BeTrue())
		})
	})
	Context("Proxy config", func() {
		It("Set Proxy from Cluster Wide Proxy", func() {
			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{OFEDDriver: &v1alpha1.OFEDDriverSpec{}}}
			clusterProxy := &osconfigv1.Proxy{
				Spec: osconfigv1.ProxySpec{
					HTTPProxy:  testClusterWideHTTPProxy,
					HTTPSProxy: testClusterWideHTTPSProxy,
					NoProxy:    testClusterWideNoProxy,
				},
			}
			stateOfed.setEnvFromClusterWideProxy(cr, clusterProxy)
			crEnv := cr.Spec.OFEDDriver.Env
			Expect(crEnv).To(HaveLen(6))
			Expect(crEnv).To(ContainElements(
				v1.EnvVar{Name: envVarNameNoProxy, Value: testClusterWideNoProxy},
				v1.EnvVar{Name: envVarNameHTTPProxy, Value: testClusterWideHTTPProxy},
				v1.EnvVar{Name: envVarNameHTTPSProxy, Value: testClusterWideHTTPSProxy},
				v1.EnvVar{Name: strings.ToLower(envVarNameNoProxy), Value: testClusterWideNoProxy},
				v1.EnvVar{Name: strings.ToLower(envVarNameHTTPProxy), Value: testClusterWideHTTPProxy},
				v1.EnvVar{Name: strings.ToLower(envVarNameHTTPSProxy), Value: testClusterWideHTTPSProxy},
			))
		})
		It("NicClusterPolicy proxy settings should have precedence", func() {
			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{OFEDDriver: &v1alpha1.OFEDDriverSpec{
					Env: []v1.EnvVar{
						{Name: envVarNameNoProxy, Value: testNicPolicyNoProxy},
						{Name: strings.ToLower(envVarNameHTTPProxy), Value: testNicPolicyHTTPProxy},
					},
				}}}
			clusterProxy := &osconfigv1.Proxy{
				Spec: osconfigv1.ProxySpec{
					HTTPProxy:  testClusterWideHTTPProxy,
					HTTPSProxy: testClusterWideHTTPSProxy,
					NoProxy:    testClusterWideNoProxy,
				},
			}
			stateOfed.setEnvFromClusterWideProxy(cr, clusterProxy)
			crEnv := cr.Spec.OFEDDriver.Env
			Expect(crEnv).To(HaveLen(4))
			Expect(crEnv).To(ContainElements(
				v1.EnvVar{Name: envVarNameNoProxy, Value: testNicPolicyNoProxy},
				v1.EnvVar{Name: strings.ToLower(envVarNameHTTPProxy), Value: testNicPolicyHTTPProxy},
				v1.EnvVar{Name: envVarNameHTTPSProxy, Value: testClusterWideHTTPSProxy},
				v1.EnvVar{Name: strings.ToLower(envVarNameHTTPSProxy), Value: testClusterWideHTTPSProxy},
			))
		})
	})

	DescribeTable("mergeWithDefaultEnvs",
		func(currEnvs []v1.EnvVar, expectedEnvs []v1.EnvVar) {
			mergedEnvs := stateOfed.mergeWithDefaultEnvs(currEnvs)
			Expect(mergedEnvs).To(BeEquivalentTo(expectedEnvs))
		},
		Entry("add defaults when no env vars",
			[]v1.EnvVar{}, []v1.EnvVar{{Name: "CREATE_IFNAMES_UDEV", Value: "true"}}),
		Entry("add defaults when env vars provided",
			[]v1.EnvVar{{Name: "Foo", Value: "Bar"}},
			[]v1.EnvVar{{Name: "Foo", Value: "Bar"}, {Name: "CREATE_IFNAMES_UDEV", Value: "true"}}),
		Entry("override defaults by user",
			[]v1.EnvVar{{Name: "CREATE_IFNAMES_UDEV", Value: "false"}},
			[]v1.EnvVar{{Name: "CREATE_IFNAMES_UDEV", Value: "false"}}),
		Entry("override defaults by user with additional env vars",
			[]v1.EnvVar{{Name: "Foo", Value: "Bar"}, {Name: "CREATE_IFNAMES_UDEV", Value: "false"}},
			[]v1.EnvVar{{Name: "Foo", Value: "Bar"}, {Name: "CREATE_IFNAMES_UDEV", Value: "false"}}),
	)

	Context("Render Manifests", func() {
		It("Should Render Mofed DaemonSet", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"
			scheme := runtime.NewScheme()

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					scheme:      scheme,
					renderer:    renderer,
				},
			}
			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
			}
			By("Creating NodeProvider with 1 Node")
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, nodeinfo.NewProvider([]*v1.Node{
				getNode("node1"),
			}))
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			// Expect 4 objects: DaemonSet, Service Account, ClusterRole, ClusterRoleBinding
			Expect(len(objs)).To(Equal(4))
			By("Verify DaemonSet")
			for _, obj := range objs {
				if obj.GetKind() != "DaemonSet" {
					continue
				}
				ds := appsv1.DaemonSet{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
				Expect(err).NotTo(HaveOccurred())
				Expect(ds.Name).To(Equal("mofed-ubuntu22.04-ds"))
				verifyDSNodeSelector(ds.Spec.Template.Spec.NodeSelector)
				verifyPodAntiInfinity(ds.Spec.Template.Spec.Affinity)
			}
		})
	})
	Context("Render Manifests DTK", func() {
		It("Should Render DaemonSet with DTK", func() {
			dtkImageName := "quay.io/openshift-release-dev/ocp-v4.0-art-dev:414"
			dtkImageStream := &apiimagev1.ImageStream{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ImageStream",
					APIVersion: "image/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "driver-toolkit",
					Namespace: "openshift",
				},
				Spec: apiimagev1.ImageStreamSpec{
					Tags: []apiimagev1.TagReference{
						{
							Name: rhcosOsTree,
							From: &v1.ObjectReference{
								Kind: "DockerImage",
								Name: dtkImageName,
							},
						},
					},
				},
			}
			scheme := runtime.NewScheme()
			Expect(apiimagev1.AddToScheme(scheme)).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dtkImageStream).Build()
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      client,
					scheme:      scheme,
					renderer:    renderer,
				},
			}
			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				Env: []v1.EnvVar{
					{
						Name:  "ENTRYPOINT_DEBUG",
						Value: "true",
					},
				},
			}
			By("Creating NodeProvider with 1 Node with RHCOS OS TREE label")
			node := getNode("node1")
			node.Labels[nodeinfo.NodeLabelOSTreeVersion] = rhcosOsTree
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				node,
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &openShiftClusterProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			// Expect 6 object due to OpenShift: DaemonSet, Service Account, ClusterRole, ClusterRoleBinding
			// Role, RoleBinding
			Expect(len(objs)).To(Equal(6))
			By("Verify DaemonSet with DTK")
			for _, obj := range objs {
				if obj.GetKind() != "DaemonSet" {
					continue
				}
				ds := appsv1.DaemonSet{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
				Expect(err).NotTo(HaveOccurred())
				By("Verify DaemonSet NodeSelector")
				verifyDSNodeSelector(ds.Spec.Template.Spec.NodeSelector)
				By("Verify DTK container image")
				Expect(len(ds.Spec.Template.Spec.Containers)).To(Equal(2))
				dtkContainer := ds.Spec.Template.Spec.Containers[1]
				Expect(dtkContainer.Image).To(Equal(dtkImageName))
			}
		})
	})
})

func verifyPodAntiInfinity(affinity *v1.Affinity) {
	By("Verify PodAntiInfinity")
	Expect(affinity).NotTo(BeNil())
	expected := v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "nvidia.com/ofed-driver",
								Operator: metav1.LabelSelectorOpExists,
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
	Expect(*affinity).To(BeEquivalentTo(expected))
}

func verifyDSNodeSelector(selector map[string]string) {
	By("Verify NodeSelector")
	nsMellanox, ok := selector["feature.node.kubernetes.io/pci-15b3.present"]
	Expect(ok).To(BeTrue())
	Expect(nsMellanox).To(Equal("true"))
	nsOsName, ok := selector["feature.node.kubernetes.io/system-os_release.ID"]
	Expect(ok).To(BeTrue())
	Expect(nsOsName).To(Equal(osName))
	nsOsVer, ok := selector["feature.node.kubernetes.io/system-os_release.VERSION_ID"]
	Expect(ok).To(BeTrue())
	Expect(nsOsVer).To(Equal(osVer))
}

func getNode(name string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				nodeinfo.NodeLabelMlnxNIC: "true",
				nodeinfo.NodeLabelOSName:  osName,
				nodeinfo.NodeLabelOSVer:   osVer,
				nodeinfo.NodeLabelCPUArch: "amd64",
			},
		},
	}
}
