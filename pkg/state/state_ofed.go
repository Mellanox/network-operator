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
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	"github.com/go-logr/logr"
	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

const (
	stateOFEDName        = "state-OFED"
	stateOFEDDescription = "OFED driver deployed in the cluster"

	// mofedImageFormat is the mofed driver container image name format
	// format: <repo>/<image-name>:<driver-version>-<os-name><os-ver>-<cpu-arch>
	// e.x: nvcr.io/nvidia/mellanox/mofed:5.7-0.1.2.0-ubuntu20.04-amd64
	mofedImageFormat = "%s/%s:%s-%s%s-%s"
)

// Openshift cluster-wide Proxy
const (
	// ocpTrustedCAConfigMapName Openshift will inject bundle with trusted CA to this ConfigMap
	ocpTrustedCAConfigMapName = "ocp-network-operator-trusted-ca"
	// ocpTrustedCABundleFileName is the name of the key in the ocpTrustedCAConfigMapName ConfigMap which
	// contains trusted CA chain injected by Openshift
	ocpTrustedCABundleFileName = "ca-bundle.crt"
	// contains target CA filename name in the container for rhcos
	ocpTrustedCATargetFileName = "tls-ca-bundle.pem"
	// if cluster-wide proxy with custom trusted CA key is defined,
	// operator need to wait for Openshift to inject this CA to the ConfigMap with ocpTrustedCAConfigMapName name
	// this const define check interval
	ocpTrustedCAConfigMapCheckInterval = time.Millisecond * 30
	// max time to wait for ConfigMap provisioning, will print warning and continue execution if
	// this timeout occurred
	ocpTrustedCAConfigMapCheckTimeout = time.Second * 15
)

// names of environment variables which used for OFED proxy configuration
const (
	envVarNameHTTPProxy  = "HTTP_PROXY"
	envVarNameHTTPSProxy = "HTTPS_PROXY"
	envVarNameNoProxy    = "NO_PROXY"
)

// CertConfigPathMap indicates standard OS specific paths for ssl keys/certificates.
// Where Go looks for certs: https://golang.org/src/crypto/x509/root_linux.go
// Where OCP mounts proxy certs on RHCOS nodes:
// https://access.redhat.com/documentation/en-us/openshift_container_platform/4.3/html/authentication/ocp-certificates#proxy-certificates_ocp-certificates
//
//nolint:lll
var CertConfigPathMap = map[string]string{
	"ubuntu": "/etc/ssl/certs",
	"rhcos":  "/etc/pki/ca-trust/extracted/pem",
}

// RepoConfigPathMap indicates standard OS specific paths for repository configuration files
var RepoConfigPathMap = map[string]string{
	"ubuntu": "/etc/apt/sources.list.d",
	"rhcos":  "/etc/yum.repos.d",
}

// ConfigMapKeysOverride contains static key override rules for ConfigMaps
// now the only use-case is to override key name in the ConfigMap which automatically
// populated by Openshift
// format is the following: {"<configMapName>": {"<keyNameInConfigMap>": "<dstFileNameInContainer>"}}
var ConfigMapKeysOverride = map[string]map[string]string{
	ocpTrustedCAConfigMapName: {ocpTrustedCABundleFileName: ocpTrustedCATargetFileName},
}

// NewStateOFED creates a new OFED driver state
func NewStateOFED(
	k8sAPIClient client.Client, scheme *runtime.Scheme, manifestDir string) (State, ManifestRenderer, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	state := &stateOFED{
		stateSkel: stateSkel{
			name:        stateOFEDName,
			description: stateOFEDDescription,
			client:      k8sAPIClient,
			scheme:      scheme,
			renderer:    renderer,
		}}
	return state, state, nil
}

type stateOFED struct {
	stateSkel
}

type additionalVolumeMounts struct {
	VolumeMounts []v1.VolumeMount
	Volumes      []v1.Volume
}

