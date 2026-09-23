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
	"errors"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	osconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/consts"
)

// fakeClusterType is a static clustertype.Provider.
type fakeClusterType struct {
	openshift bool
}

func (p fakeClusterType) GetClusterType() clustertype.Type {
	if p.openshift {
		return clustertype.Openshift
	}
	return clustertype.Kubernetes
}

func (p fakeClusterType) IsKubernetes() bool { return !p.openshift }
func (p fakeClusterType) IsOpenshift() bool  { return p.openshift }

// openshiftRESTMapper returns a RESTMapper that serves config.openshift.io/v1 Proxy.
func openshiftRESTMapper() meta.RESTMapper {
	return restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{{
		Group: metav1.APIGroup{
			Name:     osconfigv1.GroupName,
			Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "config.openshift.io/v1", Version: "v1"}},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1": {{Name: "proxies", Kind: "Proxy", Namespaced: false}},
		},
	}})
}

// kubernetesRESTMapper returns a RESTMapper for a cluster without the Openshift config API.
func kubernetesRESTMapper() meta.RESTMapper {
	return restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{{
		Group: metav1.APIGroup{
			Name:     "",
			Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "v1", Version: "v1"}},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1": {{Name: "nodes", Kind: "Node", Namespaced: false}},
		},
	}})
}

func clusterProxy(spec *osconfigv1.ProxySpec) *osconfigv1.Proxy {
	proxy := &osconfigv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: consts.OpenshiftClusterWideProxyName},
	}
	if spec != nil {
		proxy.Spec = *spec
	}
	return proxy
}

