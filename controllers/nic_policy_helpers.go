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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/state"
)

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