type initContainerConfig struct {
	InitContainerEnable    bool
	InitContainerImageName string
	SafeLoadEnable         bool
	SafeLoadAnnotation     string
}

type ofedRuntimeSpec struct {
	runtimeSpec
	CPUArch             string
	OSName              string
	OSVer               string
	MOFEDImageName      string
	InitContainerConfig initContainerConfig
	// is true if cluster type is Openshift
	IsOpenshift        bool
	ContainerResources ContainerResourcesMap
}

type ofedManifestRenderData struct {
	CrSpec                 *mellanoxv1alpha1.OFEDDriverSpec
	Tolerations            []v1.Toleration
	NodeAffinity           *v1.NodeAffinity
	RuntimeSpec            *ofedRuntimeSpec
	AdditionalVolumeMounts additionalVolumeMounts
}

// getCertConfigPath returns the standard OS specific path for ssl keys/certificates
func getCertConfigPath(osname string) (string, error) {
	if path, ok := CertConfigPathMap[osname]; ok {
		return path, nil
	}
	return "", fmt.Errorf("distribution not supported")
}

// getRepoConfigPath returns the standard OS specific path for repository configuration files
func getRepoConfigPath(osname string) (string, error) {
	if path, ok := RepoConfigPathMap[osname]; ok {
		return path, nil
	}
	return "", fmt.Errorf("distribution not supported")
}

// FromConfigMap generates Volumes and VolumeMounts data for the specified ConfigMap object
func (a *additionalVolumeMounts) FromConfigMap(configMap *v1.ConfigMap, destDir string) error {
	volumeMounts, itemsToInclude, err := a.createConfigMapVolumeMounts(configMap, destDir)
	if err != nil {
		return fmt.Errorf("failed to create VolumeMounts from ConfigMap: %v", err)
	}
	volume := a.createConfigMapVolume(configMap.Name, itemsToInclude)
	a.VolumeMounts = append(a.VolumeMounts, volumeMounts...)
	a.Volumes = append(a.Volumes, volume)

	return nil
}

// createConfigMapVolumeMounts creates a VolumeMount for each key
// in the ConfigMap. Use subPath to ensure original contents
// at destinationDir are not overwritten.
// nolint
func (a *additionalVolumeMounts) createConfigMapVolumeMounts(configMap *v1.ConfigMap, destinationDir string) (
	[]v1.VolumeMount, []v1.KeyToPath, error) {
	// static configMap key overrides
	cmKeyOverrides := ConfigMapKeysOverride[configMap.GetName()]
	// create one volume mount per file in the ConfigMap and use subPath
	var filenames = make([]string, 0, len(configMap.Data))
	for filename := range configMap.Data {
		filenames = append(filenames, filename)
	}
	// sort so volume mounts are added to spec in deterministic order to make testing easier
	sort.Strings(filenames)
	var itemsToInclude = make([]v1.KeyToPath, 0, len(filenames))
	var volumeMounts = make([]v1.VolumeMount, 0, len(filenames))
	for _, filename := range filenames {
		dstFilename := filename
		if override := cmKeyOverrides[filename]; override != "" {
			dstFilename = override
		}
		volumeMounts = append(volumeMounts,
			v1.VolumeMount{
				Name:      configMap.Name,
				ReadOnly:  true,
				MountPath: filepath.Join(destinationDir, dstFilename),
				SubPath:   dstFilename})
		itemsToInclude = append(itemsToInclude, v1.KeyToPath{
			Key:  filename,
			Path: dstFilename,
		})
	}
	return volumeMounts, itemsToInclude, nil
}

