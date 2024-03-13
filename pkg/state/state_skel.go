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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/revision"
)

type runtimeSpec struct {
	Namespace string
}

type cniRuntimeSpec struct {
	runtimeSpec
	CniBinDirectory    string
	IsOpenshift        bool
	ContainerResources ContainerResourcesMap
}

// a state skeleton intended to be embedded in structs implementing the State interface
// it provides many of the common constructs and functionality needed to implement a state.
type stateSkel struct {
	name        string
	description string

	client   client.Client
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
			Group:   "",
			Kind:    "Service",
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
			Group:   "admissionregistration.k8s.io",
			Kind:    "ValidatingWebhookConfiguration",
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
			Group:   "cert-manager.io",
			Kind:    "Issuer",
			Version: "v1",
		},
		{
			Group:   "cert-manager.io",
			Kind:    "Certificate",
			Version: "v1",
		},
	}
}

func (s *stateSkel) getObj(ctx context.Context, obj *unstructured.Unstructured) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Get Object", "Namespace:", obj.GetNamespace(), "Name:", obj.GetName())

	err := s.client.Get(
		ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj)
	if k8serrors.IsNotFound(err) {
		// does not exist (yet)
		reqLogger.V(consts.LogLevelInfo).Info("Object Does not Exists")
	}
	return err
}

func (s *stateSkel) createObj(ctx context.Context, obj *unstructured.Unstructured) error {
	reqLogger := log.FromContext(ctx)

	s.checkDeleteSupported(ctx, obj)
	reqLogger.V(consts.LogLevelInfo).Info("Creating Object", "Namespace:", obj.GetNamespace(), "Name:", obj.GetName())
	toCreate := obj.DeepCopy()
	if err := s.client.Create(ctx, toCreate); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			reqLogger.V(consts.LogLevelInfo).Info("Object Already Exists")
		}
		return err
	}
	reqLogger.V(consts.LogLevelInfo).Info("Object created successfully")
	return nil
}

func (s *stateSkel) checkDeleteSupported(ctx context.Context, obj *unstructured.Unstructured) {
	reqLogger := log.FromContext(ctx)

	for _, gvk := range getSupportedGVKs() {
		objGvk := obj.GroupVersionKind()
		if objGvk.Group == gvk.Group && objGvk.Version == gvk.Version && objGvk.Kind == gvk.Kind {
			return
		}
	}
	reqLogger.V(consts.LogLevelWarning).Info("Object will not be deleted if needed",
		"Namespace:", obj.GetNamespace(), "Name:", obj.GetName(), "GVK", obj.GroupVersionKind())
}

func (s *stateSkel) updateObj(ctx context.Context, obj *unstructured.Unstructured) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Updating Object", "Namespace:", obj.GetNamespace(), "Name:", obj.GetName())

	// Note: Some objects may require update of the resource version
	// TODO: using Patch preserves runtime attributes. In the future consider using patch if relevant
	desired := obj.DeepCopy()
	if err := s.client.Update(ctx, desired); err != nil {
		return errors.Wrap(err, "failed to update resource")
	}
	reqLogger.V(consts.LogLevelInfo).Info("Object updated successfully")
	return nil
}

