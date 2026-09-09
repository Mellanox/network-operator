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
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	osconfigv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
)

var proxyKey = types.NamespacedName{Name: consts.OpenshiftClusterWideProxyName}

// setupProxyWatch wires the production Proxy watch into a dedicated controller and returns the
// channel of reconcile requests it produces. The envtest cluster is a vanilla Kubernetes one, so
// the real controllers skip the watch; a cluster type provider that reports Openshift is the only
// difference from production here.
func setupProxyWatch(name string, enqueue handler.EventHandler) chan reconcile.Request {
	requests := make(chan reconcile.Request, 32)

	bld := ctrl.NewControllerManagedBy(k8sManager).Named(name)
	bld = watchClusterWideProxy(bld, logr.Discard(), fakeClusterType{openshift: true},
		k8sManager.GetRESTMapper(), enqueue)

	err := bld.Complete(reconcile.Func(
		func(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
			// Never block the controller if a spec stops draining the channel.
			select {
			case requests <- req:
			default:
			}
			return reconcile.Result{}, nil
		}))
	Expect(err).NotTo(HaveOccurred())

	return requests
}

func getClusterProxy() *osconfigv1.Proxy {
	proxy := &osconfigv1.Proxy{}
	Expect(k8sClient.Get(ctx, proxyKey, proxy)).To(Succeed())
	return proxy
}

func createClusterProxy() {
	proxy := &osconfigv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyKey.Name},
		Spec:       osconfigv1.ProxySpec{HTTPProxy: "http://proxy.example.com:3128"},
	}
	Expect(k8sClient.Create(ctx, proxy)).To(Succeed())
}

// deleteClusterProxyOnCleanup removes the cluster-wide Proxy once the calling container finishes.
// It must be called from BeforeAll so that the specs of an Ordered container share one Proxy.
func deleteClusterProxyOnCleanup() {
	DeferCleanup(func() {
		err := k8sClient.Delete(ctx, &osconfigv1.Proxy{ObjectMeta: metav1.ObjectMeta{Name: proxyKey.Name}})
		if err != nil && !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})
}

var _ = Describe("Openshift cluster-wide Proxy watch for NicClusterPolicy", Ordered, func() {
	var requests chan reconcile.Request

	expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{
		Name: consts.NicClusterPolicyResourceName,
	}}

	BeforeAll(func() {
		requests = setupProxyWatch("openshift-proxy-watch-ncp-test", enqueueNicClusterPolicy())
		deleteClusterProxyOnCleanup()
	})

	It("should enqueue the NicClusterPolicy when the cluster-wide Proxy is created", func() {
		createClusterProxy()

		Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))
	})

	It("should enqueue the NicClusterPolicy when a proxy URL changes", func() {
		proxy := getClusterProxy()
		proxy.Spec.HTTPSProxy = "https://proxy.example.com:3129"
		proxy.Spec.NoProxy = "127.0.0.1,.cluster.local"
		Expect(k8sClient.Update(ctx, proxy)).To(Succeed())

		Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))
	})

	It("should enqueue the NicClusterPolicy when the trusted CA reference changes", func() {
		proxy := getClusterProxy()
		proxy.Spec.TrustedCA = osconfigv1.ConfigMapNameReference{Name: "user-ca-bundle"}
		Expect(k8sClient.Update(ctx, proxy)).To(Succeed())

		Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))
	})

	It("should not enqueue the NicClusterPolicy for changes that are not rendered", func() {
		proxy := getClusterProxy()
		proxy.Annotations = map[string]string{"example.com/unrelated": "true"}
		Expect(k8sClient.Update(ctx, proxy)).To(Succeed())

		Consistently(requests, "2s", interval).ShouldNot(Receive())
	})

	It("should enqueue the NicClusterPolicy when the cluster-wide Proxy is deleted", func() {
		Expect(k8sClient.Delete(ctx, getClusterProxy())).To(Succeed())

		Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))
	})
})

