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

// Package drain provides functionality for managing node drain operations in Kubernetes clusters,
// utilizing the NVIDIA's maintenance operator.
package drain

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"time"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/drain"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/orchestrator"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/Mellanox/network-operator/pkg/consts"
)

// RequestorOptions is a struct that contains the options for the drain controller requestor
// utilizing maintenance operator
type RequestorOptions struct {
	// UseMaintenanceOperator enables requestor upgrade mode
	UseMaintenanceOperator bool
	// MaintenanceOPRequestorID is the requestor ID for maintenance operator
	MaintenanceOPRequestorID string
	// MaintenanceOPRequestorNS is a user defined namespace which nodeMaintenance
	// objects will be created
	MaintenanceOPRequestorNS string
	// NodeMaintenanceNamePrefix is a prefix for nodeMaintenance object name
	// e.g. <prefix>-<node-name> to distinguish between different requestors if desired
	NodeMaintenanceNamePrefix string
	// MaintenanceOPPodEvictionFilter is a filter to be used for pods eviction
	// by maintenance operator
	MaintenanceOPPodEvictionFilter []maintenancev1alpha1.PodEvictionFiterEntry
	// SriovNodeStateNamespace is a namespace where sriov-network-operator is deployed
	SriovNodeStateNamespace string
}

// DrainRequestor is a struct that contains configurations for
// the drain controller requestor
//
//nolint:revive
type DrainRequestor struct {
	opts                   RequestorOptions
	k8sClient              client.Client
	kubeClient             kubernetes.Interface
	orchestrator           orchestrator.Interface
	log                    logr.Logger
	defaultNodeMaintenance *maintenancev1alpha1.NodeMaintenance
}

const (
	// DefaultNodeMaintenanceNamePrefix is a default prefix for nodeMaintenance object name
	DefaultNodeMaintenanceNamePrefix = "nvidia-operator" // e.g."sriov-operator-drainer"
	// trueString is the word true as string to avoid duplication and linting errors
	trueString = "true"
	// DrainTimeOut is the default timeout for the drain operation
	DrainTimeOut = 90 * time.Second
)

// ConditionChangedPredicate contains the predicate for the condition changed
// event for the node maintenance object
type ConditionChangedPredicate struct {
	predicate.Funcs
	requestorID string

	log logr.Logger
}

// NewConditionChangedPredicate creates a new ConditionChangedPredicate
func NewConditionChangedPredicate(log logr.Logger, requestorID string) ConditionChangedPredicate {
	return ConditionChangedPredicate{
		Funcs:       predicate.Funcs{},
		log:         log,
		requestorID: requestorID,
	}
}

// Update implements Predicate.
func (p ConditionChangedPredicate) Update(e event.TypedUpdateEvent[client.Object]) bool {
	p.log.V(consts.LogLevelDebug).Info("ConditionChangedPredicate Update event")

	if e.ObjectOld == nil {
		p.log.Info("old object is nil in update event, ignoring event.")
		return false
	}
	if e.ObjectNew == nil {
		p.log.Info("new object is nil in update event, ignoring event.")
		return false
	}

	oldO, ok := e.ObjectOld.(*maintenancev1alpha1.NodeMaintenance)
	if !ok {
		err := fmt.Errorf("expected NodeMaintenance, got %T", oldO)
		p.log.Error(err, "failed to cast old object to NodeMaintenance in update event, ignoring event.")
		return false
	}

	newO, ok := e.ObjectNew.(*maintenancev1alpha1.NodeMaintenance)
	if !ok {
		err := fmt.Errorf("expected NodeMaintenance, got %T", newO)
		p.log.Error(err, "failed to cast new object to NodeMaintenance in update event, ignoring event.")
		return false
	}

	cmpByType := func(a, b metav1.Condition) int {
		return cmp.Compare(a.Type, b.Type)
	}

	// sort old and new obj.Status.Conditions so they can be compared using DeepEqual
	slices.SortFunc(oldO.Status.Conditions, cmpByType)
	slices.SortFunc(newO.Status.Conditions, cmpByType)

	condChanged := !reflect.DeepEqual(oldO.Status.Conditions, newO.Status.Conditions)
	// Check if the object is marked for deletion
	deleting := len(newO.Finalizers) == 0 && len(oldO.Finalizers) > 0
	deleting = deleting && !newO.DeletionTimestamp.IsZero()
	enqueue := condChanged || deleting

	p.log.V(consts.LogLevelDebug).Info("update event for NodeMaintenance",
		"name", newO.Name, "namespace", newO.Namespace,
		"condition-changed", condChanged,
		"deleting", deleting, "enqueue-request", enqueue)

	return enqueue
}