func (s *stateSkel) createOrUpdateObjs(
	ctx context.Context,
	setControllerReference func(obj *unstructured.Unstructured) error,
	objs []*unstructured.Unstructured) error {
	reqLogger := log.FromContext(ctx)
	for _, desiredObj := range objs {
		reqLogger.V(consts.LogLevelInfo).Info("Handling manifest object", "Kind:", desiredObj.GetKind(),
			"Name", desiredObj.GetName())
		// Set controller reference for object to allow cleanup on CR deletion
		if err := setControllerReference(desiredObj); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}

		s.addStateSpecificLabels(desiredObj)

		desiredRev, err := revision.CalculateRevision(desiredObj)
		if err != nil {
			return err
		}
		revision.SetRevision(desiredObj, desiredRev)

		alreadyExist := true
		currentObj := desiredObj.NewEmptyInstance().(*unstructured.Unstructured)
		currentObj.SetName(desiredObj.GetName())
		currentObj.SetNamespace(desiredObj.GetNamespace())
		if err := s.getObj(ctx, currentObj); err != nil {
			if k8serrors.IsNotFound(err) {
				alreadyExist = false
			} else {
				return err
			}
		}
		if !alreadyExist {
			err := s.createObj(ctx, desiredObj)
			if err != nil {
				return err
			}
			continue
		}
		currRev := revision.GetRevision(currentObj)
		if currRev != 0 && currRev == desiredRev {
			reqLogger.V(consts.LogLevelInfo).Info("Object is already in sync")
			continue
		}
		// update required
		if err := s.mergeObjects(desiredObj, currentObj); err != nil {
			return err
		}
		if err := s.updateObj(ctx, desiredObj); err != nil {
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

func (s *stateSkel) handleStateObjectsDeletion(ctx context.Context) (SyncState, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info(
		"State spec in CR is nil, deleting existing objects if needed", "State:", s.name)
	found, err := s.deleteStateRelatedObjects(ctx, stateObjects{})
	if err != nil {
		return SyncStateError, errors.Wrap(err, "failed to delete k8s objects")
	}
	if found {
		reqLogger.V(consts.LogLevelInfo).Info("State deleting objects in progress", "State:", s.name)
		return SyncStateNotReady, nil
	}
	return SyncStateIgnore, nil
}

// is a mapping where GVK is a key and a map(set) with NamespacedNames is a value
type stateObjects map[schema.GroupVersionKind]map[types.NamespacedName]struct{}

// Add object name to the stateObjects map
func (s stateObjects) Add(gvk schema.GroupVersionKind, name types.NamespacedName) {
	byType := s[gvk]
	if byType == nil {
		byType = make(map[types.NamespacedName]struct{})
	}
	byType[name] = struct{}{}
	s[gvk] = byType
}

// Exist checks if object name exist in the stateObjects map
func (s stateObjects) Exist(gvk schema.GroupVersionKind, name types.NamespacedName) bool {
	_, exist := s[gvk][name]
	return exist
}

// remove stale object of the state, returns boolean which indicates if removal is in progress and
// an error if failed to remove an object
func (s *stateSkel) handleStaleStateObjects(ctx context.Context,
	desiredObjs []*unstructured.Unstructured) (bool, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info(
		"check state for stale objects", "State:", s.name)
	objsToKeep := stateObjects{}
	for _, o := range desiredObjs {
		objsToKeep.Add(o.GroupVersionKind(), types.NamespacedName{Name: o.GetName(), Namespace: o.GetNamespace()})
	}
	found, err := s.deleteStateRelatedObjects(ctx, objsToKeep)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete k8s objects")
	}
	if found {
		reqLogger.V(consts.LogLevelInfo).Info("removal of the state stale objects is in progress ",
			"State:", s.name)
		return true, nil
	}
	reqLogger.V(consts.LogLevelInfo).Info("no stale objects detected", "State:", s.name)
	return false, nil
}

func (s *stateSkel) deleteStateRelatedObjects(ctx context.Context, stateObjectsToKeep stateObjects) (bool, error) {
	stateLabel := map[string]string{
		consts.StateLabel: s.name,
	}
	found := false
	for _, gvk := range getSupportedGVKs() {
		l := &unstructured.UnstructuredList{}
		l.SetGroupVersionKind(gvk)
		err := s.client.List(ctx, l, client.MatchingLabels(stateLabel))
		if meta.IsNoMatchError(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		for _, obj := range l.Items {
			obj := obj
			if stateObjectsToKeep.Exist(gvk, types.NamespacedName{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace()}) {
				// should keep the object
				continue
			}
			found = true
			if obj.GetDeletionTimestamp() == nil {
				err := s.client.Delete(ctx, &obj)
				if err != nil {
					return true, err
				}
			}
		}
	}
	return found, nil
}

func (s *stateSkel) mergeObjects(updated, current *unstructured.Unstructured) error {
	// Set resource version
	// ResourceVersion must be passed unmodified back to the server.
	// ResourceVersion helps the kubernetes API server to implement optimistic concurrency for PUT operations
	// when two PUT requests are specifying the resourceVersion, one of the PUTs will fail.
	updated.SetResourceVersion(current.GetResourceVersion())

	gvk := updated.GroupVersionKind()
	if gvk.Group == "" && gvk.Kind == "ServiceAccount" {
		return s.mergeServiceAccount(updated, current)
	}
	return nil
}

// For Service Account, keep secrets if exists
func (s *stateSkel) mergeServiceAccount(updated, current *unstructured.Unstructured) error {
	curSecrets, ok, err := unstructured.NestedSlice(current.Object, "secrets")
	if err != nil {
		return err
	}
	if ok {
		if err := unstructured.SetNestedField(updated.Object, curSecrets, "secrets"); err != nil {
			return err
		}
	}

	curImagePullSecrets, ok, err := unstructured.NestedSlice(current.Object, "imagePullSecrets")
	if err != nil {
		return err
	}
	if ok {
		if err := unstructured.SetNestedField(updated.Object, curImagePullSecrets, "imagePullSecrets"); err != nil {
			return err
		}
	}
	return nil
}

// Iterate over objects and check for their readiness
func (s *stateSkel) getSyncState(ctx context.Context, objs []*unstructured.Unstructured) (SyncState, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Checking related object states")

	for _, obj := range objs {
		reqLogger.V(consts.LogLevelInfo).Info("Checking object", "Kind:", obj.GetKind(), "Name", obj.GetName())
		// Check if object exists
		found := obj.DeepCopy()
		err := s.getObj(ctx, found)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// does not exist (yet)
				reqLogger.V(consts.LogLevelInfo).Info("Object is not ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
				return SyncStateNotReady, nil
			}
			// other error
			return SyncStateNotReady, errors.Wrapf(err, "failed to get object")
		}

		// Object exists, check for Kind specific readiness
		if found.GetKind() == "DaemonSet" {
			if ready, err := s.isDaemonSetReady(found, reqLogger); err != nil || !ready {
				reqLogger.V(consts.LogLevelInfo).Info("Object is not ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
				return SyncStateNotReady, err
			}
		}
		reqLogger.V(consts.LogLevelInfo).Info("Object is ready", "Kind:", obj.GetKind(), "Name", obj.GetName())
	}
	return SyncStateReady, nil
}

// isDaemonSetReady checks if daemonset is ready
func (s *stateSkel) isDaemonSetReady(uds *unstructured.Unstructured, reqLogger logr.Logger) (bool, error) {
	buf, err := uds.MarshalJSON()
	if err != nil {
		return false, errors.Wrap(err, "failed to marshall unstructured daemonset object")
	}

	ds := &appsv1.DaemonSet{}
	if err = json.Unmarshal(buf, ds); err != nil {
		return false, errors.Wrap(err, "failed to unmarshall to daemonset object")
	}

	reqLogger.V(consts.LogLevelDebug).Info(
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

func (s *stateSkel) SetRenderer(renderer render.Renderer) {
	s.renderer = renderer
}