var _ = Describe("Openshift cluster-wide Proxy watch for NicNodePolicy", Ordered, func() {
	var requests chan reconcile.Request

	const (
		policyWithOFED    = "proxy-watch-nnp-with-ofed"
		policyWithoutOFED = "proxy-watch-nnp-without-ofed"
	)

	ofedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: policyWithOFED}}

	createPolicy := func(name string, spec mellanoxv1alpha1.NicNodePolicySpec) {
		policy := &mellanoxv1alpha1.NicNodePolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		DeferCleanup(func() {
			err := k8sClient.Delete(ctx, &mellanoxv1alpha1.NicNodePolicy{
				ObjectMeta: metav1.ObjectMeta{Name: name},
			})
			if err != nil && !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}

	BeforeAll(func() {
		// Distinct node selectors keep the policies out of each other's overlap detection.
		createPolicy(policyWithOFED, mellanoxv1alpha1.NicNodePolicySpec{
			NodeSelector: map[string]string{"proxy-watch-test": "ofed"},
			OFEDDriver: &mellanoxv1alpha1.OFEDDriverSpec{
				ImageSpec: mellanoxv1alpha1.ImageSpec{
					Image:            "mofed",
					Repository:       "acme.buzz",
					Version:          "5.9-0.5.6.0",
					ImagePullSecrets: []string{},
				},
			},
		})
		createPolicy(policyWithoutOFED, mellanoxv1alpha1.NicNodePolicySpec{
			NodeSelector: map[string]string{"proxy-watch-test": "no-ofed"},
			RdmaSharedDevicePlugin: &mellanoxv1alpha1.DevicePluginSpec{
				ImageSpecWithConfig: mellanoxv1alpha1.ImageSpecWithConfig{
					ImageSpec: mellanoxv1alpha1.ImageSpec{
						Image:            "k8s-rdma-shared-dev-plugin",
						Repository:       "acme.buzz",
						Version:          "v1.5.3",
						ImagePullSecrets: []string{},
					},
				},
			},
		})

		requests = setupProxyWatch("openshift-proxy-watch-nnp-test",
			enqueueNicNodePoliciesWithOFED(k8sManager.GetClient()))
		deleteClusterProxyOnCleanup()
	})

	It("should enqueue only OFED policies when the cluster-wide Proxy is created", func() {
		createClusterProxy()

		Eventually(requests, timeout, interval).Should(Receive(Equal(ofedRequest)))
		// The policy without an ofedDriver section renders nothing from the proxy.
		Consistently(requests, "2s", interval).ShouldNot(Receive())
	})

	It("should enqueue only OFED policies when a proxy URL changes", func() {
		proxy := getClusterProxy()
		proxy.Spec.HTTPSProxy = "https://proxy.example.com:3129"
		Expect(k8sClient.Update(ctx, proxy)).To(Succeed())

		Eventually(requests, timeout, interval).Should(Receive(Equal(ofedRequest)))
		Consistently(requests, "2s", interval).ShouldNot(Receive())
	})

	It("should enqueue only OFED policies when the trusted CA reference changes", func() {
		proxy := getClusterProxy()
		proxy.Spec.TrustedCA = osconfigv1.ConfigMapNameReference{Name: "user-ca-bundle"}
		Expect(k8sClient.Update(ctx, proxy)).To(Succeed())

		Eventually(requests, timeout, interval).Should(Receive(Equal(ofedRequest)))
		Consistently(requests, "2s", interval).ShouldNot(Receive())
	})

	It("should not enqueue any policy for changes that are not rendered", func() {
		proxy := getClusterProxy()
		proxy.Annotations = map[string]string{"example.com/unrelated": "true"}
		Expect(k8sClient.Update(ctx, proxy)).To(Succeed())

		Consistently(requests, "2s", interval).ShouldNot(Receive())
	})

	It("should enqueue only OFED policies when the cluster-wide Proxy is deleted", func() {
		Expect(k8sClient.Delete(ctx, getClusterProxy())).To(Succeed())

		Eventually(requests, timeout, interval).Should(Receive(Equal(ofedRequest)))
		Consistently(requests, "2s", interval).ShouldNot(Receive())
	})
})
