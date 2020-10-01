/*
Copyright 2020 NVIDIA

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

	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
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

func (s *stateSkel) getObj(obj *unstructured.Unstructured) error {
	log.V(consts.LogLevelInfo).Info("Get Object", "Namespace:", obj.GetNamespace(), "Name:", obj.GetName())
	err := s.client.Get(
		context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj)
	if k8serrors.IsNotFound(err) {
		// does not exist (yet)
		log.V(consts.LogLevelInfo).Info("Object Does not Exists")
	}
	return err
}

func (s *stateSkel) createObj(obj *unstructured.Unstructured) error {
	log.V(consts.LogLevelInfo).Info("Creating Object", "Namespace:", obj.GetNamespace(), "Name:", obj.GetName())
	toCreate := obj.DeepCopy()
	if err := s.client.Create(context.TODO(), toCreate); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			log.V(consts.LogLevelInfo).Info("Object Already Exists")
		}
		return err
	}
	log.V(consts.LogLevelInfo).Info("Object created successfully")
	return nil
}

func (s *stateSkel) updateObj(obj *unstructured.Unstructured) error {
	log.V(consts.LogLevelInfo).Info("Updating Object", "Namespace:", obj.GetNamespace(), "Name:", obj.GetName())
	// Note: Some objects may require update of the resource version
	// TODO: using Patch preserves runtime attributes. In the future consider using patch if relevant
	desired := obj.DeepCopy()
	if err := s.client.Update(context.TODO(), desired); err != nil {
		return errors.Wrap(err, "failed to update resource")
	}
	log.V(consts.LogLevelInfo).Info("Object updated successfully")
	return nil
}

// When updating, not all resources need an updated ResourceVersion
func needToUpdateResourceVersion(kind string) bool {
	if kind == "SecurityContextConstraints" ||
		kind == "Service" ||
		kind == "ServiceMonitor" ||
		kind == "Route" ||
		kind == "BuildConfig" ||
		kind == "ImageStream" ||
		kind == "PrometheusRule" {
		return true
	}
	return false
}

func checkNestedFields(found bool, err error) {
	if !found || err != nil {
		return errors.Wrap(err, "nested field error")
	}
}

func updateResourceVersion(req, found *unstructured.Unstructured) error {
	kind := found.GetKind()

	if needToUpdateResourceVersion(kind) {
		version, fnd, err := unstructured.NestedString(found.Object, "metadata", "resourceVersion")
		checkNestedFields(fnd, err)

		if err := unstructured.SetNestedField(req.Object, version, "metadata", "resourceVersion"); err != nil {
			return errors.Wrap(err, "Couldn't update ResourceVersion")
		}
	}

	if kind == "Service" {
		clusterIP, fnd, err := unstructured.NestedString(found.Object, "spec", "clusterIP")
		checkNestedFields(fnd, err)

		if err := unstructured.SetNestedField(req.Object, clusterIP, "spec", "clusterIP"); err != nil {
			return errors.Wrap(err, "Couldn't update clusterIP")
		}
		return nil
	}

	return nil
}

func (s *stateSkel) createOrUpdateObjs(
	setControllerReference func(obj *unstructured.Unstructured) error,
	objs []*unstructured.Unstructured) error {
	for _, desiredObj := range objs {
		log.V(consts.LogLevelInfo).Info("Handling manifest object", "Kind:", desiredObj.GetKind(),
			"Name", desiredObj.GetName())
		found := desiredObj.DeepCopy()

		// Set controller reference for object to allow cleanup on CR deletion
		if err := setControllerReference(desiredObj); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}

		err := s.getObj(found)
		if k8serrors.IsNotFound(err) {
			log.V(consts.LogLevelInfo).Info("Object not found, creating", "Kind:", desiredObj.GetKind(),
				"Name", desiredObj.GetName())
			if err := s.createObj(desiredObj); err != nil {
				return errors.Wrap(err, "couldn't create resource")
			}
			return nil
		}

		if k8serrors.IsForbidden(err) {
			return errors.Wrap(err, "forbidden, check operator RBAC permissions")
		}

		if err != nil {
			return errors.Wrap(err, "unexpected error")
		}

		// Short circuit updating ServiceAccounts and Pods
		// For Pods we can only update image and some minor fields
		// ServiceAccounts cannot be updated
		if desiredObj.GetKind() == "ServiceAccount" || desiredObj.GetKind() == "Pod" {
			return nil
		}

		log.V(consts.LogLevelInfo).Info("Object found, updating", "Kind:", desiredObj.GetKind(),
			"Name", desiredObj.GetName())

		required := desiredObj.DeepCopy()
		// required.ResourceVersion = found.ResourceVersion this is only needed on resource updates
		if err := updateResourceVersion(required, found); err != nil {
			return errors.Wrap(err, "couldn't update ResourceVersion")
		}

		// Object found, Update it
		if err := s.updateObj(required); err != nil {
			return err
		}
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
		err := s.getObj(found)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// does not exist (yet)
				log.V(consts.LogLevelInfo).Info("Object is not ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
				return SyncStateNotReady, nil
			}
			// other error
			return SyncStateNotReady, errors.Wrapf(err, "failed to get object")
		}

		// Object exists, check for Kind specific readiness
		if found.GetKind() == "DaemonSet" {
			if ready, err := s.isDaemonSetReady(found); err != nil || !ready {
				log.V(consts.LogLevelInfo).Info("Object is not ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
				return SyncStateNotReady, err
			}
		}
		log.V(consts.LogLevelInfo).Info("Object is ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
	}
	return SyncStateReady, nil
}

// isDaemonSetReady checks if daemonset is ready
func (s *stateSkel) isDaemonSetReady(uds *unstructured.Unstructured) (bool, error) {
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
		"DesiredNodes:", ds.Status.DesiredNumberScheduled,
		"CurrentNodes:", ds.Status.CurrentNumberScheduled,
		"PodsAvailable:", ds.Status.NumberAvailable,
		"PodsUnavailable:", ds.Status.NumberUnavailable,
		"PodsReady:", ds.Status.NumberReady,
		"Conditions:", ds.Status.Conditions)
	// Note(adrianc): We check for DesiredNumberScheduled!=0 as we expect to have at least one node that would need
	// to have DaemonSet Pods deployed onto it. DesiredNumberScheduled == 0 then indicates that this field was not yet
	// updated by the DaemonSet controller
	// TODO: Check if we can use another field maybe to indicate it was processed by the DaemonSet controller.
	if ds.Status.DesiredNumberScheduled != 0 && ds.Status.DesiredNumberScheduled == ds.Status.NumberAvailable {
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