// NewRequestorIDPredicate creates a new predicate that checks if nodeMaintenance object is
// related to current requestorID, whether owned or shared with current requestorID
func NewRequestorIDPredicate(log logr.Logger, requestorID string) predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		nm, ok := object.(*maintenancev1alpha1.NodeMaintenance)
		if !ok {
			log.Error(nil, "failed to cast object to NodeMaintenance in update event, ignoring event.")
			return false
		}
		// check if requestorID is the owner of the object or if is under AdditionalRequestors list
		return requestorID == nm.Spec.RequestorID || slices.Contains(nm.Spec.AdditionalRequestors, requestorID)
	})
}

func setDefaultNodeMaintenance(opts *RequestorOptions) *maintenancev1alpha1.NodeMaintenance {
	drainSpec := &maintenancev1alpha1.DrainSpec{
		Force: true,
		// TODO: Add pod selector
		PodSelector:    "nvidia.com/gpu-driver-upgrade-drain.skip!=true,nvidia.com/ofed-driver-upgrade-drain.skip!=true",
		TimeoutSecond:  int32(DrainTimeOut.Seconds()),
		DeleteEmptyDir: true,
	}
	return &maintenancev1alpha1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.MaintenanceOPRequestorNS,
		},
		Spec: maintenancev1alpha1.NodeMaintenanceSpec{
			RequestorID: opts.MaintenanceOPRequestorID,
			// TODO: Add wait for pod completion
			WaitForPodCompletion: nil,
			DrainSpec:            drainSpec,
		},
	}
}

// NewDrainRequestor creates a new DrainRequestor
func NewDrainRequestor(k8sClient client.Client, k8sConfig *rest.Config, log logr.Logger,
	orchestrator orchestrator.Interface) (*DrainRequestor, error) {
	kclient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}
	opts := GetRequestorOptsFromEnvs()

	return &DrainRequestor{
		opts:                   opts,
		k8sClient:              k8sClient,
		kubeClient:             kclient,
		orchestrator:           orchestrator,
		log:                    log,
		defaultNodeMaintenance: setDefaultNodeMaintenance(&opts),
	}, nil
}

// DrainNode the function cordon a node and drain pods from it
// if fullNodeDrain true all the pods on the system will get drained
// for openshift system we also pause the machine config pool this machine is part of it
func (d *DrainRequestor) DrainNode(ctx context.Context, node *corev1.Node,
	fullNodeDrain, singleNode bool) (bool, error) {
	log := d.log.WithName("drainNode")
	log.Info("Node drain requested")

	completed, err := d.orchestrator.BeforeDrainNode(ctx, node)
	if err != nil {
		log.Error(err, "error running OpenshiftDrainNode")
		return false, err
	}

	if !completed {
		log.Info("OpenshiftDrainNode did not finish re queue the node request")
		return false, nil
	}

	// Check if we are on a single node, and we require a reboot/full-drain we just return
	if fullNodeDrain && singleNode {
		return true, nil
	}

	// create node maintenance object
	nm, err := d.createOrUpdateNodeMaintenance(ctx, node.Name)
	if err != nil {
		log.Error(err, "error creating node maintenance")
		return false, err
	}
	cond := meta.FindStatusCondition(nm.Status.Conditions, maintenancev1alpha1.ConditionReasonReady)
	if cond != nil {
		if cond.Reason == maintenancev1alpha1.ConditionReasonReady {
			log.V(consts.LogLevelDebug).Info("node maintenance operation completed", nm.Spec.NodeName, cond.Reason)
			log.Info("drainNode(): Drain completed")
			return true, nil
		}
	}

	return false, nil
}

// CompleteDrainNode run un-cordon for the requested node
// for openshift system we also remove the pause from the machine config pool this node is part of
// only if we are the last draining node on that pool
func (d *DrainRequestor) CompleteDrainNode(ctx context.Context, node *corev1.Node) (bool, error) {
	log := d.log.WithName("CompleteDrainNode")
	d.log.WithName("CompleteDrainNode")

	// run the un cordon function on the node, by deleting node maintenance object
	// once node maintenance object is actually deleted by maintenance operator,
	// the node will be uncordoned.
	deleted, err := d.deleteOrUpdateNodeMaintenance(ctx, node.Name)
	if err != nil {
		d.log.Error(
			err, "failed to delete NodeMaintenance, node uncordon failed", "nodeMaintenance",
			d.getNodeMaintenanceName(node.Name))
		return false, err
	}

	// call the openshift complete drain to unpause the MCP
	// only if we are the last draining node in the pool
	completed, err := d.orchestrator.AfterCompleteDrainNode(ctx, node)
	if err != nil {
		log.Error(err, "failed to complete openshift draining")
		return false, err
	}
	log.V(consts.LogLevelDebug).Info("CompleteDrainNode:()", " nodeMaintenance deleted",
		deleted, "drainCompleted", completed)
	return completed && deleted, nil
}

