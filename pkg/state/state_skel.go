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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
)

type runtimeSpec struct {
	Namespace string
}

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

func getSupportedGVKs() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		{
			Group:   "",
			Kind:    "ServiceAccount",
			Version: "v1",
		},
		{
			Group:   "",
			Kind:    "ConfigMap",
			Version: "v1",
		},
		{
			Group:   "apps",
			Kind:    "DaemonSet",
			Version: "v1",
		},
		{
			Group:   "apps",
			Kind:    "Deployment",
			Version: "v1",
		},
		{
			Group:   "apiextensions.k8s.io",
			Kind:    "CustomResourceDefinition",
			Version: "v1",
		},
		{
			Group:   "rbac.authorization.k8s.io",
			Kind:    "ClusterRole",
			Version: "v1",
		},
		{
			Group:   "rbac.authorization.k8s.io",
			Kind:    "ClusterRoleBinding",
			Version: "v1",
		},
		{
			Group:   "rbac.authorization.k8s.io",
			Kind:    "Role",
			Version: "v1",
		},
		{
			Group:   "rbac.authorization.k8s.io",
			Kind:    "RoleBinding",
			Version: "v1",
		},
		{
			Group:   "k8s.cni.cncf.io",
			Kind:    "NetworkAttachmentDefinition",
			Version: "v1",
		},
		{
			Group:   "batch",
			Kind:    "CronJob",
			Version: "v1",
		},
		{
			Group:   "security.openshift.io",
			Kind:    "SecurityContextConstraints",
			Version: "v1",
		},
	}
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
	s.checkDeleteSupported(obj)
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

func (s *stateSkel) checkDeleteSupported(obj *unstructured.Unstructured) {
	for _, gvk := range getSupportedGVKs() {
		objGvk := obj.GroupVersionKind()
		if objGvk.Group == gvk.Group && objGvk.Version == gvk.Version && objGvk.Kind == gvk.Kind {
			return
		}
	}
	log.V(consts.LogLevelWarning).Info("Object will not be deleted if needed",
		"Namespace:", obj.GetNamespace(), "Name:", obj.GetName(), "GVK", obj.GroupVersionKind())
}

func (s *stateSkel) updateObj(obj *unstructured.Unstructured) error {
	log.V(consts.LogLevelInfo).Info("Updating Object", "Namespace:", obj.GetNamespace(), "Name:", obj.GetName())
	applyPatchData, err := yaml.Marshal(obj)
	if err != nil {
		log.V(consts.LogLevelError).Error(err, "failed to encode apply patch data")
		return err
	}
	patchUseForce := true
	// use server-side apply
	if err := s.client.Patch(context.TODO(), obj, client.RawPatch(types.ApplyPatchType, applyPatchData),
		client.FieldOwner(consts.KubernetesClientUserAgent),
		&client.PatchOptions{Force: &patchUseForce}); err != nil {
		return errors.Wrap(err, "failed to update resource")
	}
	log.V(consts.LogLevelInfo).Info("Object updated successfully")
	return nil
}

func (s *stateSkel) createOrUpdateObjs(
	setControllerReference func(obj *unstructured.Unstructured) error,
	objs []*unstructured.Unstructured) error {
	for _, desiredObj := range objs {
		log.V(consts.LogLevelInfo).Info("Handling manifest object", "Kind:", desiredObj.GetKind(),
			"Name", desiredObj.GetName())
		// Set controller reference for object to allow cleanup on CR deletion
		if err := setControllerReference(desiredObj); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}

		s.addStateSpecificLabels(desiredObj)

		err := s.createObj(desiredObj)
		if err == nil {
			// object created successfully
			continue
		}
		if !k8serrors.IsAlreadyExists(err) {
			// Some error occurred
			return err
		}

		// Object found, Update it
		if err := s.updateObj(desiredObj); err != nil {
			return err
		}
	}
	return nil
}

func (s *stateSkel) addStateSpecificLabels(obj *unstructured.Unstructured) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[consts.StateLabel] = s.name
	obj.SetLabels(labels)
}

func (s *stateSkel) handleStateObjectsDeletion() (SyncState, error) {
	log.V(consts.LogLevelInfo).Info(
		"State spec in CR is nil, deleting existing objects if needed", "State:", s.name)
	found, err := s.deleteStateRelatedObjects()
	if err != nil {
		return SyncStateError, errors.Wrap(err, "failed to delete k8s objects")
	}
	if found {
		log.V(consts.LogLevelInfo).Info("State deleting objects in progress", "State:", s.name)
		return SyncStateNotReady, nil
	}
	return SyncStateIgnore, nil
}

func (s *stateSkel) deleteStateRelatedObjects() (bool, error) {
	stateLabel := map[string]string{
		consts.StateLabel: s.name,
	}
	found := false
	for _, gvk := range getSupportedGVKs() {
		l := &unstructured.UnstructuredList{}
		l.SetGroupVersionKind(gvk)
		err := s.client.List(context.Background(), l, client.MatchingLabels(stateLabel))
		if meta.IsNoMatchError(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		if len(l.Items) > 0 {
			found = true
		}
		for _, obj := range l.Items {
			obj := obj
			if obj.GetDeletionTimestamp() == nil {
				err := s.client.Delete(context.TODO(), &obj)
				if err != nil {
					return true, err
				}
			}
		}
	}
	return found, nil
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
		"UpdatedPodsScheduled", ds.Status.UpdatedNumberScheduled,
		"PodsReady:", ds.Status.NumberReady,
		"Conditions:", ds.Status.Conditions)
	// Note(adrianc): We check for DesiredNumberScheduled!=0 as we expect to have at least one node that would need
	// to have DaemonSet Pods deployed onto it. DesiredNumberScheduled == 0 then indicates that this field was not yet
	// updated by the DaemonSet controller
	// TODO: Check if we can use another field maybe to indicate it was processed by the DaemonSet controller.
	if ds.Status.DesiredNumberScheduled != 0 && ds.Status.DesiredNumberScheduled == ds.Status.NumberAvailable &&
		ds.Status.UpdatedNumberScheduled == ds.Status.NumberAvailable {
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
