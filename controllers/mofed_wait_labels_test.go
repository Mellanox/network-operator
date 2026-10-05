/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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

package controllers

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
)

type mofedWaitFixture struct {
	node    *corev1.Node
	pod     *corev1.Pod
	policy  mellanoxv1alpha1.NicPolicyCR
	dsOwner string
}

func newMOFEDWaitFixture(nodePolicy bool) *mofedWaitFixture {
	f := &mofedWaitFixture{
		node:    newTestNode("mofed-wait-node", "5.15.0-100-generic", "ubuntu", "22.04", "amd64", nil),
		pod:     &corev1.Pod{},
		policy:  nil,
		dsOwner: mellanoxv1alpha1.NicClusterPolicyCRDName,
	}
	spec := &mellanoxv1alpha1.OFEDDriverSpec{ImageSpec: mellanoxv1alpha1.ImageSpec{
		Image: "mofed", Repository: "acme.buzz", Version: "5.9-0.5.6.0", ImagePullSecrets: []string{},
	}}
	// The envtest API server adds this taint on Node creation; there is no
	// node controller to remove it when we set the Ready condition.
	tolerations := []corev1.Toleration{{
		Key: "node.kubernetes.io/not-ready", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule,
	}}
	if nodePolicy {
		f.policy = &mellanoxv1alpha1.NicNodePolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "mofed-wait-policy"},
			Spec: mellanoxv1alpha1.NicNodePolicySpec{
				NodeSelector: map[string]string{"test-node-label": f.node.Name}, OFEDDriver: spec, Tolerations: tolerations,
			},
		}
		f.dsOwner = mellanoxv1alpha1.NicNodePolicyShortName + "-" + f.policy.GetName()
	} else {
		f.policy = &mellanoxv1alpha1.NicClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: consts.NicClusterPolicyResourceName},
			Spec:       mellanoxv1alpha1.NicClusterPolicySpec{OFEDDriver: spec, Tolerations: tolerations},
		}
	}
	Expect(k8sClient.Create(ctx, f.policy)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(ctx, f.policy)).To(Succeed())
		Expect(k8sClient.Delete(ctx, f.pod, client.GracePeriodSeconds(0))).To(Succeed())
		Expect(k8sClient.Delete(ctx, f.node)).To(Succeed())
		// envtest has no garbage collector to delete the policy's DaemonSets.
		Expect(k8sClient.DeleteAllOf(ctx, &appsv1.DaemonSet{}, client.InNamespace(namespaceName),
			client.MatchingLabels{consts.DSOwnerLabel: f.dsOwner})).To(Succeed())
	})
	Expect(k8sClient.Create(ctx, f.node)).To(Succeed())
	f.setNodeReady(corev1.ConditionTrue)
	f.pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mofed-wait-pod", Namespace: namespaceName,
			Labels: map[string]string{consts.OfedDriverLabel: "", consts.DSOwnerLabel: f.dsOwner},
		},
		Spec: corev1.PodSpec{NodeName: f.node.Name, Containers: []corev1.Container{
			{Name: "mofed-container", Image: "acme.buzz/mofed:5.9-0.5.6.0"},
		}},
	}
	Expect(k8sClient.Create(ctx, f.pod)).To(Succeed())
	f.setDriverReady(true)
	f.markDaemonSetReady()
	f.expectWait("false")
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(f.policy), f.policy)).To(Succeed())
		g.Expect(f.policy.GetPolicyState()).To(Equal(mellanoxv1alpha1.State(mellanoxv1alpha1.StateReady)))
	}, timeout, interval).Should(Succeed())
	Consistently(func(g Gomega) {
		n := &corev1.Node{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(f.node), n)).To(Succeed())
		g.Expect(n.Labels[nodeinfo.NodeLabelWaitOFED]).To(Equal("false"))
	}, time.Duration(config.FromEnv().Controller.RequeueTimeSeconds)*time.Second+time.Second, interval).
		Should(Succeed())
	return f
}

func (f *mofedWaitFixture) setNodeReady(status corev1.ConditionStatus) {
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(f.node), f.node)).To(Succeed())
		f.node.Status.Conditions = []corev1.NodeCondition{}
		if status != "" {
			f.node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}
		}
		g.Expect(k8sClient.Status().Update(ctx, f.node)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

func (f *mofedWaitFixture) setDriverReady(ready bool) {
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(f.pod), f.pod)).To(Succeed())
	f.pod.Status = corev1.PodStatus{
		Phase:             corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "mofed-container", Ready: ready}},
	}
	Expect(k8sClient.Status().Update(ctx, f.pod)).To(Succeed())
}

func (f *mofedWaitFixture) daemonSet(g Gomega) *appsv1.DaemonSet {
	list := &appsv1.DaemonSetList{}
	g.Expect(k8sClient.List(ctx, list, client.InNamespace(namespaceName), client.MatchingLabels{
		consts.OfedDriverLabel: "", consts.DSOwnerLabel: f.dsOwner,
	})).To(Succeed())
	g.Expect(list.Items).To(HaveLen(1))
	return &list.Items[0]
}

