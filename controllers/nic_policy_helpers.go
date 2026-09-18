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
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	osconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/state"
)

// nicNodePolicyListRetryInterval is how often a cluster-wide Proxy event that could not be
// mapped to NicNodePolicies retries in the background.
const nicNodePolicyListRetryInterval = 2 * time.Second

// requeueWithDelay returns a reconcile result with the configured requeue delay.
func requeueWithDelay() (reconcile.Result, error) {
	return reconcile.Result{
		RequeueAfter: time.Duration(config.FromEnv().Controller.RequeueTimeSeconds) * time.Second,
	}, nil
}

// updatePolicyCRStatus upserts AppliedStates from sync results and updates CR status via API.
func updatePolicyCRStatus(ctx context.Context, statusClient client.StatusClient,
	cr mellanoxv1alpha1.NicPolicyCR, status state.Results) {
	reqLogger := log.FromContext(ctx)
	appliedStates := cr.GetAppliedStates()

NextResult:
	for _, stateStatus := range status.StatesStatus {
		for i := range appliedStates {
			if appliedStates[i].Name == stateStatus.StateName {
				appliedStates[i].State = mellanoxv1alpha1.State(stateStatus.Status)
				if stateStatus.ErrInfo != nil {
					appliedStates[i].Message = stateStatus.ErrInfo.Error()
				} else {
					appliedStates[i].Message = ""
				}
				continue NextResult
			}
		}
		appliedStates = append(appliedStates, mellanoxv1alpha1.AppliedState{
			Name:  stateStatus.StateName,
			State: mellanoxv1alpha1.State(stateStatus.Status),
		})
	}
	cr.SetAppliedStates(appliedStates)
	cr.SetPolicyState(mellanoxv1alpha1.State(status.Status))

	if ch, ok := cr.(mellanoxv1alpha1.ConditionHolder); ok {
		ch.SetConditions(computePolicyConditions(ch.GetConditions(), status, cr.GetGeneration()))
	}

	reqLogger.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.GetName(), "namespace", cr.GetNamespace(),
		"Result:", cr.GetPolicyState())
	if err := statusClient.Status().Update(ctx, cr); err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to update CR status")
	}
}

// setupOFEDCatalog adds NodeInfo and DocaDriverImage providers to the catalog.
// If nodeSelector is non-nil, only nodes matching both Mellanox NIC labels and
// the selector are included. Pass nil for cluster-wide (NCP) behavior.
func setupOFEDCatalog(ctx context.Context, c client.Client,
	spec *mellanoxv1alpha1.OFEDDriverSpec, docaProvider docadriverimages.Provider,
	catalog state.InfoCatalog, nodeSelector map[string]string) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Creating Node info provider")

	listOpts := append([]client.ListOption{}, nodeinfo.MellanoxNICListOptions...)
	if len(nodeSelector) > 0 {
		listOpts = append(listOpts, client.MatchingLabels(nodeSelector))
	}

	nodeList := &corev1.NodeList{}
	if err := c.List(ctx, nodeList, listOpts...); err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Error occurred on LIST nodes request from API server.")
		return err
	}

	nodePtrList := make([]*corev1.Node, len(nodeList.Items))
	nodeNames := make([]*string, len(nodeList.Items))
	for i := range nodePtrList {
		nodePtrList[i] = &nodeList.Items[i]
		nodeNames[i] = &nodeList.Items[i].Name
	}
	reqLogger.V(consts.LogLevelDebug).Info("Node info provider with", "Nodes:", nodeNames)

	catalog.Add(state.InfoTypeNodeInfo, nodeinfo.NewProvider(nodePtrList))
	docaProvider.SetImageSpec(&spec.ImageSpec)
	catalog.Add(state.InfoTypeDocaDriverImage, docaProvider)
	return nil
}

// enqueueNicClusterPolicy enqueues the single supported NicClusterPolicy on create, update and
// delete events. Watched objects that are not owned by the policy have no owner reference to map
// from, so the static name is used instead.
func enqueueNicClusterPolicy() handler.Funcs {
	add := func(q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name: consts.NicClusterPolicyResourceName,
		}})
	}
	return handler.Funcs{
		CreateFunc: func(_ context.Context, _ event.CreateEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			add(q)
		},
		UpdateFunc: func(_ context.Context, _ event.UpdateEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			add(q)
		},
		DeleteFunc: func(_ context.Context, _ event.DeleteEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			add(q)
		},
	}
}

// clusterWideProxyServed reports whether the cluster serves the Openshift Proxy kind.
// Both the cached cluster type and the RESTMapper must agree: on vanilla Kubernetes
// config.openshift.io/v1 is not registered, and starting an informer for a kind the API server
// does not serve makes the cache fail to sync, which prevents the manager from starting.
func clusterWideProxyServed(clusterType clustertype.Provider, mapper meta.RESTMapper) bool {
	if clusterType == nil || !clusterType.IsOpenshift() {
		return false
	}
	if mapper == nil {
		return false
	}
	gvk := osconfigv1.GroupVersion.WithKind("Proxy")
	_, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	return err == nil
}

// nicNodePolicyOFEDRequests returns a reconcile request per policy that configures OFED. Policies
// without an ofedDriver section render nothing from the cluster-wide proxy, so they are skipped.
func nicNodePolicyOFEDRequests(policyList *mellanoxv1alpha1.NicNodePolicyList) []reconcile.Request {
	requests := make([]reconcile.Request, 0, len(policyList.Items))
	for i := range policyList.Items {
		if policyList.Items[i].Spec.OFEDDriver == nil {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&policyList.Items[i]),
		})
	}
	return requests
}