func TestClusterWideProxyServed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		clusterType clustertype.Provider
		mapper      meta.RESTMapper
		want        bool
	}{
		{
			name:        "openshift cluster serving the Proxy kind",
			clusterType: fakeClusterType{openshift: true},
			mapper:      openshiftRESTMapper(),
			want:        true,
		},
		{
			name:        "vanilla Kubernetes cluster",
			clusterType: fakeClusterType{openshift: false},
			mapper:      kubernetesRESTMapper(),
			want:        false,
		},
		{
			// Guards against a cluster that reports itself as Openshift while the
			// config.openshift.io API is unavailable.
			name:        "openshift cluster not serving the Proxy kind",
			clusterType: fakeClusterType{openshift: true},
			mapper:      kubernetesRESTMapper(),
			want:        false,
		},
		{
			name:        "no cluster type provider",
			clusterType: nil,
			mapper:      openshiftRESTMapper(),
			want:        false,
		},
		{
			name:        "no RESTMapper",
			clusterType: fakeClusterType{openshift: true},
			mapper:      nil,
			want:        false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := clusterWideProxyServed(tc.clusterType, tc.mapper); got != tc.want {
				t.Fatalf("clusterWideProxyServed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWatchClusterWideProxyIsSkippedWithoutTheProxyKind(t *testing.T) {
	t.Parallel()

	// A nil builder would panic if Watches were called, so reaching the end of the call
	// proves that no watch was registered for a kind the cluster does not serve.
	bld := watchClusterWideProxy(nil, logr.Discard(), fakeClusterType{openshift: false},
		kubernetesRESTMapper(), enqueueNicClusterPolicy())
	if bld != nil {
		t.Fatal("expected the builder to be returned untouched")
	}
}

func TestClusterWideProxyChangedPredicateCreateAndDelete(t *testing.T) {
	t.Parallel()

	otherProxy := &osconfigv1.Proxy{ObjectMeta: metav1.ObjectMeta{Name: "not-the-cluster-proxy"}}

	for _, tc := range []struct {
		name string
		obj  client.Object
		want bool
	}{
		{name: "cluster-wide Proxy", obj: clusterProxy(nil), want: true},
		{name: "Proxy with another name", obj: otherProxy, want: false},
		{name: "another kind", obj: &corev1.Node{}, want: false},
		{name: "typed nil Proxy", obj: (*osconfigv1.Proxy)(nil), want: false},
		{name: "nil object", obj: nil, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := ClusterWideProxyChangedPredicate{}
			if got := p.Create(event.CreateEvent{Object: tc.obj}); got != tc.want {
				t.Fatalf("Create() = %v, want %v", got, tc.want)
			}
			if got := p.Delete(event.DeleteEvent{Object: tc.obj}); got != tc.want {
				t.Fatalf("Delete() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClusterWideProxyChangedPredicateUpdate(t *testing.T) {
	t.Parallel()

	base := osconfigv1.ProxySpec{
		HTTPProxy:  "http://proxy:3128",
		HTTPSProxy: "https://proxy:3129",
		NoProxy:    "127.0.0.1",
		TrustedCA:  osconfigv1.ConfigMapNameReference{Name: "user-ca-bundle"},
	}
	withHTTPProxy := base
	withHTTPProxy.HTTPProxy = "http://other-proxy:3128"
	withHTTPSProxy := base
	withHTTPSProxy.HTTPSProxy = "https://other-proxy:3129"
	withNoProxy := base
	withNoProxy.NoProxy = "127.0.0.1,.cluster.local"
	withTrustedCA := base
	withTrustedCA.TrustedCA = osconfigv1.ConfigMapNameReference{Name: "other-ca-bundle"}
	withReadiness := base
	withReadiness.ReadinessEndpoints = []string{"http://example.com"}

	for _, tc := range []struct {
		name     string
		oldProxy client.Object
		newProxy client.Object
		want     bool
	}{
		{name: "httpProxy changed", oldProxy: clusterProxy(&base), newProxy: clusterProxy(&withHTTPProxy), want: true},
		{name: "httpsProxy changed", oldProxy: clusterProxy(&base), newProxy: clusterProxy(&withHTTPSProxy), want: true},
		{name: "noProxy changed", oldProxy: clusterProxy(&base), newProxy: clusterProxy(&withNoProxy), want: true},
		{name: "trustedCA changed", oldProxy: clusterProxy(&base), newProxy: clusterProxy(&withTrustedCA), want: true},
		{name: "nothing changed", oldProxy: clusterProxy(&base), newProxy: clusterProxy(&base), want: false},
		{
			// readinessEndpoints is not rendered into any workload.
			name:     "unrendered field changed",
			oldProxy: clusterProxy(&base),
			newProxy: clusterProxy(&withReadiness),
			want:     false,
		},
		{
			name:     "Proxy with another name",
			oldProxy: &osconfigv1.Proxy{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Spec: base},
			newProxy: &osconfigv1.Proxy{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Spec: withHTTPProxy},
			want:     false,
		},
		{name: "another kind", oldProxy: &corev1.Node{}, newProxy: &corev1.Node{}, want: false},
		{name: "missing old object", oldProxy: nil, newProxy: clusterProxy(&base), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := ClusterWideProxyChangedPredicate{}
			got := p.Update(event.UpdateEvent{ObjectOld: tc.oldProxy, ObjectNew: tc.newProxy})
			if got != tc.want {
				t.Fatalf("Update() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClusterWideProxyChangedPredicateUpdateIgnoresStatusOnlyChange(t *testing.T) {
	t.Parallel()

	spec := osconfigv1.ProxySpec{HTTPProxy: "http://proxy:3128"}
	oldProxy := clusterProxy(&spec)
	newProxy := clusterProxy(&spec)
	newProxy.Status = osconfigv1.ProxyStatus{HTTPProxy: "http://proxy:3128"}

	p := ClusterWideProxyChangedPredicate{}
	if p.Update(event.UpdateEvent{ObjectOld: oldProxy, ObjectNew: newProxy}) {
		t.Fatal("expected a status-only update to be filtered out")
	}
}

func TestEnqueueNicClusterPolicy(t *testing.T) {
	t.Parallel()

	want := reconcile.Request{NamespacedName: types.NamespacedName{
		Name: consts.NicClusterPolicyResourceName,
	}}
	proxy := clusterProxy(nil)

	for _, tc := range []struct {
		name string
		fire func(h handler.Funcs, q workqueue.TypedRateLimitingInterface[reconcile.Request])
	}{
		{
			name: "create",
			fire: func(h handler.Funcs, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Create(context.Background(), event.CreateEvent{Object: proxy}, q)
			},
		},
		{
			name: "update",
			fire: func(h handler.Funcs, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Update(context.Background(), event.UpdateEvent{ObjectOld: proxy, ObjectNew: proxy}, q)
			},
		},
		{
			name: "delete",
			fire: func(h handler.Funcs, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Delete(context.Background(), event.DeleteEvent{Object: proxy}, q)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := workqueue.NewTypedRateLimitingQueue(
				workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			defer q.ShutDown()

			tc.fire(enqueueNicClusterPolicy(), q)

			if q.Len() != 1 {
				t.Fatalf("expected 1 queued request, got %d", q.Len())
			}
			got, _ := q.Get()
			if got != want {
				t.Fatalf("queued %v, want %v", got, want)
			}
		})
	}
}

func nicNodePolicy(name string, withOFED bool) *mellanoxv1alpha1.NicNodePolicy {
	policy := &mellanoxv1alpha1.NicNodePolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if withOFED {
		policy.Spec.OFEDDriver = &mellanoxv1alpha1.OFEDDriverSpec{}
	}
	return policy
}

// drainRequestNames collects every request queued by an event handler.
func drainRequestNames(q workqueue.TypedRateLimitingInterface[reconcile.Request]) []string {
	names := make([]string, 0, q.Len())
	for q.Len() > 0 {
		req, _ := q.Get()
		names = append(names, req.Name)
	}
	sort.Strings(names)
	return names
}

func TestEnqueueNicNodePoliciesWithOFED(t *testing.T) {
	t.Parallel()

	testScheme := runtime.NewScheme()
	if err := mellanoxv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}

	for _, tc := range []struct {
		name     string
		policies []client.Object
		want     []string
	}{
		{
			name: "only policies configuring OFED are enqueued",
			policies: []client.Object{
				nicNodePolicy("with-ofed", true),
				nicNodePolicy("without-ofed", false),
				nicNodePolicy("also-with-ofed", true),
			},
			want: []string{"also-with-ofed", "with-ofed"},
		},
		{
			name:     "no policy configures OFED",
			policies: []client.Object{nicNodePolicy("without-ofed", false)},
			want:     []string{},
		},
		{
			name:     "no policies at all",
			policies: nil,
			want:     []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(tc.policies...).Build()
			q := workqueue.NewTypedRateLimitingQueue(
				workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			defer q.ShutDown()

			enqueueNicNodePoliciesWithOFED(c).Create(context.Background(),
				event.CreateEvent{Object: clusterProxy(nil)}, q)

			got := drainRequestNames(q)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("queued %v, want %v", got, tc.want)
			}
		})
	}
}

// togglableListClient returns a client whose List fails for as long as failing is set.
func togglableListClient(t *testing.T, failing *atomic.Bool, objs ...client.Object) client.Client {
	t.Helper()

	testScheme := runtime.NewScheme()
	if err := mellanoxv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}

	return fake.NewClientBuilder().WithScheme(testScheme).WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList,
				opts ...client.ListOption) error {
				if failing.Load() {
					return errors.New("api server unavailable")
				}
				return c.List(ctx, list, opts...)
			},
		}).Build()
}

// testEnqueuer builds the production enqueuer with a retry interval short enough for tests.
func testEnqueuer(c client.Reader) *nicNodePolicyProxyEnqueuer {
	return &nicNodePolicyProxyEnqueuer{reader: c, retryInterval: 5 * time.Millisecond}
}

// eventuallyQueued waits for the queue to hold the wanted request names.
func eventuallyQueued(t *testing.T,
	q workqueue.TypedRateLimitingInterface[reconcile.Request], want []string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if q.Len() >= len(want) {
			if got := drainRequestNames(q); reflect.DeepEqual(got, want) {
				return
			}
			t.Fatalf("queued unexpected requests, want %v", want)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %v to be queued, queue holds %d requests", want, q.Len())
}

func TestEnqueueNicNodePoliciesWithOFEDDoesNotBlockOnListFailure(t *testing.T) {
	t.Parallel()

	// The handler runs on the watch's event delivery path, so a failing List must be handed to
	// the background retry instead of being waited on.
	failing := &atomic.Bool{}
	failing.Store(true)
	c := togglableListClient(t, failing, nicNodePolicy("with-ofed", true))
	q := workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	enqueuer := &nicNodePolicyProxyEnqueuer{reader: c, retryInterval: time.Hour}
	start := time.Now()
	enqueuer.enqueue(context.Background(), q)

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("handler blocked for %s, want it to return immediately", elapsed)
	}
	if q.Len() != 0 {
		t.Fatalf("expected nothing to be queued yet, got %d requests", q.Len())
	}
}

func TestEnqueueNicNodePoliciesWithOFEDRecoversAfterListFailure(t *testing.T) {
	t.Parallel()

	// A cluster-wide Proxy event cannot be replayed, so the policies must still be enqueued
	// once listing recovers, however long that takes.
	failing := &atomic.Bool{}
	failing.Store(true)
	c := togglableListClient(t, failing,
		nicNodePolicy("with-ofed", true), nicNodePolicy("without-ofed", false))
	q := workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testEnqueuer(c).enqueue(ctx, q)
	if q.Len() != 0 {
		t.Fatalf("expected nothing to be queued while listing fails, got %d requests", q.Len())
	}

	failing.Store(false)
	eventuallyQueued(t, q, []string{"with-ofed"})
}

func TestEnqueueNicNodePoliciesWithOFEDCoalescesBackgroundRetries(t *testing.T) {
	t.Parallel()

	// Repeated Proxy events while listing is broken must not pile up goroutines. The single
	// pending retry lists whatever exists when it runs, so nothing is lost by coalescing.
	failing := &atomic.Bool{}
	failing.Store(true)
	c := togglableListClient(t, failing, nicNodePolicy("with-ofed", true))
	q := workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	enqueuer := testEnqueuer(c)
	for range 5 {
		enqueuer.enqueue(ctx, q)
	}

	failing.Store(false)
	eventuallyQueued(t, q, []string{"with-ofed"})
}

func TestEnqueueNicNodePoliciesWithOFEDStopsWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	// On shutdown the background retry must stop instead of retrying forever.
	failing := &atomic.Bool{}
	failing.Store(true)
	c := togglableListClient(t, failing, nicNodePolicy("with-ofed", true))
	q := workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	enqueuer := testEnqueuer(c)
	enqueuer.enqueue(ctx, q)

	// Once the retry goroutine has observed the canceled context it releases its slot, and
	// nothing is ever queued even though listing recovers.
	failing.Store(false)
	deadline := time.Now().Add(5 * time.Second)
	for enqueuer.retrying.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if enqueuer.retrying.Load() {
		t.Fatal("background retry did not stop after the context was canceled")
	}
	if q.Len() != 0 {
		t.Fatalf("expected nothing to be queued, got %d requests", q.Len())
	}
}