func (f *mofedWaitFixture) markDaemonSetReady() {
	// envtest has no DaemonSet controller. Set status once, then leave it unchanged,
	// including while the node and driver become unready and recover.
	Eventually(func(g Gomega) {
		ds := f.daemonSet(g)
		ds.Status = appsv1.DaemonSetStatus{
			ObservedGeneration: ds.Generation, DesiredNumberScheduled: 1,
			CurrentNumberScheduled: 1, UpdatedNumberScheduled: 1, NumberReady: 1, NumberAvailable: 1,
		}
		g.Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

func (f *mofedWaitFixture) expectWait(value string) {
	f.expectWaitWithin(value, timeout)
}

func (f *mofedWaitFixture) expectWaitWithin(value string, deadline time.Duration) {
	Eventually(func(g Gomega) {
		n := &corev1.Node{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(f.node), n)).To(Succeed())
		g.Expect(n.Labels[nodeinfo.NodeLabelWaitOFED]).To(Equal(value))
	}, deadline, interval).Should(Succeed())
}

func (f *mofedWaitFixture) shouldRequeue() bool {
	var waiting bool
	var err error
	if nnp, ok := f.policy.(*mellanoxv1alpha1.NicNodePolicy); ok {
		waiting, err = (&NicNodePolicyReconciler{Client: k8sClient}).handleMOFEDWaitLabels(ctx, nnp)
	} else {
		waiting, err = (&NicClusterPolicyReconciler{Client: k8sClient}).handleMOFEDWaitLabels(
			ctx, f.policy.(*mellanoxv1alpha1.NicClusterPolicy))
	}
	Expect(err).NotTo(HaveOccurred())
	return waiting
}

var _ = Describe("MOFED wait readiness", func() {

	DescribeTable("reacts to node readiness without a pod or DaemonSet update",
		func(nodePolicy bool) {
			f := newMOFEDWaitFixture(nodePolicy)
			for _, status := range []corev1.ConditionStatus{corev1.ConditionFalse, corev1.ConditionUnknown, ""} {
				By("Setting only NodeReady to " + string(status) + " while the container stays ready")
				f.setNodeReady(status)
				f.expectWaitWithin("true", 2*time.Second)
				// Settle the label-triggered reconcile before testing recovery delivery.
				Consistently(func(g Gomega) {
					n := &corev1.Node{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(f.node), n)).To(Succeed())
					g.Expect(n.Labels[nodeinfo.NodeLabelWaitOFED]).To(Equal("true"))
				}, time.Second, interval).Should(Succeed())
				By("Recovering the node with the container already ready")
				f.setNodeReady(corev1.ConditionTrue)
				f.expectWaitWithin("false", 2*time.Second)
			}
			Expect(f.shouldRequeue()).To(BeFalse())
		},
		Entry("NicClusterPolicy", false),
		Entry("NicNodePolicy", true),
	)

	DescribeTable("retries driver recovery even when DaemonSet status stays Ready",
		func(nodePolicy bool) {
			f := newMOFEDWaitFixture(nodePolicy)
			before := f.daemonSet(Default)
			f.setNodeReady(corev1.ConditionFalse)
			f.expectWait("true")
			f.setDriverReady(false)
			f.setNodeReady(corev1.ConditionTrue)
			f.expectWait("true")
			Expect(f.shouldRequeue()).To(BeTrue())
			// Let Node/label/policy events settle before changing only Pod status.
			Consistently(func(g Gomega) {
				n := &corev1.Node{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(f.node), n)).To(Succeed())
				g.Expect(n.Labels[nodeinfo.NodeLabelWaitOFED]).To(Equal("true"))
			}, 2*time.Second, interval).Should(Succeed())
			f.setDriverReady(true)
			f.expectWait("false")
			Expect(f.shouldRequeue()).To(BeFalse())
			Expect(f.daemonSet(Default).Status).To(Equal(before.Status))
		},
		Entry("NicClusterPolicy", false),
		Entry("NicNodePolicy", true),
	)

	It("skips unscheduled pods and returns node read errors for retry", func() {
		waiting, err := processOFEDPodsForWaitLabels(ctx, k8sClient, &corev1.PodList{Items: []corev1.Pod{
			{Spec: corev1.PodSpec{NodeName: ""}},
		}})
		Expect(err).NotTo(HaveOccurred())
		Expect(waiting).To(BeFalse())
		waiting, err = processOFEDPodsForWaitLabels(ctx, k8sClient, &corev1.PodList{Items: []corev1.Pod{
			{Spec: corev1.PodSpec{NodeName: "missing-mofed-node"}},
		}})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(waiting).To(BeFalse())
	})
})