// nicNodePolicyProxyEnqueuer maps cluster-wide Proxy events onto the NicNodePolicies that render
// proxy settings. Unlike NicClusterPolicy there may be many of them, and a Proxy event carries no
// owner information, so the set has to be listed.
//
// A failed listing must not drop the event. controller-runtime cannot retry a mapping, and
// policies that already reached Ready are never requeued on their own, so the event is the only
// thing that will refresh their proxy env vars and trusted CA. Listing is therefore retried in
// the background until it succeeds or the manager stops, instead of giving up after a deadline.
// That also keeps event delivery for this watch unblocked.
type nicNodePolicyProxyEnqueuer struct {
	reader        client.Reader
	retryInterval time.Duration
	// retrying keeps at most one background retry in flight. A pending retry lists the policies
	// that exist when it eventually runs, so coalescing further Proxy events loses nothing.
	retrying atomic.Bool
}

// enqueue queues the affected policies, handing off to a background retry if they cannot be
// listed right now.
func (e *nicNodePolicyProxyEnqueuer) enqueue(
	ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	requests, err := e.requests(ctx)
	if err == nil {
		addRequests(q, requests)
		return
	}

	log.FromContext(ctx).V(consts.LogLevelWarning).Info(
		"failed to list NicNodePolicies for a cluster-wide proxy change, retrying in the background",
		"error", err.Error(), "interval", e.retryInterval)
	e.retryInBackground(ctx, q)
}

// retryInBackground retries the listing until it succeeds or ctx is canceled, which happens when
// the manager stops. It is a no-op if a retry is already in flight.
func (e *nicNodePolicyProxyEnqueuer) retryInBackground(
	ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if !e.retrying.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer e.retrying.Store(false)

		// Adding to a queue that has already been shut down is a no-op, so losing the race
		// with shutdown is harmless.
		_ = wait.PollUntilContextCancel(ctx, e.retryInterval, false,
			func(innerCtx context.Context) (bool, error) {
				requests, err := e.requests(innerCtx)
				if err != nil {
					log.FromContext(innerCtx).V(consts.LogLevelDebug).Info(
						"still cannot list NicNodePolicies for a cluster-wide proxy change",
						"error", err.Error())
					return false, nil
				}
				addRequests(q, requests)
				log.FromContext(innerCtx).V(consts.LogLevelInfo).Info(
					"listed NicNodePolicies for a cluster-wide proxy change after retrying",
					"policies", len(requests))
				return true, nil
			})
	}()
}

func (e *nicNodePolicyProxyEnqueuer) requests(ctx context.Context) ([]reconcile.Request, error) {
	policyList := &mellanoxv1alpha1.NicNodePolicyList{}
	if err := e.reader.List(ctx, policyList); err != nil {
		return nil, err
	}
	return nicNodePolicyOFEDRequests(policyList), nil
}

func addRequests(q workqueue.TypedRateLimitingInterface[reconcile.Request], requests []reconcile.Request) {
	for _, req := range requests {
		q.Add(req)
	}
}

// enqueueNicNodePoliciesWithOFED returns a handler that enqueues every NicNodePolicy configuring
// OFED whenever the cluster-wide Proxy changes.
func enqueueNicNodePoliciesWithOFED(reader client.Reader) handler.EventHandler {
	enqueuer := &nicNodePolicyProxyEnqueuer{reader: reader, retryInterval: nicNodePolicyListRetryInterval}
	return handler.Funcs{
		CreateFunc: func(ctx context.Context, _ event.CreateEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueuer.enqueue(ctx, q)
		},
		UpdateFunc: func(ctx context.Context, _ event.UpdateEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueuer.enqueue(ctx, q)
		},
		DeleteFunc: func(ctx context.Context, _ event.DeleteEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueuer.enqueue(ctx, q)
		},
	}
}

// watchClusterWideProxy watches the Openshift cluster-wide Proxy object so that proxy and trusted
// CA changes are re-rendered into the workloads that embed them. Without it a policy that already
// reached Ready is not reconciled again, and its rendered configuration stays stale until an
// unrelated event happens to trigger a reconcile.
// The watch is skipped on clusters that do not serve the Proxy kind.
func watchClusterWideProxy(bld *builder.Builder, setupLog logr.Logger, clusterType clustertype.Provider,
	mapper meta.RESTMapper, enqueue handler.EventHandler) *builder.Builder {
	if !clusterWideProxyServed(clusterType, mapper) {
		setupLog.V(consts.LogLevelInfo).Info(
			"Openshift cluster-wide Proxy is not served by this cluster, skipping watch")
		return bld
	}
	setupLog.V(consts.LogLevelInfo).Info("Watching", "Kind", "Proxy")
	return bld.Watches(
		&osconfigv1.Proxy{},
		enqueue,
		builder.WithPredicates(ClusterWideProxyChangedPredicate{}),
	)
}

// watchStateSources adds Watches for all state manager source kinds with
// EnqueueRequestForOwner and IgnoreSameContentPredicate.
func watchStateSources(bld *builder.Builder, mgr ctrl.Manager, setupLog logr.Logger,
	stateManager state.Manager, ownerType client.Object) *builder.Builder {
	ws := stateManager.GetWatchSources()
	for kindName := range ws {
		setupLog.V(consts.LogLevelInfo).Info("Watching", "Kind", kindName)
		bld = bld.Watches(ws[kindName], handler.EnqueueRequestForOwner(
			mgr.GetScheme(), mgr.GetRESTMapper(), ownerType, handler.OnlyControllerOwner()),
			builder.WithPredicates(IgnoreSameContentPredicate{}))
	}
	return bld
}
