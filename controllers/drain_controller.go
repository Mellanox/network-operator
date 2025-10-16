/*
2025 NVIDIA CORPORATION & AFFILIATES

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
	"fmt"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	constants "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/drain"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/platforms"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/Mellanox/network-operator/pkg/consts"
	drainer "github.com/Mellanox/network-operator/pkg/drain"
)

const (
	// MaxConcurrentDrainReconciles is the maximum number of concurrent reconciles for the drain controller
	MaxConcurrentDrainReconciles = 50
)

// DrainReconcile is a struct that contains drain controller configurations
type DrainReconcile struct {
	client.Client
	scheme      *runtime.Scheme
	recorder    record.EventRecorder
	drainer     drain.DrainInterface
	migrationCh chan struct{}

	log logr.Logger
}

// NewDrainReconcileController creates a new DrainReconcile controller
func NewDrainReconcileController(client client.Client, k8sConfig *rest.Config, scheme *runtime.Scheme,
	recorder record.EventRecorder, platformHelper platforms.Interface, migrationCh chan struct{},
	log logr.Logger) (*DrainReconcile, error) {
	drainer, err := drainer.NewDrainRequestor(client, k8sConfig, log, platformHelper)
	if err != nil {
		return nil, err
	}

	return &DrainReconcile{
		client,
		scheme,
		recorder,
		drainer,
		migrationCh,
		log}, nil
}

//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworknodestates,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances,verbs=get;list;watch
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *DrainReconcile) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	select {
	case <-r.migrationCh:
	case <-ctx.Done():
		return ctrl.Result{}, fmt.Errorf("canceled")
	}
	reqLogger := r.log.WithName("Drain Reconcile")

	// get node object
	node := &corev1.Node{}
	found, err := r.getObject(ctx, req, node)
	if err != nil {
		reqLogger.Error(err, "failed to get node object")
		return ctrl.Result{}, err
	}
	if !found {
		reqLogger.Info("node not found, don't requeue the request", "node", req.Name)
		return ctrl.Result{}, nil
	}

	req.Namespace = drainer.GetDrainRequestorOpts(r.drainer).SriovNodeStateNamespace
	// get sriovNodeNodeState object
	nodeNetworkState := &sriovnetworkv1.SriovNetworkNodeState{}
	found, err = r.getObject(ctx, req, nodeNetworkState)
	if err != nil {
		reqLogger.Error(err, "failed to get sriovNetworkNodeState object")
		return ctrl.Result{}, err
	}
	if !found {
		reqLogger.Info("sriovNetworkNodeState not found, don't requeue the request")
		return ctrl.Result{}, nil
	}

	// create the drain state annotation if it doesn't exist in the sriovNetworkNodeState object
	nodeStateDrainAnnotationCurrent, currentNodeStateExist,
		err := r.ensureAnnotationExists(ctx, nodeNetworkState, constants.NodeStateDrainAnnotationCurrent)
	if err != nil {
		reqLogger.Error(err, "failed to ensure nodeStateDrainAnnotationCurrent")
		return ctrl.Result{}, err
	}
	_, desiredNodeStateExist, err := r.ensureAnnotationExists(ctx, nodeNetworkState,
		constants.NodeStateDrainAnnotation)
	if err != nil {
		reqLogger.Error(err, "failed to ensure nodeStateDrainAnnotation")
		return ctrl.Result{}, err
	}

	// create the drain state annotation if it doesn't exist in the node object
	nodeDrainAnnotation, nodeExist, err := r.ensureAnnotationExists(ctx, node, constants.NodeDrainAnnotation)
	if err != nil {
		reqLogger.Error(err, "failed to ensure nodeStateDrainAnnotation")
		return ctrl.Result{}, err
	}

	// requeue the request if we needed to add any of the annotations
	if !nodeExist || !currentNodeStateExist || !desiredNodeStateExist {
		return ctrl.Result{Requeue: true}, nil
	}
	reqLogger.V(consts.LogLevelInfo).Info("Drain annotations", "nodeAnnotation", nodeDrainAnnotation,
		"nodeStateAnnotation", nodeStateDrainAnnotationCurrent)

	// Check the node request
	if nodeDrainAnnotation == constants.DrainIdle {
		// this cover the case the node is on idle

		// node request to be on idle and the current state is idle
		// we don't do anything
		if nodeStateDrainAnnotationCurrent == constants.DrainIdle {
			reqLogger.Info("node and nodeState are on idle nothing todo")
			return reconcile.Result{}, nil
		}

		// we have two options here:
		// 1. node request idle and the current status is drain complete
		// this means the daemon finish is work, so we need to clean the drain
		//
		// 2. the operator is still draining the node but maybe the sriov policy changed and the daemon
		//  doesn't need to drain anymore, so we can stop the drain
		if nodeStateDrainAnnotationCurrent == constants.DrainComplete ||
			nodeStateDrainAnnotationCurrent == constants.Draining {
			return r.handleNodeIdleNodeStateDrainingOrCompleted(ctx, node, nodeNetworkState)
		}
	}

	// this cover the case a node request to drain or reboot
	if nodeDrainAnnotation == constants.DrainRequired ||
		nodeDrainAnnotation == constants.RebootRequired {
		return r.handleNodeDrainOrReboot(ctx, node, nodeNetworkState,
			nodeDrainAnnotation, nodeStateDrainAnnotationCurrent)
	}

	reqLogger.Error(nil, "unexpected node drain annotation", "nodeDrainAnnotation", nodeDrainAnnotation)
	return reconcile.Result{}, fmt.Errorf("unexpected node drain annotation")
}

func (r *DrainReconcile) getObject(ctx context.Context, req ctrl.Request, object client.Object) (bool, error) {
	err := r.Get(ctx, req.NamespacedName, object)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *DrainReconcile) ensureAnnotationExists(ctx context.Context,
	object client.Object, key string) (string, bool, error) {
	value, exist := object.GetAnnotations()[key]
	if !exist {
		err := utils.AnnotateObject(ctx, object, key, constants.DrainIdle, r.Client)
		if err != nil {
			return "", false, err
		}
		return constants.DrainIdle, false, nil
	}

	return value, true, nil
}

func (r *DrainReconcile) handleNodeIdleNodeStateDrainingOrCompleted(ctx context.Context,
	node *corev1.Node,
	nodeNetworkState *sriovnetworkv1.SriovNetworkNodeState) (ctrl.Result, error) {
	reqLogger := r.log.WithName("handleNodeIdleNodeStateDrainingOrCompleted")
	completed, err := r.drainer.CompleteDrainNode(ctx, node)
	if err != nil {
		reqLogger.Error(err, "failed to complete drain on node")
		r.recorder.Event(nodeNetworkState,
			corev1.EventTypeWarning,
			"DrainController",
			"failed to drain node")
		return ctrl.Result{}, err
	}

	// if we didn't manage to complete the un drain of the node we retry
	if !completed {
		reqLogger.Info("CompleteDrainNode() was not completed re queueing the request")
		return reconcile.Result{RequeueAfter: constants.DrainControllerRequeueTime}, nil
	}

	// check if nodeState annotation is already set to drain idle
	if utils.ObjectHasAnnotation(nodeNetworkState, constants.NodeStateDrainAnnotationCurrent, constants.DrainIdle) {
		reqLogger.Info("nodeState annotation is already set to drain idle, nothing to do")
		return ctrl.Result{}, nil
	}

	// move the node state back to idle
	err = utils.AnnotateObject(ctx, nodeNetworkState, constants.NodeStateDrainAnnotationCurrent,
		constants.DrainIdle, r.Client)
	if err != nil {
		reqLogger.Error(err, "failed to annotate node with annotation", "annotation", constants.DrainIdle)
		return ctrl.Result{}, err
	}

	reqLogger.Info("completed the un drain for node")
	r.recorder.Event(nodeNetworkState,
		corev1.EventTypeNormal,
		"DrainController",
		"node un drain completed")
	return ctrl.Result{}, nil
}

func (r *DrainReconcile) handleNodeDrainOrReboot(ctx context.Context,
	node *corev1.Node,
	nodeNetworkState *sriovnetworkv1.SriovNetworkNodeState,
	nodeDrainAnnotation,
	nodeStateDrainAnnotationCurrent string) (ctrl.Result, error) {
	reqLogger := r.log.WithName("handleNodeDrainOrReboot")
	// nothing to do here we need to wait for the node to move back to idle
	if nodeStateDrainAnnotationCurrent == constants.DrainComplete {
		reqLogger.Info("node requested a drain and nodeState is on drain completed nothing to do")
		return ctrl.Result{}, nil
	}

	// Check if we are on a single node, and we require a reboot/full-drain we just return
	fullNodeDrain := nodeDrainAnnotation == constants.RebootRequired
	singleNode := false
	if fullNodeDrain {
		nodeList := &corev1.NodeList{}
		err := r.Client.List(ctx, nodeList)
		if err != nil {
			reqLogger.Error(err, "failed to list nodes")
			return ctrl.Result{}, err
		}
		if len(nodeList.Items) == 1 {
			reqLogger.Info("drainNode(): FullNodeDrain requested and we are on Single node")
			singleNode = true
		}
	}

	// call the drain function that will also call drain to other platform providers like openshift
	drained, err := r.drainer.DrainNode(ctx, node, fullNodeDrain, singleNode)
	if err != nil {
		reqLogger.Error(err, "error trying to drain the node")
		r.recorder.Event(nodeNetworkState,
			corev1.EventTypeWarning,
			"DrainController",
			"failed to drain node")
		return reconcile.Result{}, err
	}

	// if we didn't manage to complete the drain of the node we retry
	if !drained {
		reqLogger.Info("the nodes was not drained, re queueing the request")
		return reconcile.Result{RequeueAfter: constants.DrainControllerRequeueTime}, nil
	}

	// if we manage to drain we label the node state with drain completed and finish
	err = utils.AnnotateObject(ctx, nodeNetworkState, constants.NodeStateDrainAnnotationCurrent,
		constants.DrainComplete, r.Client)
	if err != nil {
		reqLogger.Error(err, "failed to annotate node with annotation", "annotation", constants.DrainComplete)
		return ctrl.Result{}, err
	}

	reqLogger.Info("node drained successfully")
	r.recorder.Event(nodeNetworkState,
		corev1.EventTypeNormal,
		"DrainController",
		"node drain completed")
	return ctrl.Result{}, nil
}

// DrainAnnotationPredicate contains the predicate for node drain annotation changes
type DrainAnnotationPredicate struct {
	predicate.Funcs
	log logr.Logger
}

//nolint:dupl,revive
func (DrainAnnotationPredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}

	if _, hasAnno := e.Object.GetAnnotations()[constants.NodeDrainAnnotation]; hasAnno {
		return true
	}
	return false
}

//nolint:dupl,revive
func (d DrainAnnotationPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	oldAnno, hasOldAnno := e.ObjectOld.GetAnnotations()[constants.NodeDrainAnnotation]
	newAnno, hasNewAnno := e.ObjectNew.GetAnnotations()[constants.NodeDrainAnnotation]

	d.log.V(consts.LogLevelDebug).Info("Update", "oldAnno", oldAnno, "newAnno", newAnno)
	if !hasOldAnno && hasNewAnno {
		return true
	}

	return oldAnno != newAnno
}

// DrainStateAnnotationPredicate contains the predicate for
// sriov node state drain annotation changes
type DrainStateAnnotationPredicate struct {
	predicate.Funcs
	log logr.Logger
}

//nolint:revive
func (DrainStateAnnotationPredicate) Create(e event.CreateEvent) bool {
	return e.Object != nil
}

//nolint:revive
func (d DrainStateAnnotationPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	oldAnno, hasOldAnno := e.ObjectOld.GetAnnotations()[constants.NodeStateDrainAnnotationCurrent]
	newAnno, hasNewAnno := e.ObjectNew.GetAnnotations()[constants.NodeStateDrainAnnotationCurrent]

	d.log.V(consts.LogLevelDebug).Info("Update", "oldAnno", oldAnno, "newAnno", newAnno)
	if !hasOldAnno || !hasNewAnno {
		return true
	}

	return oldAnno != newAnno
}

// SetupWithManager sets up the controller with the Manager.
func (r *DrainReconcile) SetupWithManager(mgr ctrl.Manager) error {
	createUpdateEnqueue := handler.Funcs{
		CreateFunc: func(_ context.Context, e event.TypedCreateEvent[client.Object],
			w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			w.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: drainer.GetDrainRequestorOpts(r.drainer).MaintenanceOPRequestorNS,
				Name:      e.Object.GetName(),
			}})
		},
		UpdateFunc: func(_ context.Context, e event.TypedUpdateEvent[client.Object],
			w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			w.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: drainer.GetDrainRequestorOpts(r.drainer).MaintenanceOPRequestorNS,
				Name:      e.ObjectNew.GetName(),
			}})
		},
	}

	logger := mgr.GetLogger().WithValues("Function", "Drain")
	requestorOpts := drainer.GetRequestorOptsFromEnvs()
	// Watch for spec and annotation changes
	nodePredicates := builder.WithPredicates(DrainAnnotationPredicate{})
	nodeStatePredicates := builder.WithPredicates(DrainStateAnnotationPredicate{log: logger})
	// TODO: Make sure there is once logger instance to be used for all the predicates
	nodeMaintenancePredicates := drainer.NewConditionChangedPredicate(logger,
		requestorOpts.MaintenanceOPRequestorID)
	requestorIDPredicate := drainer.NewRequestorIDPredicate(logger,
		requestorOpts.MaintenanceOPRequestorID)
	m := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: MaxConcurrentDrainReconciles,
			LogConstructor: func(request *reconcile.Request) logr.Logger {
				//nolint:lll
				// Inspired by https://github.com/kubernetes-sigs/controller-runtime/blob/52b17917caa97ec546423867d9637f1787830f3e/pkg/builder/controller.go#L447
				if req, ok := any(request).(*reconcile.Request); ok && req != nil {
					logger = logger.WithValues("node", request.Name)
				}
				return logger
			},
		}).
		For(&corev1.Node{}, nodePredicates).
		Watches(&sriovnetworkv1.SriovNetworkNodeState{}, createUpdateEnqueue, nodeStatePredicates).
		Watches(&maintenancev1alpha1.NodeMaintenance{}, createUpdateEnqueue,
			builder.WithPredicates(nodeMaintenancePredicates, requestorIDPredicate))

	return m.Complete(r)
}
