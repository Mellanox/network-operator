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
	"fmt"
	"slices"

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
	kernelFull1               = "5.15.0-78-generic"
	kernelFull2               = "5.15.0-91-generic"
	archAmd                   = "amd64"
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

type dummyOfedImageProvider struct {
	tagExists bool
}

func (d *dummyOfedImageProvider) TagExists(_ string) (bool, error) {
	return d.tagExists, nil
}

func (d *dummyOfedImageProvider) SetImageSpec(*v1alpha1.ImageSpec) {}

type errorOfedImageProvider struct {
	tagExists bool
	err       error
}

func (d *errorOfedImageProvider) TagExists(_ string) (bool, error) {
	return d.tagExists, d.err
}

func (d *errorOfedImageProvider) SetImageSpec(*v1alpha1.ImageSpec) {}

// nolint:dupl
var _ = Describe("MOFED state test", func() {
	var stateOfed stateOFED
	var ctx context.Context

	BeforeEach(func() {
		stateOfed = stateOFED{}
		ctx = context.Background()
	})

	Context("getMofedDriverImageName", func() {
		nodePool := &nodeinfo.NodePool{
			OsName:    "ubuntu",
			OsVersion: "20.04",
			Arch:      "amd64",
		}

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
			imageName := stateOfed.getMofedDriverImageName(cr, nodePool, false, false, testLogger)
			Expect(imageName).To(Equal("nvcr.io/mellanox/mofed:5.7-1.0.0.0-ubuntu20.04-amd64"))
		})
		It("generates new image format double digit minor", func() {
			cr.Spec.OFEDDriver.Version = "5.10-0.0.0.1"
			imageName := stateOfed.getMofedDriverImageName(cr, nodePool, false, false, testLogger)
			Expect(imageName).To(Equal("nvcr.io/mellanox/mofed:5.10-0.0.0.1-ubuntu20.04-amd64"))
		})
		It("return new image format in case of a bad version", func() {
			cr.Spec.OFEDDriver.Version = "1.1.1.1.1"
			imageName := stateOfed.getMofedDriverImageName(cr, nodePool, false, false, testLogger)
			Expect(imageName).To(Equal("nvcr.io/mellanox/mofed:1.1.1.1.1-ubuntu20.04-amd64"))
		})
		It("return SHA256 image format", func() {
			cr.Spec.OFEDDriver.Version = "sha256:1234567890"
			imageName := stateOfed.getMofedDriverImageName(cr, nodePool, false, true, testLogger)
			Expect(imageName).To(Equal("nvcr.io/mellanox/mofed@sha256:1234567890"))
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
			[]v1.EnvVar{}, []v1.EnvVar{
				{Name: envVarCreateIfNamesUdev, Value: "true"},
				{Name: envVarDriversInventoryPath, Value: defaultDriversInventoryPath}}),
		Entry("add defaults when env vars provided",
			[]v1.EnvVar{{Name: "Foo", Value: "Bar"}},
			[]v1.EnvVar{
				{Name: "Foo", Value: "Bar"},
				{Name: envVarCreateIfNamesUdev, Value: "true"},
				{Name: envVarDriversInventoryPath, Value: defaultDriversInventoryPath}}),
		Entry("override defaults by user",
			[]v1.EnvVar{
				{Name: envVarCreateIfNamesUdev, Value: "false"},
				{Name: envVarDriversInventoryPath, Value: ""}},
			[]v1.EnvVar{
				{Name: envVarCreateIfNamesUdev, Value: "false"},
				{Name: envVarDriversInventoryPath, Value: ""}}),
		Entry("override defaults by user with additional env vars",
			[]v1.EnvVar{
				{Name: "Foo", Value: "Bar"},
				{Name: envVarCreateIfNamesUdev, Value: "false"},
				{Name: envVarDriversInventoryPath, Value: ""}},
			[]v1.EnvVar{
				{Name: "Foo", Value: "Bar"},
				{Name: envVarCreateIfNamesUdev, Value: "false"},
				{Name: envVarDriversInventoryPath, Value: ""}}),
	)

	DescribeTable("GetStringHash",
		func(input, hash string) {
			computedHash := getStringHash(input)
			Expect(computedHash).To(BeEquivalentTo(hash))
		},
		Entry("kernel rhcos", "5.14.0-284.43.1.el9_2.x86_64", "687cd9dc94"),
		Entry("kernel ubuntu", "5.15.0-78-generic", "54669c9886"),
		Entry("kernel ubuntu - patch 91", "5.15.0-91-generic", "6d568d699f"),
	)

	Context("Render Manifests", func() {
		It("Should Render multiple DaemonSet", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
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

			By("Creating NodeProvider with 3 Nodes, that form 2 Node pools")
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
				getNode("node2", kernelFull2),
				getNode("node3", kernelFull2),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			// Expect 5 objects: 1 DS per pool, Service Account, Role, RoleBinding
			Expect(len(objs)).To(Equal(5))
			By("Verify DaemonSets NodeSelector")
			for _, obj := range objs {
				if obj.GetKind() != "DaemonSet" {
					continue
				}
				ds := appsv1.DaemonSet{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
				Expect(err).NotTo(HaveOccurred())
				if ds.Name == fmt.Sprintf("mofed-%s%s-%s-ds", osName, osVer, "54669c9886") {
					verifyDSNodeSelector(ds.Spec.Template.Spec.NodeSelector, kernelFull1)
				}
				if ds.Name == fmt.Sprintf("mofed-%s%s-%s-ds", osName, osVer, "6d568d699f") {
					verifyDSNodeSelector(ds.Spec.Template.Spec.NodeSelector, kernelFull2)
				}
				verifyPodAntiInfinity(ds.Spec.Template.Spec.Affinity)
			}
		})
		It("Should Render subscription mounts for RHEL + containerd", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
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

			By("Creating NodeProvider with 1 Nodes, RHEL with containerd")
			node := getNode("node1", kernelFull1)
			setContainerRuntime(node, "containerd://1.27.1")
			node.Labels[nodeinfo.NodeLabelOSName] = "rhel"
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				node,
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: false})
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			// Expect 5 objects: 1 DS per pool, Service Account, Role, RoleBinding
			Expect(len(objs)).To(Equal(4))
			By("Verify Subscription mounts")
			for _, obj := range objs {
				if obj.GetKind() != "DaemonSet" {
					continue
				}
				ds := appsv1.DaemonSet{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
				Expect(err).NotTo(HaveOccurred())
				verifySubscriptionMountsRhel(ds.Spec.Template.Spec.Containers[0].VolumeMounts)
				verifySubscriptionVolumesRhel(ds.Spec.Template.Spec.Volumes)
			}
		})
		It("Should render DaemonSet with CR and default GPU tolerations, and filter nodes correctly", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy-tolerations-test"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
			}
			cr.Spec.Tolerations = []v1.Toleration{
				{
					Key:      "custom-taint",
					Operator: v1.TolerationOpEqual,
					Value:    "true",
					Effect:   v1.TaintEffectNoSchedule,
				},
			}

			node1 := getNode("node1-selected", kernelFull1)
			node1.Spec.Taints = []v1.Taint{
				{Key: "nvidia.com/gpu", Value: "true", Effect: v1.TaintEffectNoSchedule},
				{Key: "custom-taint", Value: "true", Effect: v1.TaintEffectNoSchedule},
			}

			node2 := getNode("node2-not-selected", kernelFull2)
			node2.Spec.Taints = []v1.Taint{
				{Key: "another-critical-taint", Value: "true", Effect: v1.TaintEffectNoSchedule},
			}

			infoProvider := nodeinfo.NewProvider([]*v1.Node{node1, node2})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})

			By("Calling GetManifestObjects with nodes requiring specific tolerations")
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())

			// Expect 1 DaemonSet + ServiceAccount, Role, RoleBinding
			By("Verifying the correct number of objects are rendered")
			Expect(len(objs)).To(Equal(4), "Expected 1 DaemonSet and its 3 associated RBAC resources")

			By("Finding the rendered DaemonSet")
			var daemonSet *appsv1.DaemonSet
			for _, obj := range objs {
				if obj.GetKind() == "DaemonSet" {
					ds := &appsv1.DaemonSet{}
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ds)
					Expect(err).NotTo(HaveOccurred())
					// Ensure we only find one DaemonSet
					Expect(daemonSet).To(BeNil(), "Expected only one DaemonSet to be rendered")
					daemonSet = ds
				}
			}
			Expect(daemonSet).NotTo(BeNil(), "A DaemonSet should have been rendered")
		})
	})
	It("Should Render subscription mounts for SLES", func() {
		client := mocks.ControllerRuntimeClient{}
		manifestBaseDir := "../../manifests/state-ofed-driver"

		files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
		Expect(err).NotTo(HaveOccurred())
		renderer := render.NewRenderer(files)

		ofedState := stateOFED{
			stateSkel: stateSkel{
				name:        stateOFEDName,
				description: stateOFEDDescription,
				client:      &client,
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

		By("Creating NodeProvider with 1 Nodes, SLES")
		node := getNode("node1", kernelFull1)
		node.Labels[nodeinfo.NodeLabelOSName] = "sles"
		infoProvider := nodeinfo.NewProvider([]*v1.Node{
			node,
		})
		catalog := NewInfoCatalog()
		catalog.Add(InfoTypeClusterType, &dummyProvider{})
		catalog.Add(InfoTypeNodeInfo, infoProvider)
		catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: false})
		objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
		Expect(err).NotTo(HaveOccurred())
		// Expect 5 objects: 1 DS per pool, Service Account, Role, RoleBinding
		Expect(len(objs)).To(Equal(4))
		By("Verify Subscription mounts")
		for _, obj := range objs {
			if obj.GetKind() != "DaemonSet" {
				continue
			}
			ds := appsv1.DaemonSet{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
			Expect(err).NotTo(HaveOccurred())
			verifySubscriptionMountsSles(ds.Spec.Template.Spec.Containers[0].VolumeMounts)
			verifySubscriptionVolumesSles(ds.Spec.Template.Spec.Volumes)
		}
	})
	Context("SHA256 version", func() {
		It("Should Render DaemonSet with SHA256 version", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}
			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy-sha256"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "sha256:9a831bfdf85f313b1f5749b7c9b2673bb8fff18b4ff768c9242dabaa4468e449",
				},
			}

			By("Creating NodeProvider with 1 Node")
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: false})

			By("Calling GetManifestObjects with SHA256 version")
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())

			// Expect 4 objects: 1 DaemonSet + ServiceAccount, Role, RoleBinding
			By("Verifying the correct number of objects are rendered")
			Expect(len(objs)).To(Equal(4), "Expected 1 DaemonSet and its 3 associated RBAC resources")

			By("Finding and verifying the rendered DaemonSet with SHA256 image")
			var daemonSet *appsv1.DaemonSet
			for _, obj := range objs {
				if obj.GetKind() == "DaemonSet" {
					ds := &appsv1.DaemonSet{}
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ds)
					Expect(err).NotTo(HaveOccurred())
					daemonSet = ds
					break
				}
			}
			Expect(daemonSet).NotTo(BeNil(), "A DaemonSet should have been rendered")

			By("Verifying the SHA256 image format in the DaemonSet")
			expectedImageName := "nvcr.io/mellanox/mofed@sha256:9a831bfdf85f313b1f5749b7c9b2673bb8fff18b4ff768c9242dabaa4468e449"
			Expect(daemonSet.Spec.Template.Spec.Containers[0].Image).To(Equal(expectedImageName),
				"DaemonSet should use SHA256 image format with @ separator")

			Expect(len(daemonSet.Spec.Template.Spec.Containers)).To(Equal(1))

			By("Verifying node selector is set correctly")
			verifyDSNodeSelector(daemonSet.Spec.Template.Spec.NodeSelector, kernelFull1)

			By("Verifying pod anti-affinity is set")
			verifyPodAntiInfinity(daemonSet.Spec.Template.Spec.Affinity)
		})
	})
	Context("Render Manifests DTK", func() {
		It("Should Render DaemonSet with DTK and additional mounts", func() {
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
			cmRepo := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repo-cm",
					Namespace: "nvidia-network-operator",
				},
				Data: map[string]string{"ubi.repo": "somerepocontents"},
			}
			cmCert := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cert-cm",
					Namespace: "nvidia-network-operator",
				},
				Data: map[string]string{"my-cert": "somecertificate"},
			}
			scheme := runtime.NewScheme()
			Expect(v1.AddToScheme(scheme)).NotTo(HaveOccurred())
			Expect(apiimagev1.AddToScheme(scheme)).NotTo(HaveOccurred())
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dtkImageStream, cmRepo, cmCert).Build()
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      client,
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
				RepoConfig: &v1alpha1.ConfigMapNameReference{
					Name: "repo-cm",
				},
				CertConfig: &v1alpha1.ConfigMapNameReference{
					Name: "cert-cm",
				},
				Env: []v1.EnvVar{
					{
						Name:  "ENTRYPOINT_DEBUG",
						Value: "true",
					},
				},
			}
			By("Creating NodeProvider with 1 Node with RHCOS OS TREE label")
			node := getNode("node1", kernelFull1)
			node.Labels[nodeinfo.NodeLabelOSTreeVersion] = rhcosOsTree
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				node,
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &openShiftClusterProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: false})
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
				verifyDSNodeSelector(ds.Spec.Template.Spec.NodeSelector, kernelFull1)
				By("Verify DTK container image")
				Expect(len(ds.Spec.Template.Spec.Containers)).To(Equal(2))
				dtkContainer := ds.Spec.Template.Spec.Containers[1]
				Expect(dtkContainer.Image).To(Equal(dtkImageName))
				verifyAdditionalMounts(ds.Spec.Template.Spec.Containers[0].VolumeMounts)
				verifyAdditionalMounts(ds.Spec.Template.Spec.Containers[1].VolumeMounts)
				verifyAdditionalVolumes(ds.Spec.Template.Spec.Volumes)
			}
		})
	})
	Context("Force Precompiled", func() {
		It("Should fail getManifestObjects, forcePrecompiled true and tag does not exists", func() {
			ofedState := getOfedState()
			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				ForcePrecompiled: true,
			}
			By("Creating NodeProvider with 1 Node, that form 1 Node pool")
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: false})
			_, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).To(HaveOccurred())
		})
		It("Should use image with sources format, forcePrecompiled false and tag does not exists", func() {
			ofedState := getOfedState()
			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				ForcePrecompiled: false,
			}
			By("Creating NodeProvider with 1 Node, that form 1 Node pool")
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: false})
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			By("Verify image is not precompiled format")
			// Expect 4 objects: DS , Service Account, Role, RoleBinding
			Expect(len(objs)).To(Equal(4))
			for _, obj := range objs {
				if obj.GetKind() != "DaemonSet" {
					continue
				}
				ds := appsv1.DaemonSet{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
				Expect(err).NotTo(HaveOccurred())
				withSourceImage := fmt.Sprintf(mofedImageFormat,
					cr.Spec.OFEDDriver.Repository, cr.Spec.OFEDDriver.Image, cr.Spec.OFEDDriver.Version,
					osName, osVer, archAmd)
				Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(withSourceImage))
			}
		})
		It("Should use image with sources format, forcePrecompiled false and tag exists", func() {
			ofedState := getOfedState()
			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				ForcePrecompiled: false,
			}
			By("Creating NodeProvider with 1 Node, that form 1 Node pool")
			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			By("Verify image is not precompiled format")
			// Expect 4 objects: DS , Service Account, Role, RoleBinding
			Expect(len(objs)).To(Equal(4))
			for _, obj := range objs {
				if obj.GetKind() != "DaemonSet" {
					continue
				}
				ds := appsv1.DaemonSet{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
				Expect(err).NotTo(HaveOccurred())
				precompiledImage := fmt.Sprintf(precompiledImageFormat,
					cr.Spec.OFEDDriver.Repository, cr.Spec.OFEDDriver.Image, cr.Spec.OFEDDriver.Version,
					kernelFull1, osName, osVer, archAmd)
				Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(precompiledImage))
			}
		})
	})
	Context("Probe Configuration", func() {
		It("Should set default probe values when not provided", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "mofed",
							Repository: "nvcr.io/mellanox",
							Version:    "23.10-0.5.5.0",
						},
					},
				},
			}

			// Verify probes are initially nil
			Expect(cr.Spec.OFEDDriver.StartupProbe).To(BeNil())
			Expect(cr.Spec.OFEDDriver.LivenessProbe).To(BeNil())
			Expect(cr.Spec.OFEDDriver.ReadinessProbe).To(BeNil())

			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})

			// Call GetManifestObjects which internally calls setProbesDefaults
			_, err = ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())

			// Verify default values are set in the CR after GetManifestObjects
			Expect(cr.Spec.OFEDDriver.StartupProbe).NotTo(BeNil())
			Expect(cr.Spec.OFEDDriver.StartupProbe.InitialDelaySeconds).To(Equal(30))
			Expect(cr.Spec.OFEDDriver.StartupProbe.PeriodSeconds).To(Equal(10))

			Expect(cr.Spec.OFEDDriver.LivenessProbe).NotTo(BeNil())
			Expect(cr.Spec.OFEDDriver.LivenessProbe.InitialDelaySeconds).To(Equal(10))
			Expect(cr.Spec.OFEDDriver.LivenessProbe.PeriodSeconds).To(Equal(30))

			Expect(cr.Spec.OFEDDriver.ReadinessProbe).NotTo(BeNil())
			Expect(cr.Spec.OFEDDriver.ReadinessProbe.InitialDelaySeconds).To(Equal(10))
			Expect(cr.Spec.OFEDDriver.ReadinessProbe.PeriodSeconds).To(Equal(30))
		})

		It("Should preserve custom probe values when provided", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			customStartupProbe := &v1alpha1.PodProbeSpec{
				InitialDelaySeconds: 15,
				PeriodSeconds:       20,
			}
			customLivenessProbe := &v1alpha1.PodProbeSpec{
				InitialDelaySeconds: 45,
				PeriodSeconds:       60,
			}
			customReadinessProbe := &v1alpha1.PodProbeSpec{
				InitialDelaySeconds: 25,
				PeriodSeconds:       40,
			}

			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "mofed",
							Repository: "nvcr.io/mellanox",
							Version:    "23.10-0.5.5.0",
						},
						StartupProbe:   customStartupProbe,
						LivenessProbe:  customLivenessProbe,
						ReadinessProbe: customReadinessProbe,
					},
				},
			}

			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})

			// Call GetManifestObjects which internally calls setProbesDefaults
			_, err = ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())

			// Verify custom values are preserved after GetManifestObjects
			Expect(cr.Spec.OFEDDriver.StartupProbe).To(Equal(customStartupProbe))
			Expect(cr.Spec.OFEDDriver.LivenessProbe).To(Equal(customLivenessProbe))
			Expect(cr.Spec.OFEDDriver.ReadinessProbe).To(Equal(customReadinessProbe))
		})

		It("Should render DaemonSet with probe configurations", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "mofed",
							Repository: "nvcr.io/mellanox",
							Version:    "23.10-0.5.5.0",
						},
						StartupProbe: &v1alpha1.PodProbeSpec{
							InitialDelaySeconds: 15,
							PeriodSeconds:       20,
							FailureThreshold:    65,
							TimeoutSeconds:      5,
						},
						LivenessProbe: &v1alpha1.PodProbeSpec{
							InitialDelaySeconds: 45,
							PeriodSeconds:       60,
							FailureThreshold:    5,
							TimeoutSeconds:      15,
						},
						ReadinessProbe: &v1alpha1.PodProbeSpec{
							InitialDelaySeconds: 25,
							PeriodSeconds:       40,
							FailureThreshold:    3,
							TimeoutSeconds:      20,
						},
					},
				},
			}

			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})

			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())

			// Find the DaemonSet and verify probe configurations
			for _, obj := range objs {
				if obj.GetKind() == "DaemonSet" {
					ds := appsv1.DaemonSet{}
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
					Expect(err).NotTo(HaveOccurred())

					container := ds.Spec.Template.Spec.Containers[0]

					// Verify StartupProbe
					Expect(container.StartupProbe).NotTo(BeNil())
					Expect(container.StartupProbe.InitialDelaySeconds).To(Equal(int32(15)))
					Expect(container.StartupProbe.PeriodSeconds).To(Equal(int32(20)))
					Expect(container.StartupProbe.FailureThreshold).To(Equal(int32(65)))
					Expect(container.StartupProbe.SuccessThreshold).To(Equal(int32(1)))
					Expect(container.StartupProbe.TimeoutSeconds).To(Equal(int32(5)))

					// Verify LivenessProbe
					Expect(container.LivenessProbe).NotTo(BeNil())
					Expect(container.LivenessProbe.InitialDelaySeconds).To(Equal(int32(45)))
					Expect(container.LivenessProbe.PeriodSeconds).To(Equal(int32(60)))
					Expect(container.LivenessProbe.FailureThreshold).To(Equal(int32(5)))
					Expect(container.LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
					Expect(container.LivenessProbe.TimeoutSeconds).To(Equal(int32(15)))

					// Verify ReadinessProbe
					Expect(container.ReadinessProbe).NotTo(BeNil())
					Expect(container.ReadinessProbe.InitialDelaySeconds).To(Equal(int32(25)))
					Expect(container.ReadinessProbe.PeriodSeconds).To(Equal(int32(40)))
					Expect(container.ReadinessProbe.FailureThreshold).To(Equal(int32(3)))
					Expect(container.ReadinessProbe.SuccessThreshold).To(Equal(int32(1)))
					Expect(container.ReadinessProbe.TimeoutSeconds).To(Equal(int32(20)))

					// Verify probe commands
					Expect(container.StartupProbe.Exec.Command).To(
						Equal([]string{"sh", "-c", "ls /run/mellanox/drivers/.driver-ready"}))
					Expect(container.LivenessProbe.Exec.Command).To(Equal([]string{"sh", "-c", "lsmod | grep mlx5_core"}))
					Expect(container.ReadinessProbe.Exec.Command).To(Equal([]string{"sh", "-c", "lsmod | grep mlx5_core"}))

					break
				}
			}
		})

		It("Should render DaemonSet with default probe configurations when not specified", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "mofed",
							Repository: "nvcr.io/mellanox",
							Version:    "23.10-0.5.5.0",
						},
					},
				},
			}

			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})

			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())

			// Find the DaemonSet and verify default probe configurations
			for _, obj := range objs {
				if obj.GetKind() == "DaemonSet" {
					ds := appsv1.DaemonSet{}
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ds)
					Expect(err).NotTo(HaveOccurred())

					container := ds.Spec.Template.Spec.Containers[0]

					// Verify StartupProbe defaults
					Expect(container.StartupProbe).NotTo(BeNil())
					Expect(container.StartupProbe.InitialDelaySeconds).To(Equal(int32(30)))
					Expect(container.StartupProbe.PeriodSeconds).To(Equal(int32(10)))
					Expect(container.StartupProbe.FailureThreshold).To(Equal(int32(60)))
					Expect(container.StartupProbe.SuccessThreshold).To(Equal(int32(1)))
					Expect(container.StartupProbe.TimeoutSeconds).To(Equal(int32(10)))

					// Verify LivenessProbe defaults
					Expect(container.LivenessProbe).NotTo(BeNil())
					Expect(container.LivenessProbe.InitialDelaySeconds).To(Equal(int32(10)))
					Expect(container.LivenessProbe.PeriodSeconds).To(Equal(int32(30)))
					Expect(container.LivenessProbe.FailureThreshold).To(Equal(int32(1)))
					Expect(container.LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
					Expect(container.LivenessProbe.TimeoutSeconds).To(Equal(int32(10)))

					// Verify ReadinessProbe defaults
					Expect(container.ReadinessProbe).NotTo(BeNil())
					Expect(container.ReadinessProbe.InitialDelaySeconds).To(Equal(int32(10)))
					Expect(container.ReadinessProbe.PeriodSeconds).To(Equal(int32(30)))
					Expect(container.ReadinessProbe.FailureThreshold).To(Equal(int32(1)))
					Expect(container.ReadinessProbe.SuccessThreshold).To(Equal(int32(1)))
					Expect(container.ReadinessProbe.TimeoutSeconds).To(Equal(int32(10)))

					break
				}
			}
		})

		It("Should handle partial probe configurations", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{
				Spec: v1alpha1.NicClusterPolicySpec{
					OFEDDriver: &v1alpha1.OFEDDriverSpec{
						ImageSpec: v1alpha1.ImageSpec{
							Image:      "mofed",
							Repository: "nvcr.io/mellanox",
							Version:    "23.10-0.5.5.0",
						},
						StartupProbe: &v1alpha1.PodProbeSpec{
							InitialDelaySeconds: 20,
							PeriodSeconds:       25,
						},
						LivenessProbe: &v1alpha1.PodProbeSpec{
							FailureThreshold: 7,
							TimeoutSeconds:   17,
						},
					},
				},
			}

			infoProvider := nodeinfo.NewProvider([]*v1.Node{
				getNode("node1", kernelFull1),
			})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})

			_, err = ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())

			// Verify custom and default values are used for StartupProbe and LivenessProbe
			Expect(cr.Spec.OFEDDriver.StartupProbe.InitialDelaySeconds).To(Equal(20))
			Expect(cr.Spec.OFEDDriver.StartupProbe.PeriodSeconds).To(Equal(25))
			Expect(cr.Spec.OFEDDriver.StartupProbe.FailureThreshold).To(Equal(60))
			Expect(cr.Spec.OFEDDriver.StartupProbe.TimeoutSeconds).To(Equal(10))

			Expect(cr.Spec.OFEDDriver.LivenessProbe.InitialDelaySeconds).To(Equal(10))
			Expect(cr.Spec.OFEDDriver.LivenessProbe.PeriodSeconds).To(Equal(30))
			Expect(cr.Spec.OFEDDriver.LivenessProbe.FailureThreshold).To(Equal(7))
			Expect(cr.Spec.OFEDDriver.LivenessProbe.TimeoutSeconds).To(Equal(17))

			// Verify defaults are set for ReadinessProbe
			Expect(cr.Spec.OFEDDriver.ReadinessProbe).NotTo(BeNil())
			Expect(cr.Spec.OFEDDriver.ReadinessProbe.InitialDelaySeconds).To(Equal(10))
			Expect(cr.Spec.OFEDDriver.ReadinessProbe.PeriodSeconds).To(Equal(30))
			Expect(cr.Spec.OFEDDriver.ReadinessProbe.FailureThreshold).To(Equal(1))
			Expect(cr.Spec.OFEDDriver.ReadinessProbe.TimeoutSeconds).To(Equal(10))
		})
	})
	Context("TagExists error handling", func() {
		It("Should return error when TagExists fails and ForcePrecompiled is enabled", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy-error-test"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				ForcePrecompiled: true,
			}

			node := getNode("node1", kernelFull1)
			infoProvider := nodeinfo.NewProvider([]*v1.Node{node})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &errorOfedImageProvider{
				tagExists: false,
				err:       fmt.Errorf("network error: failed to list tags"),
			})

			By("Calling GetManifestObjects with TagExists returning error and ForcePrecompiled enabled")
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(
				ContainSubstring("ForcePrecompiled is enabled, but failed to verify existence of precompiled tag"))
			Expect(err.Error()).To(ContainSubstring("network error: failed to list tags"))
			Expect(objs).To(BeNil())
		})

		It("Should not return error when TagExists fails and ForcePrecompiled is disabled", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy-error-test"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				ForcePrecompiled: false,
			}

			node := getNode("node1", kernelFull1)
			infoProvider := nodeinfo.NewProvider([]*v1.Node{node})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &errorOfedImageProvider{
				tagExists: false,
				err:       fmt.Errorf("authentication failed"),
			})

			By("Calling GetManifestObjects with TagExists returning error and ForcePrecompiled disabled")
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(objs).NotTo(BeNil())
			Expect(len(objs)).To(BeNumerically(">", 0))
		})

		It("Should succeed when TagExists returns false without error and ForcePrecompiled is disabled", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy-success-test"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				ForcePrecompiled: false,
			}

			node := getNode("node1", kernelFull1)
			infoProvider := nodeinfo.NewProvider([]*v1.Node{node})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &errorOfedImageProvider{
				tagExists: false,
				err:       nil,
			})

			By("Calling GetManifestObjects with TagExists returning false without error")
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(objs).NotTo(BeNil())
			Expect(len(objs)).To(BeNumerically(">", 0))
		})

		It("Should succeed when TagExists returns true without error", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ofed-driver"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ofedState := stateOFED{
				stateSkel: stateSkel{
					name:        stateOFEDName,
					description: stateOFEDDescription,
					client:      &client,
					renderer:    renderer,
				},
			}

			cr := &v1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy-success-test"
			cr.Spec.OFEDDriver = &v1alpha1.OFEDDriverSpec{
				ImageSpec: v1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/mellanox",
					Version:    "23.10-0.5.5.0",
				},
				ForcePrecompiled: true,
			}

			node := getNode("node1", kernelFull1)
			infoProvider := nodeinfo.NewProvider([]*v1.Node{node})
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			catalog.Add(InfoTypeNodeInfo, infoProvider)
			catalog.Add(InfoTypeDocaDriverImage, &errorOfedImageProvider{
				tagExists: true,
				err:       nil,
			})

			By("Calling GetManifestObjects with TagExists returning true without error")
			objs, err := ofedState.GetManifestObjects(ctx, cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(objs).NotTo(BeNil())
			Expect(len(objs)).To(BeNumerically(">", 0))
		})
	})

	Context("addDefaultDaemonSetTolerations", func() {
		It("Should add all default DaemonSet tolerations to an empty list", func() {
			result := addDefaultDaemonSetTolerations(nil)

			// Should have 8 default tolerations: GPU + 7 DaemonSet defaults
			Expect(len(result)).To(Equal(8))

			// Verify each expected toleration is present
			expectedKeys := []string{
				"nvidia.com/gpu",
				v1.TaintNodeNotReady,
				v1.TaintNodeUnreachable,
				v1.TaintNodeDiskPressure,
				v1.TaintNodeMemoryPressure,
				v1.TaintNodePIDPressure,
				v1.TaintNodeUnschedulable,
				v1.TaintNodeNetworkUnavailable,
			}

			for _, key := range expectedKeys {
				found := false
				for _, tol := range result {
					if tol.Key == key {
						found = true
						Expect(tol.Operator).To(Equal(v1.TolerationOpExists))
						break
					}
				}
				Expect(found).To(BeTrue(), fmt.Sprintf("Expected toleration with key %s to be present", key))
			}
		})

		It("Should not add duplicate tolerations when GPU toleration already exists", func() {
			existingTolerations := []v1.Toleration{
				{
					Key:      "nvidia.com/gpu",
					Effect:   v1.TaintEffectNoSchedule,
					Operator: v1.TolerationOpExists,
				},
			}

			result := addDefaultDaemonSetTolerations(existingTolerations)

			// Should have 8 total: 1 existing GPU + 7 new DaemonSet defaults
			Expect(len(result)).To(Equal(8))

			// Count GPU tolerations - should only be 1
			gpuCount := 0
			for _, tol := range result {
				if tol.Key == "nvidia.com/gpu" {
					gpuCount++
				}
			}
			Expect(gpuCount).To(Equal(1), "Should not have duplicate GPU tolerations")
		})

		It("Should not add duplicate tolerations when unschedulable toleration already exists", func() {
			existingTolerations := []v1.Toleration{
				{
					Key:      v1.TaintNodeUnschedulable,
					Effect:   v1.TaintEffectNoSchedule,
					Operator: v1.TolerationOpExists,
				},
			}

			result := addDefaultDaemonSetTolerations(existingTolerations)

			// Should have 8 total: 1 existing unschedulable + 7 new defaults
			Expect(len(result)).To(Equal(8))

			// Count unschedulable tolerations - should only be 1
			unschedulableCount := 0
			for _, tol := range result {
				if tol.Key == v1.TaintNodeUnschedulable {
					unschedulableCount++
				}
			}
			Expect(unschedulableCount).To(Equal(1), "Should not have duplicate unschedulable tolerations")
		})

		It("Should preserve user-defined custom tolerations", func() {
			customTolerations := []v1.Toleration{
				{
					Key:      "custom-taint",
					Effect:   v1.TaintEffectNoSchedule,
					Operator: v1.TolerationOpEqual,
					Value:    "true",
				},
				{
					Key:      "another-custom-taint",
					Effect:   v1.TaintEffectNoExecute,
					Operator: v1.TolerationOpExists,
				},
			}

			result := addDefaultDaemonSetTolerations(customTolerations)

			// Should have 10 total: 2 custom + 8 defaults
			Expect(len(result)).To(Equal(10))

			// Verify custom tolerations are preserved
			foundCustom1 := false
			foundCustom2 := false
			for _, tol := range result {
				if tol.Key == "custom-taint" {
					foundCustom1 = true
					Expect(tol.Value).To(Equal("true"))
					Expect(tol.Operator).To(Equal(v1.TolerationOpEqual))
				}
				if tol.Key == "another-custom-taint" {
					foundCustom2 = true
					Expect(tol.Effect).To(Equal(v1.TaintEffectNoExecute))
				}
			}
			Expect(foundCustom1).To(BeTrue(), "First custom toleration should be preserved")
			Expect(foundCustom2).To(BeTrue(), "Second custom toleration should be preserved")
		})

		It("Should not add duplicates when all default tolerations already exist", func() {
			// Start with all default tolerations
			existingTolerations := []v1.Toleration{
				{Key: "nvidia.com/gpu", Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
				{Key: v1.TaintNodeNotReady, Effect: v1.TaintEffectNoExecute, Operator: v1.TolerationOpExists},
				{Key: v1.TaintNodeUnreachable, Effect: v1.TaintEffectNoExecute, Operator: v1.TolerationOpExists},
				{Key: v1.TaintNodeDiskPressure, Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
				{Key: v1.TaintNodeMemoryPressure, Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
				{Key: v1.TaintNodePIDPressure, Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
				{Key: v1.TaintNodeUnschedulable, Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
				{Key: v1.TaintNodeNetworkUnavailable, Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
			}

			result := addDefaultDaemonSetTolerations(existingTolerations)

			// Should still have exactly 8 - no duplicates added
			Expect(len(result)).To(Equal(8))
		})

		It("Should handle mixed scenario with some existing and some new tolerations", func() {
			mixedTolerations := []v1.Toleration{
				// User's custom toleration
				{Key: "custom-taint", Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
				// One default toleration that user explicitly added
				{Key: v1.TaintNodeDiskPressure, Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists},
				// Another user custom toleration
				{Key: "another-custom", Effect: v1.TaintEffectNoExecute, Operator: v1.TolerationOpExists},
			}

			result := addDefaultDaemonSetTolerations(mixedTolerations)

			// Should have 10 total: 3 existing (2 custom + 1 default) + 7 new defaults
			Expect(len(result)).To(Equal(10))

			// Verify disk pressure toleration appears only once
			diskPressureCount := 0
			for _, tol := range result {
				if tol.Key == v1.TaintNodeDiskPressure {
					diskPressureCount++
				}
			}
			Expect(diskPressureCount).To(Equal(1), "Disk pressure toleration should not be duplicated")

			// Verify all defaults are present
			expectedDefaultKeys := []string{
				"nvidia.com/gpu",
				v1.TaintNodeNotReady,
				v1.TaintNodeUnreachable,
				v1.TaintNodeDiskPressure,
				v1.TaintNodeMemoryPressure,
				v1.TaintNodePIDPressure,
				v1.TaintNodeUnschedulable,
				v1.TaintNodeNetworkUnavailable,
			}

			for _, key := range expectedDefaultKeys {
				found := false
				for _, tol := range result {
					if tol.Key == key {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), fmt.Sprintf("Default toleration %s should be present", key))
			}
		})

		It("Should handle empty slice input", func() {
			emptySlice := []v1.Toleration{}
			result := addDefaultDaemonSetTolerations(emptySlice)

			// Should have 8 default tolerations
			Expect(len(result)).To(Equal(8))
		})
	})
})

func getOfedState() *stateOFED {
	client := mocks.ControllerRuntimeClient{}
	manifestBaseDir := "../../manifests/state-ofed-driver"

	files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
	Expect(err).NotTo(HaveOccurred())
	renderer := render.NewRenderer(files)

	ofedState := &stateOFED{
		stateSkel: stateSkel{
			name:        stateOFEDName,
			description: stateOFEDDescription,
			client:      &client,
			renderer:    renderer,
		},
	}
	return ofedState
}

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

func verifySubscriptionMountsRhel(mounts []v1.VolumeMount) {
	By("Verify Subscription Mounts")
	sub0 := v1.VolumeMount{
		Name:             "subscription-config-0",
		ReadOnly:         true,
		MountPath:        "/run/secrets/etc-pki-entitlement",
		SubPath:          "",
		MountPropagation: nil,
		SubPathExpr:      "",
	}
	Expect(slices.Contains(mounts, sub0)).To(BeTrue())
	sub1 := v1.VolumeMount{
		Name:             "subscription-config-1",
		ReadOnly:         true,
		MountPath:        "/run/secrets/redhat.repo",
		SubPath:          "",
		MountPropagation: nil,
		SubPathExpr:      "",
	}
	Expect(slices.Contains(mounts, sub1)).To(BeTrue())
	sub2 := v1.VolumeMount{
		Name:             "subscription-config-2",
		ReadOnly:         true,
		MountPath:        "/run/secrets/rhsm",
		SubPath:          "",
		MountPropagation: nil,
		SubPathExpr:      "",
	}
	Expect(slices.Contains(mounts, sub2)).To(BeTrue())
}

func verifySubscriptionVolumesRhel(volumes []v1.Volume) {
	By("Verify Subscription Volumes")
	sub0 := v1.Volume{
		Name: "subscription-config-0",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/pki/entitlement",
				Type: newHostPathType(v1.HostPathDirectory),
			},
		},
	}
	sub1 := v1.Volume{
		Name: "subscription-config-1",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/yum.repos.d/redhat.repo",
				Type: newHostPathType(v1.HostPathFile),
			},
		},
	}
	sub2 := v1.Volume{
		Name: "subscription-config-2",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/rhsm",
				Type: newHostPathType(v1.HostPathDirectory),
			},
		},
	}
	foundSub0 := false
	foundSub1 := false
	foundSub2 := false
	for i := range volumes {
		if volumes[i].Name == "subscription-config-0" {
			Expect(volumes[i]).To(BeEquivalentTo(sub0))
			foundSub0 = true
		}
		if volumes[i].Name == "subscription-config-1" {
			Expect(volumes[i]).To(BeEquivalentTo(sub1))
			foundSub1 = true
		}
		if volumes[i].Name == "subscription-config-2" {
			Expect(volumes[i]).To(BeEquivalentTo(sub2))
			foundSub2 = true
		}
	}
	Expect(foundSub0).To(BeTrue())
	Expect(foundSub1).To(BeTrue())
	Expect(foundSub2).To(BeTrue())
}

