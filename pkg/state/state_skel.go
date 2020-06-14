package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/mellanox-network-operator/pkg/consts"
	"github.com/Mellanox/mellanox-network-operator/pkg/nodeinfo"
	"github.com/Mellanox/mellanox-network-operator/pkg/render"
)

// a state skeleton intended to be embedded in structs implementing the State interface
// it provides many of the common constructs and functionality needed to implement a state.
type stateSkel struct {
	name        string
	description string

	client   client.Client
	scheme   *runtime.Scheme
	renderer render.Renderer
}

// Name provides the State name
func (s *stateSkel) Name() string {
	return s.name
}

// Description provides the State description
func (s *stateSkel) Description() string {
	return s.description
}

func (s *stateSkel) createOrUpdateObjs(
	setControllerReference func(obj *unstructured.Unstructured) error,
	objs []*unstructured.Unstructured) error {
	for _, obj := range objs {
		log.V(consts.LogLevelInfo).Info("Handling manifest object", "Kind:", obj.GetKind(), "Name", obj.GetName())
		// Set controller reference for object to allow cleanup on CR deletion
		if err := setControllerReference(obj); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}
		// Check if this object already exists
		found := obj.DeepCopy()
		err := s.client.Get(
			context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
		if err != nil && k8serrors.IsNotFound(err) {
			// does not exist, create it.
			log.V(consts.LogLevelInfo).Info("Object does not exist, Creating object")
			if err = s.client.Create(context.TODO(), obj); err != nil {
				return errors.Wrap(err, "failed to create object")
			}
			log.V(consts.LogLevelInfo).Info("Object created successfully")
			continue
		} else if err != nil {
			// error occurred fetching object
			return errors.Wrapf(
				err, "failed to get object: Name: %s, Namespace: %s", obj.GetName(), obj.GetNamespace())
		}

		log.V(consts.LogLevelInfo).Info("Object found, Updating object.")
		// Object found, Update it
		// Note: Some objects may require update of the resource version
		// TODO: using Patch preserves runtime attributes. consider using patch if relevant
		required := obj.DeepCopy()

		if err := s.client.Update(context.TODO(), required); err != nil {
			return errors.Wrap(err, "failed to update resource")
		}
		log.V(consts.LogLevelInfo).Info("Object updated successfully")
	}
	return nil
}

// Iterate over objects and check for their readiness
func (s *stateSkel) getSyncState(objs []*unstructured.Unstructured) (SyncState, error) {
	log.V(consts.LogLevelInfo).Info("Checking related object states")
	for _, obj := range objs {
		log.V(consts.LogLevelInfo).Info("Checking object", "Kind:", obj.GetKind(), "Name", obj.GetName())
		// Check if object exists
		found := obj.DeepCopy()
		err := s.client.Get(
			context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
		if err != nil {
			log.V(consts.LogLevelInfo).Info("Object is not ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
			if k8serrors.IsNotFound(err) {
				// does not exist (yet)
				return SyncStateNotReady, nil
			}
			// other error
			return SyncStateNotReady, errors.Wrapf(err, "failed to get object")
		}
		// Object exists, check for Kind specific readiness
		if found.GetKind() == "DaemonSet" {
			if ready, err := s.daemonsetReady(found); err != nil || !ready {
				log.V(consts.LogLevelInfo).Info("Object is not ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
				return SyncStateNotReady, err
			}
		}
		log.V(consts.LogLevelInfo).Info("Object is ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
	}
	return SyncStateReady, nil
}

// daemonsetReady checks if daemonset is ready
func (s *stateSkel) daemonsetReady(uds *unstructured.Unstructured) (bool, error) {
	buf, err := uds.MarshalJSON()
	if err != nil {
		return false, errors.Wrap(err, "failed to marshall unstructured daemonset object")
	}

	ds := &appsv1.DaemonSet{}
	if err = json.Unmarshal(buf, ds); err != nil {
		return false, errors.Wrap(err, "failed to unmarshall to daemonset object")
	}

	log.V(consts.LogLevelDebug).Info(
		"Check daemonset state",
		"Desired:", ds.Status.DesiredNumberScheduled,
		"Current:", ds.Status.CurrentNumberScheduled,
		"Conditions:", ds.Status.Conditions)
	if ds.Status.DesiredNumberScheduled != 0 && ds.Status.DesiredNumberScheduled == ds.Status.CurrentNumberScheduled {
		return true, nil
	}
	return false, nil
}

// Check if provided attrTypes are present in NodeAttributes.Attributes
func (s *stateSkel) checkAttributesExist(attrs nodeinfo.NodeAttributes, attrTypes ...nodeinfo.AttributeType) error {
	for _, t := range attrTypes {
		if _, ok := attrs.Attributes[t]; !ok {
			return fmt.Errorf("mandatory node attribute does not exist for node %s", attrs.Name)
		}
	}
	return nil
}