func (a *additionalVolumeMounts) createConfigMapVolume(configMapName string, itemsToInclude []v1.KeyToPath) v1.Volume {
	volumeSource := v1.VolumeSource{
		ConfigMap: &v1.ConfigMapVolumeSource{
			LocalObjectReference: v1.LocalObjectReference{
				Name: configMapName,
			},
			Items: itemsToInclude,
		},
	}
	return v1.Volume{Name: configMapName, VolumeSource: volumeSource}
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
//
//nolint:dupl
func (s *stateOFED) Sync(ctx context.Context, customResource interface{}, infoCatalog InfoCatalog) (SyncState, error) {
	reqLogger := log.FromContext(ctx)
	cr := customResource.(*mellanoxv1alpha1.NicClusterPolicy)
	reqLogger.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.Name, "Namespace:", cr.Namespace)

	if cr.Spec.OFEDDriver == nil {
		// Either this state was not required to run or an update occurred and we need to remove
		// the resources that where created.
		return s.handleStateObjectsDeletion(ctx)
	}
	// Fill ManifestRenderData and render objects
	nodeInfo := infoCatalog.GetNodeInfoProvider()
	if nodeInfo == nil {
		return SyncStateError, errors.New("unexpected state, catalog does not provide node information")
	}

	clusterInfo := infoCatalog.GetClusterTypeProvider()
	if clusterInfo == nil {
		return SyncStateError, errors.New("unexpected state, catalog does not provide cluster type info")
	}

	if clusterInfo.IsOpenshift() {
		if err := s.handleOpenshiftClusterWideProxyConfig(ctx, cr); err != nil {
			return SyncStateNotReady, errors.Wrap(err, "failed to handle Openshift cluster-wide proxy settings")
		}
	}

	objs, err := s.GetManifestObjects(ctx, cr, infoCatalog, log.FromContext(ctx))
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create k8s objects from manifest")
	}
	if len(objs) == 0 {
		// GetManifestObjects returned no objects, this means that no objects need to be applied to the cluster
		// as (most likely) no Mellanox hardware is found (No mellanox labels where found).
		// Return SyncStateNotReady so we retry the Sync.
		return SyncStateNotReady, nil
	}

	// Create objects if they dont exist, Update objects if they do exist
	err = s.createOrUpdateObjs(ctx, func(obj *unstructured.Unstructured) error {
		if err := controllerutil.SetControllerReference(cr, obj, s.scheme); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}
		return nil
	}, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create/update objects")
	}
	waitForStaleObjectsRemoval, err := s.handleStaleStateObjects(ctx, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to handle state stale objects")
	}
	if waitForStaleObjectsRemoval {
		return SyncStateNotReady, nil
	}
	// Check objects status
	syncState, err := s.getSyncState(ctx, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to get sync state")
	}
	return syncState, nil
}

// GetWatchSources returns map of source kinds that should be watched for the state keyed by the source kind name
func (s *stateOFED) GetWatchSources() map[string]client.Object {
	wr := make(map[string]client.Object)
	wr["DaemonSet"] = &appsv1.DaemonSet{}
	return wr
}

// handleOpenshiftClusterWideProxyConfig handles cluster-wide proxy configuration in Openshift cluster,
// populates CA an ENV configs in NicClusterPolicy with dynamic configuration from osconfigv1.Proxy object if
// these settings were not explicitly set in NicClusterPolicy by admin
func (s *stateOFED) handleOpenshiftClusterWideProxyConfig(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy) error {
	reqLogger := log.FromContext(ctx)
	// read ClusterWide Proxy configuration for Openshift
	clusterWideProxyConfig, err := s.readOpenshiftProxyConfig(ctx)
	if err != nil {
		return err
	}
	if clusterWideProxyConfig == nil {
		return nil
	}

	s.setEnvFromClusterWideProxy(cr, clusterWideProxyConfig)

	if cr.Spec.OFEDDriver.CertConfig != nil && cr.Spec.OFEDDriver.CertConfig.Name != "" {
		// CA certificate configMap explicitly set in NicClusterPolicy, ignore CA settings
		// in cluster-wide proxy
		reqLogger.V(consts.LogLevelDebug).Info("use trusted certificate configuration from NicClusterPolicy",
			"ConfigMap", cr.Spec.OFEDDriver.CertConfig.Name)
		return nil
	}

	if clusterWideProxyConfig.Spec.TrustedCA.Name == "" {
		// trustedCA is not configured in ClusterWide proxy
		return nil
	}

	ocpTrustedCAConfigMap, err := s.getOrCreateTrustedCAConfigMap(ctx, cr)
	if err != nil {
		return err
	}
	cr.Spec.OFEDDriver.CertConfig = &mellanoxv1alpha1.ConfigMapNameReference{Name: ocpTrustedCAConfigMap.GetName()}
	reqLogger.V(consts.LogLevelDebug).Info("use trusted certificate configuration from Openshift cluster-Wide proxy",
		"ConfigMap", ocpTrustedCAConfigMap.GetName())
	return nil
}