func verifySubscriptionMountsSles(mounts []v1.VolumeMount) {
	By("Verify Subscription Mounts")
	sub0 := v1.VolumeMount{
		Name:             "subscription-config-0",
		ReadOnly:         true,
		MountPath:        "/etc/SUSEConnect",
		SubPath:          "",
		MountPropagation: nil,
		SubPathExpr:      "",
	}
	Expect(slices.Contains(mounts, sub0)).To(BeTrue())
	sub1 := v1.VolumeMount{
		Name:             "subscription-config-1",
		ReadOnly:         true,
		MountPath:        "/etc/zypp/credentials.d",
		SubPath:          "",
		MountPropagation: nil,
		SubPathExpr:      "",
	}
	Expect(slices.Contains(mounts, sub1)).To(BeTrue())
}

func verifySubscriptionVolumesSles(volumes []v1.Volume) {
	By("Verify Subscription Volumes")
	sub0 := v1.Volume{
		Name: "subscription-config-0",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/SUSEConnect",
				Type: newHostPathType(v1.HostPathFileOrCreate),
			},
		},
	}
	sub1 := v1.Volume{
		Name: "subscription-config-1",
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/zypp/credentials.d",
				Type: newHostPathType(v1.HostPathDirectory),
			},
		},
	}
	foundSub0 := false
	foundSub1 := false
	for i := range volumes {
		if volumes[i].Name == "subscription-config-0" {
			Expect(volumes[i]).To(BeEquivalentTo(sub0))
			foundSub0 = true
		}
		if volumes[i].Name == "subscription-config-1" {
			Expect(volumes[i]).To(BeEquivalentTo(sub1))
			foundSub1 = true
		}
	}
	Expect(foundSub0).To(BeTrue())
	Expect(foundSub1).To(BeTrue())
}

