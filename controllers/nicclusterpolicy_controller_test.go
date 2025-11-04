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

package controllers //nolint:dupl

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
)

var _ = Describe("NicClusterPolicyReconciler Controller", func() {
	Context("When NicClusterPolicy CR is created", func() {
		It("should create nv-ipam and delete it after un-setting CR value", func() {
			By("Check NicClusterPolicy with nv-ipam")
			cr := mellanoxv1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nic-cluster-policy",
					Namespace: "",
				},
				Spec: mellanoxv1alpha1.NicClusterPolicySpec{
					NvIpam: &mellanoxv1alpha1.NVIPAMSpec{
						ImageSpec: mellanoxv1alpha1.ImageSpec{
							Image:            "nvidia-k8s-ipam",
							Repository:       "ghcr.io/mellanox",
							Version:          "v0.3.7",
							ImagePullSecrets: []string{},
						},
					},
				},
			}

			err := k8sClient.Create(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			ncp := &mellanoxv1alpha1.NicClusterPolicy{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, ncp)
			Expect(err).NotTo(HaveOccurred())

			By("Check DS created with state label")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "nv-ipam-node"}, ds)
				if err != nil {
					return false
				}
				l, ok := ds.Labels[consts.StateLabel]
				if !ok {
					return false
				}
				return l == "state-nv-ipam-cni"
			}, timeout*3, interval).Should(BeTrue())

			By("Check SA created with state label")
			Eventually(func() bool {
				ds := &corev1.ServiceAccount{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "nv-ipam-node"}, ds)
				if err != nil {
					return false
				}
				l, ok := ds.Labels[consts.StateLabel]
				if !ok {
					return false
				}
				return l == "state-nv-ipam-cni"
			}, timeout*3, interval).Should(BeTrue())

			By("Update CR to remove nv-ipam")
			ncp = &mellanoxv1alpha1.NicClusterPolicy{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, ncp)
			Expect(err).NotTo(HaveOccurred())

			ncp.Spec.NvIpam = nil
			err = k8sClient.Update(context.TODO(), ncp)
			Expect(err).NotTo(HaveOccurred())

			By("Check DS is deleted")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "nv-ipam-node"}, ds)
				return apierrors.IsNotFound(err)
			}, timeout*3, interval).Should(BeTrue())

			By("Check SA is deleted")
			Eventually(func() bool {
				sa := &corev1.ServiceAccount{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "nv-ipam-node"}, sa)
				return apierrors.IsNotFound(err)
			}, timeout*3, interval).Should(BeTrue())

			By("Delete NicClusterPolicy")
			err = k8sClient.Delete(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Unsupported name", func() {
			cr := mellanoxv1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "",
				},
			}
			err := k8sClient.Create(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() string {
				found := &mellanoxv1alpha1.NicClusterPolicy{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
				Expect(err).NotTo(HaveOccurred())
				return string(found.Status.State)
			}, timeout*3, interval).Should(BeEquivalentTo(mellanoxv1alpha1.StateIgnore))

			err = k8sClient.Delete(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("When MOFED precompiled tag does not exists", func() {
		It("should set error message in status", func() {
			By("Create Node")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						nodeinfo.NodeLabelMlnxNIC:       "true",
						nodeinfo.NodeLabelOSName:        "ubuntu",
						nodeinfo.NodeLabelCPUArch:       "amd64",
						nodeinfo.NodeLabelKernelVerFull: "generic-9.0.1",
						nodeinfo.NodeLabelOSVer:         "20.0.4"},
					Annotations: make(map[string]string),
				},
			}
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).NotTo(HaveOccurred())
			By("Create NicClusterPolicy with MOFED ForcePrecompiled")
			cr := mellanoxv1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nic-cluster-policy",
					Namespace: "",
				},
				Spec: mellanoxv1alpha1.NicClusterPolicySpec{
					OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{
						ForcePrecompiled: true,
						ImageSpec: mellanoxv1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "acme.buzz",
							Version:          "5.9-0.5.6.0",
							ImagePullSecrets: []string{},
						},
					},
					// The node created by the API call is subject to default taints applied by Kubernetes
					// (e.g., "node.kubernetes.io/not-ready:NoSchedule" for new nodes).
					// This toleration is added to the NicClusterPolicy to ensure the OFED driver DaemonSet
					// can be scheduled on the test node by matching such taints.
					Tolerations: []corev1.Toleration{
						{
							Key:      "node.kubernetes.io/not-ready",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			}

			err = k8sClient.Create(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			ncp := &mellanoxv1alpha1.NicClusterPolicy{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, ncp)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for NicClusterPolicy OFED state error message to be populated")
			msg := "failed to create k8s objects from manifest: " +
				"failed to render objects: ForcePrecompiled is enabled " +
				"and precompiled tag was not found: " +
				"5.9-0.5.6.0-generic-9.0.1-ubuntu20.0.4-amd64"

			Eventually(func() string {
				found := &mellanoxv1alpha1.NicClusterPolicy{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
				Expect(err).NotTo(HaveOccurred())
				return getAppliedStateMessage(found.Status.AppliedStates, "state-OFED")
			}, timeout*10, interval).Should(BeEquivalentTo(msg))

			By("Set MOFED ForcePrecompiled to false")
			patch := []byte(`{"spec": {"ofedDriver":{"forcePrecompiled": false}}}`)
			Expect(k8sClient.Patch(context.TODO(), &cr, client.RawPatch(types.MergePatchType, patch))).To(Succeed())

			By("Wait for NicClusterPolicy OFED state error message to be cleared")
			msg = ""
			Eventually(func() string {
				found := &mellanoxv1alpha1.NicClusterPolicy{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
				Expect(err).NotTo(HaveOccurred())
				return getAppliedStateMessage(found.Status.AppliedStates, "state-OFED")
			}, timeout*10, interval).Should(BeEquivalentTo(msg))

			By("Delete NicClusterPolicy")
			err = k8sClient.Delete(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			By("Delete Node")
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("When NicClusterPolicy CR is deleted", func() {
		It("should set mofed.wait and nic-configuration.wait to false", func() {
			By("Create Node")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Labels:      make(map[string]string),
					Annotations: make(map[string]string),
				},
			}
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).NotTo(HaveOccurred())
			By("Create NicClusterPolicy")
			cr := mellanoxv1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nic-cluster-policy",
					Namespace: "",
				},
				Spec: mellanoxv1alpha1.NicClusterPolicySpec{
					OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{
						ImageSpec: mellanoxv1alpha1.ImageSpec{
							Image:            "mofed",
							Repository:       "nvcr.io/nvidia/mellanox",
							Version:          "5.9-0.5.6.0",
							ImagePullSecrets: []string{},
						},
					},
					NicConfigurationOperator: &mellanoxv1alpha1.NicConfigurationOperatorSpec{
						Operator: &mellanoxv1alpha1.ImageSpec{
							Image:            "nic-configuration-operator",
							Repository:       "nvcr.io/nvidia/mellanox",
							Version:          "1.0.0",
							ImagePullSecrets: []string{},
						},
						ConfigurationDaemon: &mellanoxv1alpha1.ImageSpec{
							Image:            "nic-configuration-daemon",
							Repository:       "nvcr.io/nvidia/mellanox",
							Version:          "1.0.0",
							ImagePullSecrets: []string{},
						},
					},
				},
			}

			err = k8sClient.Create(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			ncp := &mellanoxv1alpha1.NicClusterPolicy{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, ncp)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for NicClusterPolicy state to be populated")
			Eventually(func() string {
				found := &mellanoxv1alpha1.NicClusterPolicy{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, found)
				Expect(err).NotTo(HaveOccurred())
				return string(found.Status.State)
			}, timeout*3, interval).Should(BeEquivalentTo(mellanoxv1alpha1.StateNotReady))

			By("Update Node labels")
			n := &corev1.Node{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: node.GetNamespace(), Name: node.GetName()}, n)
			Expect(err).NotTo(HaveOccurred())

			patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:"true", %q:"true", %q:"true"}}}`,
				nodeinfo.NodeLabelWaitOFED, nodeinfo.NodeLabelMlnxNIC, nodeinfo.NodeLabelWaitNicConfig))
			err = k8sClient.Patch(context.TODO(), n, client.RawPatch(types.StrategicMergePatchType, patch))
			Expect(err).NotTo(HaveOccurred())

			Consistently(func() bool {
				n := &corev1.Node{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: node.GetNamespace(), Name: node.GetName()}, n)
				if err != nil {
					return false
				}
				return n.ObjectMeta.Labels[nodeinfo.NodeLabelWaitOFED] == "true" &&
					n.ObjectMeta.Labels[nodeinfo.NodeLabelWaitNicConfig] == "true"
			}, timeout, interval).Should(BeTrue())

			By("Delete NicClusterPolicy")
			err = k8sClient.Delete(context.TODO(), &cr)
			Expect(err).NotTo(HaveOccurred())

			By("Verify Mofed Label is false")
			Eventually(func() bool {
				n := &corev1.Node{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: node.GetNamespace(), Name: node.GetName()}, n)
				if err != nil {
					return false
				}
				return n.ObjectMeta.Labels[nodeinfo.NodeLabelWaitOFED] == "false"
			}, timeout*3, interval).Should(BeTrue())

			By("Verify Nic Configuration Wait Label is false")
			Eventually(func() bool {
				n := &corev1.Node{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: node.GetNamespace(), Name: node.GetName()}, n)
				if err != nil {
					return false
				}
				return n.ObjectMeta.Labels[nodeinfo.NodeLabelWaitNicConfig] == "false"
			}, timeout*3, interval).Should(BeTrue())

			By("Delete Node")
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Create DOCATelemetryService deployment through NICClusterPolicy", func() {
		stateLabel := "state-doca-telemetry-service"
		imageRepo := "nvcr.io/nvidia/doca"
		imageName := "doca-telemetry-service"
		imageVersion := "1.15.5-doca2.5.0"
		updatedVersion := "sha256:1699d23027ea30c9fa"
		ctx := context.Background()
		It("should create, update and delete doca-telemetry-service through NICClusterPolicy", func() {
			By("Create doca-telemetry-service through NICClusterPolicy")
			cr := &mellanoxv1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nic-cluster-policy",
					Namespace: "test-namespace",
				},
				Spec: mellanoxv1alpha1.NicClusterPolicySpec{
					DOCATelemetryService: &mellanoxv1alpha1.DOCATelemetryServiceSpec{
						ImageSpec: mellanoxv1alpha1.ImageSpec{
							Image:      imageName,
							Repository: imageRepo,
							Version:    imageVersion,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cr)).To(Succeed())

			By("Check DaemonSet is correctly created")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: "doca-telemetry-service"}, ds)
				if err != nil {
					return false
				}
				l, ok := ds.Labels[consts.StateLabel]
				if !ok {
					return false
				}
				return l == stateLabel
			}, timeout*3, interval).Should(BeTrue())

			By("Check ConfigMap is created with state label")
			Eventually(func() bool {
				cm := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: "doca-telemetry-service"}, cm)
				if err != nil {
					return false
				}
				l, ok := cm.Labels[consts.StateLabel]
				if !ok {
					return false
				}
				return l == stateLabel
			}, timeout*3, interval).Should(BeTrue())

			By("Update DOCATelemetryService through NICClusterPolicy")
			expectedImageName := fmt.Sprintf("%v/%v@%v", imageRepo, imageName, updatedVersion)

			// Patch the NICClusterPolicy with the updated DOCATelemetryService version number.
			patch := []byte(fmt.Sprintf(`{"spec": {"docaTelemetryService":{"version": %q}}}`, updatedVersion))
			Expect(k8sClient.Patch(ctx, cr, client.RawPatch(types.MergePatchType, patch))).To(Succeed())

			// Expect the image name in the Daemonset to be updated with the new version.
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: "doca-telemetry-service"}, ds)
				if err != nil {
					return false
				}
				return ds.Spec.Template.Spec.Containers[0].Image == expectedImageName
			}, timeout*3, interval).Should(BeTrue())

			By("Delete DOCATelemetryService through NICClusterPolicy")

			// Patch the NICClusterPolicy to drop the DOCATelemetryService.
			patch = []byte(`{"spec": {"docaTelemetryService": null}}`)
			Expect(k8sClient.Patch(ctx, cr, client.RawPatch(types.MergePatchType, patch))).To(Succeed())

			// Expect the DaemonSet to be NotFound by the client.
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: "doca-telemetry-service"}, ds)
				return apierrors.IsNotFound(err)
			}, timeout*3, interval).Should(BeTrue())

			// Expect the ConfigMap to be NotFound by the client.
			Eventually(func() bool {
				cm := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: "doca-telemetry-service"}, cm)
				return apierrors.IsNotFound(err)
			}, timeout*3, interval).Should(BeTrue())

			By("Delete NICClusterPolicy")
			Expect(k8sClient.Delete(ctx, cr)).To(Succeed())
		})
	})
})

const (
	stateOFEDName = "state-OFED" // Assuming this is the value for the state label for OFED
)

// Helper function to create a new Node for testing
func newTestNode(name, kernelVersion, osName, osVer, arch string, taints []corev1.Taint) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				nodeinfo.NodeLabelMlnxNIC:       "true",
				nodeinfo.NodeLabelOSName:        osName,
				nodeinfo.NodeLabelOSVer:         osVer,
				nodeinfo.NodeLabelKernelVerFull: kernelVersion,
				nodeinfo.NodeLabelCPUArch:       arch,
				// Add a unique label to help select this node if needed, though not strictly used in these tests for DS directly
				"test-node-label": name,
			},
		},
		Spec: corev1.NodeSpec{
			Taints: taints,
		},
		Status: corev1.NodeStatus{ // Add basic status to make node appear more realistic to controller
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

// Helper function to create a NicClusterPolicy with OFEDDriver spec
func newOfedNicClusterPolicy(name string, tolerations []corev1.Toleration) *mellanoxv1alpha1.NicClusterPolicy {
	return &mellanoxv1alpha1.NicClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			// Namespace: "test-namespace", // NCP is cluster-scoped
		},
		Spec: mellanoxv1alpha1.NicClusterPolicySpec{
			OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{
				ImageSpec: mellanoxv1alpha1.ImageSpec{
					Image:      "mofed",
					Repository: "nvcr.io/nvidia/mellanox", // Using a common repo
					Version:    "5.9-0.5.6.0",             // A version used in other tests
				},
			},
			Tolerations: tolerations,
		},
	}
}

var _ = Describe("NicClusterPolicyReconciler Controller - Taint and Toleration Interactions", func() {
	const (
		testNodeNamePrefix   = "taint-test-node-"
		testOSName           = "ubuntu"
		testOSVer            = "20.04"
		testKernelVersion    = "5.15.0-testkernel"
		testArch             = "amd64"
		nicClusterPolicyName = "nic-cluster-policy"
		customTaintKey       = "custom/test-taint"
		customTaintValue     = "true"
		customTaintEffect    = corev1.TaintEffectNoSchedule
	)

	ctx := context.Background()

	// Default toleration for newly created nodes in test environments
	defaultNodeTolerations := []corev1.Toleration{
		{
			Key:      "node.kubernetes.io/not-ready",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	Context("When Node taint changes and NicClusterPolicy tolerations are static", func() {
		It("should create OFED DaemonSet initially and delete it after Node is tainted", func() {
			nodeName := fmt.Sprintf("%s%s", testNodeNamePrefix, "test1")
			node := newTestNode(nodeName, testKernelVersion, testOSName, testOSVer, testArch, nil) // No taints initially
			Expect(k8sClient.Create(ctx, node)).Should(Succeed())
			defer func() { Expect(k8sClient.Delete(ctx, node)).Should(Succeed()) }()

			ncp := newOfedNicClusterPolicy(nicClusterPolicyName, defaultNodeTolerations)
			Expect(k8sClient.Create(ctx, ncp)).Should(Succeed())
			defer func() { Expect(k8sClient.Delete(ctx, ncp)).Should(Succeed()) }()

			By("Verifying OFED DaemonSet is created for the untainted node")
			Eventually(func(g Gomega) {
				dsList := &appsv1.DaemonSetList{}
				g.Expect(k8sClient.List(ctx, dsList, client.InNamespace(namespaceName),
					client.MatchingLabels{consts.StateLabel: stateOFEDName})).Should(Succeed())
				g.Expect(dsList.Items).To(HaveLen(1), "Expected 1 OFED DaemonSet")
			}, timeout*3, interval).Should(Succeed())

			By("Adding a taint to the Node")
			updatedNode := &corev1.Node{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, updatedNode)).Should(Succeed())
			updatedNode.Spec.Taints = []corev1.Taint{
				{Key: customTaintKey, Value: customTaintValue, Effect: customTaintEffect},
			}
			Expect(k8sClient.Update(ctx, updatedNode)).Should(Succeed())

			By("Verifying OFED DaemonSet is deleted as NCP does not tolerate the new taint")
			Eventually(func(g Gomega) {
				dsList := &appsv1.DaemonSetList{}
				g.Expect(k8sClient.List(ctx, dsList, client.InNamespace(namespaceName),
					client.MatchingLabels{consts.StateLabel: stateOFEDName})).Should(Succeed())
				g.Expect(dsList.Items).To(BeEmpty(), "Expected 0 OFED DaemonSets after node taint")
			}, timeout*3, interval).Should(Succeed())
		})
	})

	Context("When NicClusterPolicy tolerations change for a tainted Node", func() {
		It("should not create OFED DaemonSet initially and create it after NCP toleration is added", func() {
			nodeName := fmt.Sprintf("%s%s", testNodeNamePrefix, "test2")
			nodeTaints := []corev1.Taint{
				{Key: customTaintKey, Value: customTaintValue, Effect: customTaintEffect},
			}
			node := newTestNode(nodeName, testKernelVersion, testOSName, testOSVer, testArch, nodeTaints)
			Expect(k8sClient.Create(ctx, node)).Should(Succeed())
			defer func() { Expect(k8sClient.Delete(ctx, node)).Should(Succeed()) }()

			ncp := newOfedNicClusterPolicy(nicClusterPolicyName, defaultNodeTolerations) // Initially no custom toleration
			Expect(k8sClient.Create(ctx, ncp)).Should(Succeed())
			defer func() { Expect(k8sClient.Delete(ctx, ncp)).Should(Succeed()) }()

			By("Verifying no OFED DaemonSet is created for the tainted node without specific toleration")
			Consistently(func(g Gomega) {
				dsList := &appsv1.DaemonSetList{}
				g.Expect(k8sClient.List(ctx, dsList, client.InNamespace(namespaceName),
					client.MatchingLabels{consts.StateLabel: stateOFEDName})).Should(Succeed())
				g.Expect(dsList.Items).To(BeEmpty(), "Expected 0 OFED DaemonSets initially for tainted node")
			}, timeout, interval*2).Should(Succeed()) // Check consistently for a period

			By("Updating NicClusterPolicy to add toleration for the Node's taint")
			updatedNcp := &mellanoxv1alpha1.NicClusterPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nicClusterPolicyName}, updatedNcp)).Should(Succeed())
			updatedNcp.Spec.Tolerations = append(updatedNcp.Spec.Tolerations, corev1.Toleration{
				Key:      customTaintKey,
				Operator: corev1.TolerationOpEqual,
				Value:    customTaintValue,
				Effect:   customTaintEffect,
			})
			Expect(k8sClient.Update(ctx, updatedNcp)).Should(Succeed())

			By("Verifying OFED DaemonSet is created after NCP toleration is added")
			Eventually(func(g Gomega) {
				dsList := &appsv1.DaemonSetList{}
				g.Expect(k8sClient.List(ctx, dsList, client.InNamespace(namespaceName),
					client.MatchingLabels{consts.StateLabel: stateOFEDName})).Should(Succeed())
				g.Expect(dsList.Items).To(HaveLen(1), "Expected 1 OFED DaemonSet after adding toleration")
			}, timeout*5, interval).Should(Succeed())
		})
	})
})

func getAppliedStateMessage(states []mellanoxv1alpha1.AppliedState, stateName string) string {
	for _, state := range states {
		if state.Name == stateName {
			return state.Message
		}
	}
	return ""
}