// handleAdditionalMounts generates AdditionalVolumeMounts information for the specified ConfigMap
func (s *stateOFED) handleAdditionalMounts(
	ctx context.Context, volMounts *additionalVolumeMounts, configMapName, destDir string) error {
	configMap := &v1.ConfigMap{}

	namespace := config.FromEnv().State.NetworkOperatorResourceNamespace
	objKey := client.ObjectKey{Namespace: namespace, Name: configMapName}
	err := s.client.Get(ctx, objKey, configMap)
	if err != nil {
		return fmt.Errorf("could not get ConfigMap %s from client: %v", configMapName, err)
	}

	err = volMounts.FromConfigMap(configMap, destDir)
	if err != nil {
		return fmt.Errorf("could not create volume mounts for ConfigMap: %s", configMapName)
	}

	return nil
}

//nolint:funlen
func (s *stateOFED) GetManifestObjects(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy,
	catalog InfoCatalog, reqLogger logr.Logger) ([]*unstructured.Unstructured, error) {
	nodeInfo := catalog.GetNodeInfoProvider()
	if nodeInfo == nil {
		return nil, errors.New("nodeInfo provider required")
	}
	clusterInfo := catalog.GetClusterTypeProvider()
	if clusterInfo == nil {
		return nil, errors.New("clusterInfo provider required")
	}
	attrs := nodeInfo.GetNodesAttributes(
		nodeinfo.NewNodeLabelFilterBuilder().WithLabel(nodeinfo.NodeLabelMlnxNIC, "true").Build())
	if len(attrs) == 0 {
		reqLogger.V(consts.LogLevelInfo).Info("No nodes with Mellanox NICs where found in the cluster.")
		return []*unstructured.Unstructured{}, nil
	}

	// TODO: Render daemonset multiple times according to CPUXOS matrix (ATM assume all nodes are the same)
	// Note: it is assumed MOFED driver container is able to handle multiple kernel version e.g by triggering DKMS
	// if driver was compiled against a missmatching kernel to begin with.
	if err := s.checkAttributesExist(attrs[0],
		nodeinfo.AttrTypeCPUArch, nodeinfo.AttrTypeOSName, nodeinfo.AttrTypeOSVer); err != nil {
		return nil, err
	}
	nodeAttr := attrs[0].Attributes

	setProbesDefaults(cr)

	// Update MOFED Env variables with defaults for the cluster
	cr.Spec.OFEDDriver.Env = s.mergeWithDefaultEnvs(cr.Spec.OFEDDriver.Env, nodeAttr)

	additionalVolMounts := additionalVolumeMounts{}
	osname := nodeAttr[nodeinfo.AttrTypeOSName]
	// set any custom ssl key/certificate configuration provided
	err := s.handleCertConfig(ctx, cr, osname, additionalVolMounts)
	if err != nil {
		return nil, err
	}

	// set any custom repo configuration provided
	err = s.handleRepoConfig(ctx, cr, osname, additionalVolMounts)
	if err != nil {
		return nil, err
	}

	renderData := &ofedManifestRenderData{
		CrSpec: cr.Spec.OFEDDriver,
		RuntimeSpec: &ofedRuntimeSpec{
			runtimeSpec:    runtimeSpec{config.FromEnv().State.NetworkOperatorResourceNamespace},
			CPUArch:        nodeAttr[nodeinfo.AttrTypeCPUArch],
			OSName:         nodeAttr[nodeinfo.AttrTypeOSName],
			OSVer:          nodeAttr[nodeinfo.AttrTypeOSVer],
			MOFEDImageName: s.getMofedDriverImageName(cr, nodeAttr, reqLogger),
			InitContainerConfig: s.getInitContainerConfig(cr, reqLogger,
				config.FromEnv().State.OFEDState.InitContainerImage),
			IsOpenshift:        clusterInfo.IsOpenshift(),
			ContainerResources: createContainerResourcesMap(cr.Spec.OFEDDriver.ContainerResources),
		},
		Tolerations:            cr.Spec.Tolerations,
		NodeAffinity:           cr.Spec.NodeAffinity,
		AdditionalVolumeMounts: additionalVolMounts,
	}
	// render objects
	reqLogger.V(consts.LogLevelDebug).Info("Rendering objects", "data:", renderData)
	objs, err := s.renderer.RenderObjects(&render.TemplatingData{Data: renderData})
	if err != nil {
		return nil, errors.Wrap(err, "failed to render objects")
	}
	reqLogger.V(consts.LogLevelDebug).Info("Rendered", "objects:", objs)
	return objs, nil
}