func verifyAdditionalMounts(mounts []v1.VolumeMount) {
	By("Verify Additional Mounts")
	repo := v1.VolumeMount{
		Name:             "repo-cm",
		ReadOnly:         true,
		MountPath:        "/etc/apt/sources.list.d/ubi.repo",
		SubPath:          "ubi.repo",
		MountPropagation: nil,
		SubPathExpr:      "",
	}
	Expect(slices.Contains(mounts, repo)).To(BeTrue())
	cert := v1.VolumeMount{
		Name:             "cert-cm",
		ReadOnly:         true,
		MountPath:        "/usr/local/share/ca-certificates/my-cert",
		SubPath:          "my-cert",
		MountPropagation: nil,
		SubPathExpr:      "",
	}
	Expect(slices.Contains(mounts, cert)).To(BeTrue())
}

func verifyAdditionalVolumes(volumes []v1.Volume) {
	By("Verify Additional Volumes")
	certVol := v1.Volume{
		Name: "cert-cm",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "cert-cm",
				},
				Items: []v1.KeyToPath{
					{
						Key:  "my-cert",
						Path: "my-cert",
					},
				},
			},
		},
	}
	repoVol := v1.Volume{
		Name: "repo-cm",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "repo-cm",
				},
				Items: []v1.KeyToPath{
					{
						Key:  "ubi.repo",
						Path: "ubi.repo",
					},
				},
			},
		},
	}
	foundCert := false
	foundRepo := false
	for i := range volumes {
		if volumes[i].Name == "cert-cm" {
			Expect(volumes[i]).To(BeEquivalentTo(certVol))
			foundCert = true
		}
		if volumes[i].Name == "repo-cm" {
			Expect(volumes[i]).To(BeEquivalentTo(repoVol))
			foundRepo = true
		}
	}
	Expect(foundCert).To(BeTrue())
	Expect(foundRepo).To(BeTrue())
}

func verifyDSNodeSelector(selector map[string]string, kernelFull string) {
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
	nsKernelMinor, ok := selector["feature.node.kubernetes.io/kernel-version.full"]
	Expect(ok).To(BeTrue())
	Expect(nsKernelMinor).To(Equal(kernelFull))
}

func getNode(name, kernelFull string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				nodeinfo.NodeLabelMlnxNIC:       "true",
				nodeinfo.NodeLabelOSName:        osName,
				nodeinfo.NodeLabelOSVer:         osVer,
				nodeinfo.NodeLabelKernelVerFull: kernelFull,
				nodeinfo.NodeLabelCPUArch:       "amd64",
			},
		},
	}
}

func setContainerRuntime(node *v1.Node, containerRuntime string) {
	node.Status = v1.NodeStatus{
		NodeInfo: v1.NodeSystemInfo{
			ContainerRuntimeVersion: containerRuntime,
		},
	}
}