//nolint:unused,unparam
func (d *DrainRequestor) createOrUpdateNodeMaintenance(ctx context.Context,
	nodeName string) (*maintenancev1alpha1.NodeMaintenance, error) {
	nm, err := d.checkForExistingNodeMaintenance(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	// check for existing nodeMaintenance obj
	if nm != nil {
		// if default prefix is used shared-requestor mode can be used
		if d.opts.NodeMaintenanceNamePrefix == DefaultNodeMaintenanceNamePrefix {
			// if exists append requestorID to spec.AdditionalRequestors list
			// check if object is owned by the requestor, if so skip re-creation
			if nm.Spec.RequestorID == d.opts.MaintenanceOPRequestorID {
				d.log.V(consts.LogLevelInfo).Info("nodeMaintenance already exists", nm.Name, "skip creation",
					"requestorID", d.opts.MaintenanceOPRequestorID)
				return nm, nil
			}

			// check if requestor is already in AdditionalRequestors
			if slices.Contains(nm.Spec.AdditionalRequestors, d.opts.MaintenanceOPRequestorID) {
				d.log.V(consts.LogLevelInfo).Info("requestor already in AdditionalRequestors list",
					"requestorID", d.opts.MaintenanceOPRequestorID)
				return nm, nil
			}

			d.log.V(consts.LogLevelInfo).Info("appending new requestor under AdditionalRequestors", "requestor",
				d.opts.MaintenanceOPRequestorID, "nodeMaintenance", client.ObjectKeyFromObject(nm))
			// create a deep copy of the original object before modifying it
			originalNm := nm.DeepCopy()
			// update AdditionalRequestor list
			nm.Spec.AdditionalRequestors = append(nm.Spec.AdditionalRequestors, d.opts.MaintenanceOPRequestorID)
			// using optimistic lock and patch command to avoid updating entire object and refraining of
			// additionalRequestors list overwritten by other operators
			patch := client.MergeFromWithOptions(originalNm, client.MergeFromWithOptimisticLock{})
			err := d.k8sClient.Patch(ctx, nm, patch)
			if err != nil {
				d.log.V(consts.LogLevelError).Error(err, "failed to update nodeMaintenance")
				return nil, err
			}
		}
	} else {
		nm, err = d.createNodeMaintenance(ctx, nodeName)
		if err != nil {
			d.log.V(consts.LogLevelError).Error(err, "failed to create nodeMaintenance")
			return nil, err
		}
	}

	return nm, nil
}

func (d *DrainRequestor) checkForExistingNodeMaintenance(ctx context.Context,
	nodeName string) (*maintenancev1alpha1.NodeMaintenance, error) {
	nm := &maintenancev1alpha1.NodeMaintenance{}
	err := d.k8sClient.Get(ctx, types.NamespacedName{Name: d.getNodeMaintenanceName(nodeName),
		Namespace: d.opts.MaintenanceOPRequestorNS},
		nm, &client.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nm, err
		}
	}
	// check if node maintenance object is already exists
	if nm.GetUID() != "" {
		return nm, nil
	}
	return nil, nil
}

func (d *DrainRequestor) createNodeMaintenance(ctx context.Context,
	nodeName string) (*maintenancev1alpha1.NodeMaintenance, error) {
	nm := d.defaultNodeMaintenance.DeepCopy()
	nm.Name = d.getNodeMaintenanceName(nodeName)
	nm.Spec.NodeName = nodeName
	err := d.k8sClient.Create(ctx, nm, &client.CreateOptions{})
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return nm, err
		}
	}
	d.log.V(consts.LogLevelInfo).Info("Creating", "node maintenance", nm, "node", nodeName)

	return nm, nil
}