// prepare configuration for the init container,
// the init container will be disabled if the image is empty
func (s *stateOFED) getInitContainerConfig(
	cr *mellanoxv1alpha1.NicClusterPolicy, reqLogger logr.Logger, image string) initContainerConfig {
	var initContCfg initContainerConfig
	safeLoadEnable := cr.Spec.OFEDDriver.OfedUpgradePolicy != nil &&
		cr.Spec.OFEDDriver.OfedUpgradePolicy.AutoUpgrade &&
		cr.Spec.OFEDDriver.OfedUpgradePolicy.SafeLoad
	if image != "" {
		initContCfg = initContainerConfig{
			InitContainerEnable:    true,
			InitContainerImageName: image,
			SafeLoadEnable:         safeLoadEnable,
			SafeLoadAnnotation:     upgrade.GetUpgradeDriverWaitForSafeLoadAnnotationKey(),
		}
	}

	if safeLoadEnable && !initContCfg.InitContainerEnable {
		reqLogger.Error(nil, "safe driver loading feature is enabled, but init container is "+
			"disabled. It is required to enable init container to use safe driver loading feature.")
	}
	return initContCfg
}

// getMofedDriverImageName generates MOFED driver image name based on the driver version specified in CR
// TODO(adrianc): in Network-Operator v1.5.0, we should just use the new naming scheme
func (s *stateOFED) getMofedDriverImageName(cr *mellanoxv1alpha1.NicClusterPolicy,
	nodeAttr map[nodeinfo.AttributeType]string, reqLogger logr.Logger) string {
	curDriverVer, err := semver.NewVersion(cr.Spec.OFEDDriver.Version)
	if err != nil {
		reqLogger.V(consts.LogLevelDebug).Info("failed to parse ofed driver version as semver")
	}
	reqLogger.V(consts.LogLevelDebug).Info("Generating ofed driver image name for version: %v", "version", curDriverVer)

	return fmt.Sprintf(mofedImageFormat,
		cr.Spec.OFEDDriver.Repository,
		cr.Spec.OFEDDriver.Image,
		cr.Spec.OFEDDriver.Version,
		nodeAttr[nodeinfo.AttrTypeOSName],
		nodeAttr[nodeinfo.AttrTypeOSVer],
		nodeAttr[nodeinfo.AttrTypeCPUArch])
}

