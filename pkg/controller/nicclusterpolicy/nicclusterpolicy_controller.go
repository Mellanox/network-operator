package nicclusterpolicy

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/mellanox-network-operator/pkg/apis/mellanox/v1alpha1"
	"github.com/Mellanox/mellanox-network-operator/pkg/consts"
	"github.com/Mellanox/mellanox-network-operator/pkg/nodeinfo"
	"github.com/Mellanox/mellanox-network-operator/pkg/state"
)

var log = logf.Log.WithName("controller_nicclusterpolicy")

// Add creates a new NicClusterPolicy Controller and adds it to the Manager.
// The Manager will set fields on the Controller and Start it when the Manager is started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	// Create state manager
	stateManager, err := state.NewManager(mellanoxv1alpha1.NicClusterPolicyCRDName, mgr.GetClient(), mgr.GetScheme())
	if err != nil {
		// Error creating stateManager
		log.V(consts.LogLevelError).Info("Error creating state manager.", "error:", err)
		panic("Failed to create State manager")
	}
	return &ReconcileNicClusterPolicy{client: mgr.GetClient(), scheme: mgr.GetScheme(), stateManager: stateManager}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nicclusterpolicy-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource NicClusterPolicy
	err = c.Watch(&source.Kind{Type: &mellanoxv1alpha1.NicClusterPolicy{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource DaemonSet and requeue the owner NicClusterPolicy
	ws := r.(*ReconcileNicClusterPolicy).stateManager.GetWatchSources()
	log.V(consts.LogLevelInfo).Info("Watch Sources", "Kind:", ws)
	for i := range ws {
		err = c.Watch(ws[i], &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &mellanoxv1alpha1.NicClusterPolicy{},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// blank assignment to verify that ReconcileNicClusterPolicy implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNicClusterPolicy{}

// ReconcileNicClusterPolicy reconciles a NicClusterPolicy object
type ReconcileNicClusterPolicy struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	stateManager state.Manager
}

// Reconcile reads that state of the cluster for a NicClusterPolicy object and makes changes based on the state read
// and what is in the NicClusterPolicy.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Results.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNicClusterPolicy) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.V(consts.LogLevelInfo).Info("Reconciling NicClusterPolicy")

	// Fetch the NicClusterPolicy instance
	instance := &mellanoxv1alpha1.NicClusterPolicy{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.V(consts.LogLevelError).Info("Error occurred on GET CRD request from API server.", "error:", err)
		return reconcile.Result{}, err
	}

	// Create a new State service catalog
	sc := state.NewServiceCatalog()
	if instance.Spec.OFEDDriver != nil || instance.Spec.NVPeerDriver != nil {
		// Create node infoProvider and add to the service catalog
		reqLogger.V(consts.LogLevelInfo).Info("Creating Node info provider")
		nodeList := &corev1.NodeList{}
		err = r.client.List(context.TODO(), nodeList, nodeinfo.MellanoxNICListOptions...)
		if err != nil {
			// Failed to get node list
			reqLogger.V(consts.LogLevelError).Info("Error occurred on LIST nodes request from API server.", "error:", err)
			return reconcile.Result{}, err
		}
		nodePtrList := make([]*corev1.Node, len(nodeList.Items))
		nodeNames := make([]*string, len(nodeList.Items))
		for i := range nodePtrList {
			nodePtrList[i] = &nodeList.Items[i]
			nodeNames[i] = &nodeList.Items[i].Name
		}
		reqLogger.V(consts.LogLevelDebug).Info("Node info provider with", "Nodes:", nodeNames)
		infoProvider := nodeinfo.NewProvider(nodePtrList)
		sc.Add(state.ServiceTypeNodeInfo, infoProvider)
	}
	// Create manager
	managerStatus, err := r.stateManager.SyncState(instance, sc)
	r.updateCrStatus(instance, managerStatus)

	if err != nil {
		return reconcile.Result{}, err
	}
	if managerStatus.Status != state.SyncStateReady {
		return reconcile.Result{
			RequeueAfter: 5 * time.Second,
		}, nil
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileNicClusterPolicy) updateCrStatus(cr *mellanoxv1alpha1.NicClusterPolicy, status state.Results) {
NextResult:
	for _, stateStatus := range status.StatesStatus {
		// basically iterate over results and add/update crStatus.AppliedStates
		for i := range cr.Status.AppliedStates {
			if cr.Status.AppliedStates[i].Name == stateStatus.StateName {
				cr.Status.AppliedStates[i].State = mellanoxv1alpha1.State(stateStatus.Status)
				continue NextResult
			}
		}
		cr.Status.AppliedStates = append(cr.Status.AppliedStates, mellanoxv1alpha1.AppliedState{
			Name:  stateStatus.StateName,
			State: mellanoxv1alpha1.State(stateStatus.Status),
		})
	}
	// Update global State
	cr.Status.State = mellanoxv1alpha1.State(status.Status)

	// send status update request to k8s API
	log.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.Name, "namespace", cr.Namespace, "Result:", cr.Status)
	err := r.client.Status().Update(context.TODO(), cr)
	if err != nil {
		log.V(consts.LogLevelError).Info("Failed to update CR status", "error:", err)
	}
}