func (d *DrainRequestor) deleteOrUpdateNodeMaintenance(ctx context.Context,
	nodeName string) (bool, error) {
	// check for existing nodeMaintenance obj
	nm, err := d.checkForExistingNodeMaintenance(ctx, nodeName)
	if err != nil {
		return false, err
	}
	if nm == nil {
		return true, nil
	}

	// check if object is owned by deleting requestor, if so proceed to deletion
	if nm.Spec.RequestorID == d.opts.MaintenanceOPRequestorID {
		d.log.V(consts.LogLevelInfo).Info("deleting node maintenance",
			"nodeMaintenance", client.ObjectKeyFromObject(nm))
		err := d.deleteNodeMaintenance(ctx, nm)
		if err != nil {
			d.log.V(consts.LogLevelWarning).Error(
				err, "failed to delete NodeMaintenance, node uncordon failed", "nodeMaintenance",
				client.ObjectKeyFromObject(nm))
			return false, err
		}
		return true, nil
	}
	d.log.V(consts.LogLevelInfo).Info("removing requestor from node maintenance additional requestors list",
		nm.GetName(), nm.GetNamespace())
	// remove requestorID from spec.AdditionalRequestors list and patch the object
	// check if requestorID is under additional requestors list
	if slices.Contains(nm.Spec.AdditionalRequestors, d.opts.MaintenanceOPRequestorID) {
		originalNm := nm.DeepCopy()
		nm.Spec.AdditionalRequestors = slices.DeleteFunc(nm.Spec.AdditionalRequestors, func(id string) bool {
			return id == d.opts.MaintenanceOPRequestorID
		})
		patch := client.MergeFromWithOptions(originalNm, client.MergeFromWithOptimisticLock{})
		err := d.k8sClient.Patch(ctx, nm, patch)
		if err != nil {
			return false, fmt.Errorf("failed to remove requestor from additionalRequestors."+
				"failed to patch nodeMaintenance %s. %w", client.ObjectKeyFromObject(nm), err)
		}
	}

	return false, nil
}

// deleteNodeMaintenance requests to delete nodeMaintenance obj
func (d *DrainRequestor) deleteNodeMaintenance(ctx context.Context,
	nm *maintenancev1alpha1.NodeMaintenance) error {
	if nm.Spec.RequestorID == d.opts.MaintenanceOPRequestorID {
		d.log.V(consts.LogLevelInfo).Info("Deleting",
			"nodeMaintenance", client.ObjectKeyFromObject(nm), "node", nm.Spec.NodeName)

		// send deletion request to uncordon undelying node
		// avoid deletion if deletion timestamp is already set
		if nm.DeletionTimestamp == nil {
			err := d.k8sClient.Delete(ctx, nm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// getNodeMaintenanceName returns expected name of the nodeMaintenance object
func (d *DrainRequestor) getNodeMaintenanceName(nodeName string) string {
	return fmt.Sprintf("%s-%s", d.opts.NodeMaintenanceNamePrefix, nodeName)
}

// GetDrainRequestorOpts from a drain interface
func GetDrainRequestorOpts(drainer drain.DrainInterface) RequestorOptions {
	drainRequestor, ok := drainer.(*DrainRequestor)
	if !ok {
		return RequestorOptions{}
	}

	return drainRequestor.opts
}

// GetRequestorOptsFromEnvs returns requestor upgrade related options according to
// provided environment variables
func GetRequestorOptsFromEnvs() RequestorOptions {
	opts := RequestorOptions{}
	if os.Getenv("DRAIN_CONTROLLER_ENABLED") == trueString {
		opts.UseMaintenanceOperator = true
	}
	if os.Getenv("DRAIN_CONTROLLER_REQUESTOR_NAMESPACE") != "" {
		opts.MaintenanceOPRequestorNS = os.Getenv("DRAIN_CONTROLLER_REQUESTOR_NAMESPACE")
	} else {
		opts.MaintenanceOPRequestorNS = "default"
	}
	if os.Getenv("DRAIN_CONTROLLER_REQUESTOR_ID") != "" {
		opts.MaintenanceOPRequestorID = os.Getenv("DRAIN_CONTROLLER_REQUESTOR_ID")
	}
	if os.Getenv("DRAIN_CONTROLLER_NODE_MAINTENANCE_PREFIX") != "" {
		opts.NodeMaintenanceNamePrefix = os.Getenv("DRAIN_CONTROLLER_NODE_MAINTENANCE_PREFIX")
	} else {
		opts.NodeMaintenanceNamePrefix = DefaultNodeMaintenanceNamePrefix
	}
	if os.Getenv("DRAIN_CONTROLLER_SRIOV_NODE_STATE_NAMESPACE") != "" {
		opts.SriovNodeStateNamespace = os.Getenv("DRAIN_CONTROLLER_SRIOV_NODE_STATE_NAMESPACE")
	}
	return opts
}