// readOpenshiftProxyConfig reads ClusterWide Proxy configuration for Openshift
// https://docs.openshift.com/container-platform/4.10/networking/enable-cluster-wide-proxy.html
// returns nil if object not found, error if generic API error happened
func (s *stateOFED) readOpenshiftProxyConfig(ctx context.Context) (*osconfigv1.Proxy, error) {
	proxyConfig := &osconfigv1.Proxy{}
	err := s.client.Get(ctx, types.NamespacedName{Name: "cluster"}, proxyConfig)
	if err != nil {
		if meta.IsNoMatchError(err) || apiErrors.IsNotFound(err) {
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

// getOrCreateTrustedCAConfigMap creates or returns an existing Trusted CA Bundle ConfigMap.
// returns nil ConfigMap if trustedCA is not configured
func (s *stateOFED) getOrCreateTrustedCAConfigMap(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy) (*v1.ConfigMap, error) {
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
	if err != nil && !apiErrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get trusted CA bundle config map %s: %s", cmName, err)
	}

	// configMap not found, try to create
	configMap = &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: cmNamespace,
			// apply label "config.openshift.io/inject-trusted-cabundle: true",
			// so that cert is automatically filled/updated by Openshift
			Labels: map[string]string{"config.openshift.io/inject-trusted-cabundle": "true"},
		},
		Data: map[string]string{
			ocpTrustedCABundleFileName: "",
		},
	}
	if err := controllerutil.SetControllerReference(cr, configMap, s.scheme); err != nil {
		return nil, err
	}

	err = s.client.Create(ctx, configMap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TrustedCAConfigMap")
	}
	reqLogger.V(consts.LogLevelInfo).Info("TrustedCAConfigMap created",
		"name", cmName, "namespace", cmNamespace)

	// check that CA bundle is populated by Openshift before proceed
	err = wait.PollUntilContextTimeout(ctx, ocpTrustedCAConfigMapCheckInterval,
		ocpTrustedCAConfigMapCheckTimeout, true, func(innerCtx context.Context) (bool, error) {
			err := s.client.Get(innerCtx, types.NamespacedName{Namespace: cmNamespace, Name: cmName}, configMap)
			if err != nil {
				if apiErrors.IsNotFound(err) {
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
			"this may result in misconfiguration of trusted certificates for the OFED container",
			"name", cmName, "namespace", cmNamespace)
	} else {
		reqLogger.V(consts.LogLevelInfo).Info("TrustedCAConfigMap has been populated by Openshift",
			"name", cmName, "namespace", cmNamespace)
	}
	return configMap, nil
}

// setEnvFromClusterWideProxy set proxy env variables from cluster wide proxy in OCP
// values which already configured in NicClusterPolicy take precedence
func (s *stateOFED) setEnvFromClusterWideProxy(cr *mellanoxv1alpha1.NicClusterPolicy, proxyConfig *osconfigv1.Proxy) {
	// use [][]string to preserve order of env variables
	proxiesParams := [][]string{
		{envVarNameHTTPSProxy, proxyConfig.Spec.HTTPSProxy},
		{envVarNameHTTPProxy, proxyConfig.Spec.HTTPProxy},
		{envVarNameNoProxy, proxyConfig.Spec.NoProxy},
	}
	envsFromStaticCfg := map[string]v1.EnvVar{}
	for _, e := range cr.Spec.OFEDDriver.Env {
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
		cr.Spec.OFEDDriver.Env = append(cr.Spec.OFEDDriver.Env,
			v1.EnvVar{Name: strings.ToUpper(envKey), Value: envValue},
			v1.EnvVar{Name: strings.ToLower(envKey), Value: envValue},
		)
	}
}

// mergeWithDefaultEnvs returns env variables provided in currentEnvs merged with default
// env variables for MOFED container, taking into account attributes of the k8s cluster.
func (s *stateOFED) mergeWithDefaultEnvs(
	currentEnvs []v1.EnvVar, nodeAttrs map[nodeinfo.AttributeType]string) []v1.EnvVar {
	if envVarsWithGet(currentEnvs).Get("CREATE_IFNAMES_UDEV") != nil {
		// already exists dont overwrite
		return currentEnvs
	}

	// CREATE_IFNAMES_UDEV: should be set to true for ubuntu < 22.04 , RHEL < 9, OCP < 4.13 if not provided
	osName := nodeAttrs[nodeinfo.AttrTypeOSName]
	osVer := nodeAttrs[nodeinfo.AttrTypeOSVer]
	createIfnameUdevEnv := v1.EnvVar{Name: "CREATE_IFNAMES_UDEV", Value: "true"}

	currVer, err := semver.NewVersion(osVer)
	if err != nil {
		return currentEnvs
	}

	switch osName {
	case "ubuntu":
		if currVer.LessThan(semver.MustParse("22.04")) {
			currentEnvs = append(currentEnvs, createIfnameUdevEnv)
		}
	case "rhel":
		if currVer.LessThan(semver.MustParse("9.0")) {
			currentEnvs = append(currentEnvs, createIfnameUdevEnv)
		}
	case "rhcos":
		if currVer.LessThan(semver.MustParse("4.13")) {
			currentEnvs = append(currentEnvs, createIfnameUdevEnv)
		}
	}

	return currentEnvs
}

// envVarsWithGet is a wrapper type for []EnvVar to extend with additional functionality
type envVarsWithGet []v1.EnvVar

// Get returns pointer to EnvVar if found in the list, else returns nil
func (e envVarsWithGet) Get(name string) *v1.EnvVar {
	for i := range e {
		if e[i].Name == name {
			return &e[i]
		}
	}

	return nil
}

// setProbesDefaults populates NicClusterPolicy CR with default Probe values
// if not provided by user
func setProbesDefaults(cr *mellanoxv1alpha1.NicClusterPolicy) {
	if cr.Spec.OFEDDriver.StartupProbe == nil {
		cr.Spec.OFEDDriver.StartupProbe = &mellanoxv1alpha1.PodProbeSpec{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
		}
	}

	if cr.Spec.OFEDDriver.LivenessProbe == nil {
		cr.Spec.OFEDDriver.LivenessProbe = &mellanoxv1alpha1.PodProbeSpec{
			InitialDelaySeconds: 30,
			PeriodSeconds:       30,
		}
	}

	if cr.Spec.OFEDDriver.ReadinessProbe == nil {
		cr.Spec.OFEDDriver.ReadinessProbe = &mellanoxv1alpha1.PodProbeSpec{
			InitialDelaySeconds: 10,
			PeriodSeconds:       30,
		}
	}
}

// handleCertConfig handles additional mounts required for Certificates if specified
func (s *stateOFED) handleCertConfig(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy, osname string, mounts additionalVolumeMounts) error {
	if cr.Spec.OFEDDriver.CertConfig != nil && cr.Spec.OFEDDriver.CertConfig.Name != "" {
		destinationDir, err := getCertConfigPath(osname)
		if err != nil {
			return fmt.Errorf("failed to get destination directory for custom TLS certificates config: %v", err)
		}

		err = s.handleAdditionalMounts(ctx, &mounts, cr.Spec.OFEDDriver.CertConfig.Name, destinationDir)
		if err != nil {
			return fmt.Errorf("failed to mount volumes for custom TLS certificates: %v", err)
		}
	}
	return nil
}

// handleRepoConfig handles additional mounts required for custom repo if specified
func (s *stateOFED) handleRepoConfig(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy, osname string, mounts additionalVolumeMounts) error {
	if cr.Spec.OFEDDriver.RepoConfig != nil && cr.Spec.OFEDDriver.RepoConfig.Name != "" {
		destinationDir, err := getRepoConfigPath(osname)
		if err != nil {
			return fmt.Errorf("failed to get destination directory for custom repo config: %v", err)
		}

		err = s.handleAdditionalMounts(ctx, &mounts, cr.Spec.OFEDDriver.RepoConfig.Name, destinationDir)
		if err != nil {
			return fmt.Errorf("failed to mount volumes for custom repositories configuration: %v", err)
		}
	}
	return nil
}
