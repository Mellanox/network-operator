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
		It("should create whereabouts and delete it after un-setting CR value", func() {
			By("Check NicClusterPolicy with whereabouts")
			cr := mellanoxv1alpha1.NicClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nic-cluster-policy",
					Namespace: "",
				},
				Spec: mellanoxv1alpha1.NicClusterPolicySpec{
					SecondaryNetwork: &mellanoxv1alpha1.SecondaryNetworkSpec{
						IpamPlugin: &mellanoxv1alpha1.ImageSpec{
							Image:            "whereabouts",
							Repository:       "ghcr.io/k8snetworkplumbingwg",
							Version:          "v0.5.4-amd64",
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
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "whereabouts"}, ds)
				if err != nil {
					return false
				}
				l, ok := ds.Labels[consts.StateLabel]
				if !ok {
					return false
				}
				return l == "state-whereabouts-cni"
			}, timeout*3, interval).Should(BeTrue())

			By("Check SA created with state label")
			Eventually(func() bool {
				ds := &corev1.ServiceAccount{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "whereabouts"}, ds)
				if err != nil {
					return false
				}
				l, ok := ds.Labels[consts.StateLabel]
				if !ok {
					return false
				}
				return l == "state-whereabouts-cni"
			}, timeout*3, interval).Should(BeTrue())

			By("Update CR to remove whereabout")
			ncp = &mellanoxv1alpha1.NicClusterPolicy{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, ncp)
			Expect(err).NotTo(HaveOccurred())

			ncp.Spec.SecondaryNetwork = nil
			err = k8sClient.Update(context.TODO(), ncp)
			Expect(err).NotTo(HaveOccurred())

			By("Check DS is deleted")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "whereabouts"}, ds)
				return apierrors.IsNotFound(err)
			}, timeout*3, interval).Should(BeTrue())

			By("Check SA is deleted")
			Eventually(func() bool {
				sa := &corev1.ServiceAccount{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: "whereabouts"}, sa)
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
		It("should set mofed.wait to false", func() {
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

			patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:"true", %q:"true"}}}`,
				nodeinfo.NodeLabelWaitOFED, nodeinfo.NodeLabelMlnxNIC))
			err = k8sClient.Patch(context.TODO(), n, client.RawPatch(types.StrategicMergePatchType, patch))
			Expect(err).NotTo(HaveOccurred())

			Consistently(func() bool {
				n := &corev1.Node{}
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: node.GetNamespace(), Name: node.GetName()}, n)
				if err != nil {
					return false
				}
				return n.ObjectMeta.Labels[nodeinfo.NodeLabelWaitOFED] == "true"
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

func getAppliedStateMessage(states []mellanoxv1alpha1.AppliedState, stateName string) string {
	for _, state := range states {
		if state.Name == stateName {
			return state.Message
		}
	}
	return ""
}
