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
	"hash/fnv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/Mellanox/network-operator/pkg/config"

	"github.com/go-logr/logr"
	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/revision"
)

type runtimeSpec struct {
	Namespace string
}

type cniRuntimeSpec struct {
	runtimeSpec
	CniBinDirectory     string
	IsOpenshift         bool
	ContainerResources  ContainerResourcesMap
	CniNetworkDirectory string
}

// a state skeleton intended to be embedded in structs implementing the State interface
// it provides many of the common constructs and functionality needed to implement a state.
type stateSkel struct {
	name        string
	description string
	dsOwner     string

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

// dsOwnerValue returns the ds-owner label value for the given CR.
// For NicClusterPolicy, it returns just the CRD name (preserving backwards compatibility).
// For NicNodePolicy, it returns "nnp-<name>" to allow multiple instances while keeping
// the label value short enough to fit within Kubernetes limits.
func dsOwnerValue(cr mellanoxv1alpha1.NicPolicyCR) string {
	if cr.GetCRDName() == mellanoxv1alpha1.NicClusterPolicyCRDName {
		return cr.GetCRDName()
	}
	return mellanoxv1alpha1.NicNodePolicyShortName + "-" + cr.GetName()
}

// nameSuffix returns the resource name suffix for the given CR.
// For NicClusterPolicy, it returns an empty string (preserving backwards compatibility).
// For other CR types, it returns "-<cr-name>" to ensure unique resource names.
func nameSuffix(cr mellanoxv1alpha1.NicPolicyCR) string {
	if cr.GetCRDName() == mellanoxv1alpha1.NicClusterPolicyCRDName {
		return ""
	}
	return "-" + cr.GetName()
}

// hashedNameSuffix returns a bounded resource name suffix for the given CR.
// NicClusterPolicy resources keep their existing names, while NicNodePolicy
// resources use the same short deterministic hash as OFED DaemonSet names.
func hashedNameSuffix(cr mellanoxv1alpha1.NicPolicyCR) string {
	if cr.GetCRDName() == mellanoxv1alpha1.NicClusterPolicyCRDName {
		return ""
	}
	return "-" + getStringHash(cr.GetName())
}

// getStringHash returns a short deterministic hash.
func getStringHash(s string) string {
	hasher := fnv.New32a()
	if _, err := hasher.Write([]byte(s)); err != nil {
		panic(err)
	}
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// GetSupportedGVKs returns a list of GetSupportedGVKs managed by Network Operator
func GetSupportedGVKs() []schema.GroupVersionKind {
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
		{
			Group:   "",
			Kind:    "PersistentVolumeClaim",
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

	for _, gvk := range GetSupportedGVKs() {
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
	if s.dsOwner != "" {
		labels[consts.DSOwnerLabel] = s.dsOwner
	}
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

func (s *stateSkel) deleteStateRelatedObjects(
	ctx context.Context, stateObjectsToKeep stateObjects) (bool, error) {
	stateLabel := map[string]string{
		consts.StateLabel: s.name,
	}
	if s.dsOwner != "" {
		stateLabel[consts.DSOwnerLabel] = s.dsOwner
	}
	found := false
	for _, gvk := range GetSupportedGVKs() {
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

// SetConfigHashAnnotation sets config hash annotation on DaemonSet pod templates.
func SetConfigHashAnnotation(objs []*unstructured.Unstructured, configHash string) error {
	if configHash == "" {
		return nil
	}

	for _, obj := range objs {
		if obj.GetKind() != "DaemonSet" {
			continue
		}

		annotations, found, err := unstructured.NestedStringMap(obj.Object,
			"spec", "template", "metadata", "annotations")
		if err != nil {
			return errors.Wrap(err, "failed to get pod template annotations")
		}
		if !found || annotations == nil {
			annotations = make(map[string]string)
		}

		annotations[consts.ConfigHashAnnotation] = configHash

		if err := unstructured.SetNestedStringMap(obj.Object, annotations,
			"spec", "template", "metadata", "annotations"); err != nil {
			return errors.Wrap(err, "failed to set pod template annotations")
		}
	}
	return nil
}

// handleOpenshiftClusterWideProxyConfig applies cluster-wide proxy env and TrustedCA
// from the OpenShift Proxy object. Admin env and certConfig on the caller spec take precedence.
// certConfig is the field from the component being synced (OFED or NIC Configuration Operator),
// not looked up from the CR, so one NicClusterPolicy can configure both independently.
func (s *stateSkel) handleOpenshiftClusterWideProxyConfig(
	ctx context.Context,
	cr mellanoxv1alpha1.NicPolicyCR,
	env []v1.EnvVar,
	certConfig *mellanoxv1alpha1.ConfigMapNameReference,
) ([]v1.EnvVar, *mellanoxv1alpha1.ConfigMapNameReference, error) {
	clusterWideProxyConfig, err := s.readOpenshiftProxyConfig(ctx)
	if err != nil {
		return env, certConfig, err
	}
	if clusterWideProxyConfig == nil {
		return env, certConfig, nil
	}

	env = s.setEnvFromClusterWideProxy(env, clusterWideProxyConfig)
	certConfig, err = s.handleOpenshiftTrustedCA(ctx, cr, clusterWideProxyConfig, certConfig)
	return env, certConfig, err
}

// handleOpenshiftTrustedCA returns the ConfigMap name to use for trusted CA.
// Existing certConfig from NicClusterPolicy wins; otherwise a ConfigMap is created
// when the cluster Proxy has spec.trustedCA.name set.
func (s *stateSkel) handleOpenshiftTrustedCA(
	ctx context.Context,
	cr mellanoxv1alpha1.NicPolicyCR,
	proxyConfig *osconfigv1.Proxy,
	certConfig *mellanoxv1alpha1.ConfigMapNameReference,
) (*mellanoxv1alpha1.ConfigMapNameReference, error) {
	reqLogger := log.FromContext(ctx)

	if certConfig != nil && certConfig.Name != "" {
		reqLogger.V(consts.LogLevelDebug).Info("use trusted certificate configuration from NicClusterPolicy",
			"ConfigMap", certConfig.Name)
		return certConfig, nil
	}
	if proxyConfig.Spec.TrustedCA.Name == "" {
		return certConfig, nil
	}

	ocpTrustedCAConfigMap, err := s.getOrCreateTrustedCAConfigMap(ctx, cr)
	if err != nil {
		return certConfig, err
	}
	updated := &mellanoxv1alpha1.ConfigMapNameReference{Name: ocpTrustedCAConfigMap.GetName()}
	reqLogger.V(consts.LogLevelDebug).Info("use trusted certificate configuration from Openshift cluster-Wide proxy",
		"ConfigMap", updated.Name)
	return updated, nil
}

// setEnvFromClusterWideProxy set proxy env variables from cluster wide proxy in OCP
// values which already configured in NicClusterPolicy take precedence
func (s *stateSkel) setEnvFromClusterWideProxy(env []v1.EnvVar, proxyConfig *osconfigv1.Proxy) []v1.EnvVar {
	// use [][]string to preserve order of env variables
	proxiesParams := [][]string{
		{envVarNameHTTPSProxy, proxyConfig.Spec.HTTPSProxy},
		{envVarNameHTTPProxy, proxyConfig.Spec.HTTPProxy},
		{envVarNameNoProxy, proxyConfig.Spec.NoProxy},
	}
	envsFromStaticCfg := map[string]v1.EnvVar{}
	for _, e := range env {
		envsFromStaticCfg[e.Name] = e
	}
	for _, param := range proxiesParams {
		envKey, envValue := param[0], param[1]
		if envValue == "" {
			continue
		}
		_, upperCaseExist := envsFromStaticCfg[strings.ToUpper(envKey)]
		_, lowerCaseExist := envsFromStaticCfg[strings.ToLower(envKey)]
		if upperCaseExist || lowerCaseExist {
			// environment variable statically configured in NicClusterPolicy
			continue
		}
		// add proxy settings in both cases for compatibility
		env = append(env,
			v1.EnvVar{Name: strings.ToUpper(envKey), Value: envValue},
			v1.EnvVar{Name: strings.ToLower(envKey), Value: envValue},
		)
	}
	return env
}

// readOpenshiftProxyConfig reads ClusterWide Proxy configuration for Openshift
// https://docs.openshift.com/container-platform/4.10/networking/enable-cluster-wide-proxy.html
// returns nil if object not found, error if generic API error happened
func (s *stateSkel) readOpenshiftProxyConfig(ctx context.Context) (*osconfigv1.Proxy, error) {
	proxyConfig := &osconfigv1.Proxy{}
	err := s.client.Get(ctx, types.NamespacedName{Name: "cluster"}, proxyConfig)
	if err != nil {
		if meta.IsNoMatchError(err) || k8serrors.IsNotFound(err) {
			// Proxy CRD is not registered (probably we are not in Openshift cluster)
			// or CR with name "cluster" not found
			// skip Cluster wide Proxy configuration
			return nil, nil
		}
		// retryable API error, e.g. connectivity issue
		return nil, errors.Wrap(err, "failed to read Cluster Wide proxy settings")
	}
	return proxyConfig, nil
}

// getOrCreateTrustedCAConfigMap creates or returns the ConfigMap OpenShift fills
// with the cluster trusted CA bundle (label inject-trusted-cabundle).
// Callers must decide whether TrustedCA is enabled before calling this.
func (s *stateSkel) getOrCreateTrustedCAConfigMap(
	ctx context.Context, cr mellanoxv1alpha1.NicPolicyCR) (*v1.ConfigMap, error) {
	var (
		cmName      = ocpTrustedCAConfigMapName
		cmNamespace = config.FromEnv().State.NetworkOperatorResourceNamespace
		reqLogger   = log.FromContext(ctx)
	)

	configMap := &v1.ConfigMap{}
	err := s.client.Get(ctx, types.NamespacedName{Namespace: cmNamespace, Name: cmName}, configMap)
	if err == nil {
		reqLogger.V(consts.LogLevelDebug).Info("TrustedCAConfigMap already exist",
			"name", cmName, "namespace", cmNamespace)
		if configMap.Data[ocpTrustedCABundleFileName] == "" {
			reqLogger.V(consts.LogLevelWarning).Info("TrustedCAConfigMap has empty ca-bundle.crt key",
				"name", cmName, "namespace", cmNamespace)
		}
		return configMap, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get trusted CA bundle config map %s: %s", cmName, err)
	}

	configMap = &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: cmNamespace,
			Labels:    map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"},
		},
		Data: map[string]string{
			ocpTrustedCABundleFileName: "",
		},
	}
	if err := controllerutil.SetControllerReference(cr, configMap, s.client.Scheme()); err != nil {
		return nil, err
	}

	err = s.client.Create(ctx, configMap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TrustedCAConfigMap")
	}
	reqLogger.V(consts.LogLevelInfo).Info("TrustedCAConfigMap created",
		"name", cmName, "namespace", cmNamespace)

	err = wait.PollUntilContextTimeout(ctx, ocpTrustedCAConfigMapCheckInterval,
		ocpTrustedCAConfigMapCheckTimeout, true, func(innerCtx context.Context) (bool, error) {
			err := s.client.Get(innerCtx, types.NamespacedName{Namespace: cmNamespace, Name: cmName}, configMap)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			return configMap.Data[ocpTrustedCABundleFileName] != "", nil
		})
	if err != nil {
		if !wait.Interrupted(err) {
			return nil, errors.Wrap(err, "failed to check TrustedCAConfigMap content")
		}
		reqLogger.V(consts.LogLevelWarning).Info("TrustedCAConfigMap was not populated by Openshift,"+
			"this may result in misconfiguration of trusted certificates",
			"name", cmName, "namespace", cmNamespace)
	} else {
		reqLogger.V(consts.LogLevelInfo).Info("TrustedCAConfigMap has been populated by Openshift",
			"name", cmName, "namespace", cmNamespace)
	}
	return configMap, nil
}
