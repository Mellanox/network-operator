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

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	"github.com/go-logr/logr"
	apiimagev1 "github.com/openshift/api/image/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

const (
	stateOFEDName        = "state-OFED"
	stateOFEDDescription = "OFED driver deployed in the cluster"

	// mofedImageFormat is the DOCA driver container image name format
	// format: <repo>/<image-name>:<driver-container-version>-<os-name><os-ver>-<cpu-arch>
	// e.x: nvcr.io/nvidia/mellanox/doca-driver:5.7-0.1.2.0-0-ubuntu20.04-amd64
	mofedImageFormat = "%s/%s:%s-%s%s-%s"

	// precompiledTagFormat is the tag format for precompiled drivers.
	// format: <container-driver-version>-<kernel-full>-<os-name><os-ver>-<cpu-arch>
	precompiledTagFormat = "%s-%s-%s%s-%s"

	// precompiledImageFormat is the precompiled DOCA driver container image name format
	// format: <repo>/<image-name>:<driver-container-version>-<kernel-full>-<os-name><os-ver>-<cpu-arch>
	// e.x: nvcr.io/nvidia/mellanox/doca-driver:5.7-0.1.2.0-0-5.15.0-91-generic-ubuntu22.04-amd64
	precompiledImageFormat = "%s/%s:%s-%s-%s%s-%s"

	// sha256ImageFormat is the sha256 DOCA driver container image name format
	// format: <repo>/<image-name>@<sha256-hash>
	sha256ImageFormat = "%s/%s@%s"

	// staleOFEDGracePeriod bounds how long a MOFED DaemonSet whose node pool has no eligible
	// nodes is kept before it is reaped. A pool reaching zero nodes carries no information about
	// intent: a node that is NotReady, tainted or rebooting leaves pool discovery exactly like a
	// node whose kernel was upgraded away. The DaemonSet is therefore kept until the pool has
	// been absent long enough that a retired pool is the only remaining explanation.
	staleOFEDGracePeriod = 20 * time.Minute
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
	envVarNameHTTPProxy         = "HTTP_PROXY"
	envVarNameHTTPSProxy        = "HTTPS_PROXY"
	envVarNameNoProxy           = "NO_PROXY"
	envVarCreateIfNamesUdev     = "CREATE_IFNAMES_UDEV"
	envVarDriversInventoryPath  = "NVIDIA_NIC_DRIVERS_INVENTORY_PATH"
	defaultDriversInventoryPath = "/mnt/drivers-inventory"
)

const (
	startupInitialDelaySeconds = 30
	startupPeriodSeconds       = 10
	startupFailureThreshold    = 120
	startupTimeoutSeconds      = 10

	defaultInitialDelaySeconds = 10
	defaultPeriodSeconds       = 30
	defaultFailureThreshold    = 1
	defaultTimeoutSeconds      = 10
)

var (
	startupProbeSpec = mellanoxv1alpha1.PodProbeSpec{
		InitialDelaySeconds: startupInitialDelaySeconds,
		PeriodSeconds:       startupPeriodSeconds,
		FailureThreshold:    startupFailureThreshold,
		TimeoutSeconds:      startupTimeoutSeconds,
	}

	defaultProbeSpec = mellanoxv1alpha1.PodProbeSpec{
		InitialDelaySeconds: defaultInitialDelaySeconds,
		PeriodSeconds:       defaultPeriodSeconds,
		FailureThreshold:    defaultFailureThreshold,
		TimeoutSeconds:      defaultTimeoutSeconds,
	}
)

// CertConfigPathMap indicates standard OS specific paths for ssl keys/certificates.
// Where Go looks for certs: https://golang.org/src/crypto/x509/root_linux.go
// Where OCP mounts proxy certs on RHCOS nodes:
// https://access.redhat.com/documentation/en-us/openshift_container_platform/4.3/html/authentication/ocp-certificates#proxy-certificates_ocp-certificates
//
//nolint:lll
var CertConfigPathMap = map[string]string{
	"ubuntu": "/usr/local/share/ca-certificates",
	"rhcos":  "/etc/pki/ca-trust/extracted/pem",
	"rhel":   "/etc/pki/ca-trust/extracted/pem",
	"rocky":  "/etc/pki/ca-trust/extracted/pem", // Rocky Linux is not officially supported.
	// NOTE: DOCA OFED driver containers are not available for Rocky Linux.
	"sles": "/etc/ssl",
}

// RepoConfigPathMap indicates standard OS specific paths for repository configuration files
var RepoConfigPathMap = map[string]string{
	"ubuntu": "/etc/apt/sources.list.d",
	"rhcos":  "/etc/yum.repos.d",
	"rhel":   "/etc/yum.repos.d",
	"rocky":  "/etc/yum.repos.d", // Rocky Linux is not officially supported.
	// NOTE: DOCA OFED driver containers are not available for Rocky Linux.
	"sles": "/etc/zypp/repos.d",
}

// MountPathToVolumeSource maps a container mount path to a VolumeSource
type MountPathToVolumeSource map[string]v1.VolumeSource

// SubscriptionPathMap contains information on OS-specific paths
// that provide entitlements/subscription details on the host.
// These are used to enable Driver Container's access to packages controlled by
// the distro through their subscription and support program.
var SubscriptionPathMap = map[string]MountPathToVolumeSource{
	"rhel": {
		"/run/secrets/etc-pki-entitlement": v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/pki/entitlement",
				Type: newHostPathType(v1.HostPathDirectory),
			},
		},
		"/run/secrets/redhat.repo": v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/yum.repos.d/redhat.repo",
				Type: newHostPathType(v1.HostPathFile),
			},
		},
		"/run/secrets/rhsm": v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/rhsm",
				Type: newHostPathType(v1.HostPathDirectory),
			},
		},
	},
	"sles": {
		"/etc/zypp/credentials.d": v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/zypp/credentials.d",
				Type: newHostPathType(v1.HostPathDirectory),
			},
		},
		"/etc/SUSEConnect": v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: "/etc/SUSEConnect",
				Type: newHostPathType(v1.HostPathFileOrCreate),
			},
		},
	},
}

func newHostPathType(pathType v1.HostPathType) *v1.HostPathType {
	hostPathType := new(v1.HostPathType)
	*hostPathType = pathType
	return hostPathType
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
	k8sAPIClient client.Client, manifestDir string) (State, ManifestRenderer, error) {
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
			renderer:    renderer,
		}}
	return state, state, nil
}

type stateOFED struct {
	stateSkel

	// staleRequeue is the delay until the soonest pending stale-DaemonSet deadline found by the
	// last Sync, or 0 when nothing is pending. Like dsOwner it is per-Sync scratch state and
	// relies on a state never being synced concurrently with itself.
	staleRequeue time.Duration
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
	ModuleDepCheckModules  string
}

type ofedRuntimeSpec struct {
	runtimeSpec
	CPUArch             string
	OSName              string
	OSVer               string
	Kernel              string
	KernelHash          string
	MOFEDImageName      string
	InitContainerConfig initContainerConfig
	// is true if cluster type is Openshift
	IsOpenshift        bool
	ContainerResources ContainerResourcesMap
	UseDtk             bool
	DtkImageName       string
	RhcosVersion       string
}

type ofedManifestRenderData struct {
	CrSpec                 *mellanoxv1alpha1.OFEDDriverSpec
	Tolerations            []v1.Toleration
	NodeAffinity           *v1.NodeAffinity
	NodeSelector           map[string]string
	DSOwner                string
	DaemonSetNameSuffix    string
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
	cr, ok := customResource.(mellanoxv1alpha1.NicPolicyCR)
	if !ok {
		return SyncStateError, fmt.Errorf("unsupported CR type: %T", customResource)
	}
	s.dsOwner = dsOwnerValue(cr)
	s.staleRequeue = 0
	reqLogger.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.GetName(), "Namespace:", cr.GetNamespace())

	if cr.GetOFEDDriverSpec() == nil {
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
		spec := cr.GetOFEDDriverSpec()
		env, certConfig, err := s.handleOpenshiftClusterWideProxyConfig(ctx, cr, spec.Env, spec.CertConfig)
		if err != nil {
			return SyncStateNotReady, errors.Wrap(err, "failed to handle Openshift cluster-wide proxy settings")
		}
		spec.Env = env
		spec.CertConfig = certConfig
	}

	objs, err := s.GetManifestObjects(ctx, cr, infoCatalog, log.FromContext(ctx))

	if err != nil {
		return SyncStateError, errors.Wrap(err, "failed to create k8s objects from manifest")
	}

	// Create objects if they dont exist, Update objects if they do exist
	err = s.createOrUpdateObjs(ctx, func(obj *unstructured.Unstructured) error {
		if err := controllerutil.SetControllerReference(cr, obj, s.client.Scheme()); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}
		return nil
	}, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create/update objects")
	}
	retention, err := s.planStaleRetention(ctx, cr, objs, poolNameSet(ofedNodePools(nodeInfo, cr)))
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to evaluate stale OFED DaemonSets")
	}
	s.staleRequeue = retention.nextDeadline

	waitForStaleObjectsRemoval, err := s.handleStaleStateObjectsWithRetention(ctx, objs, retention.retain)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to handle state stale objects")
	}
	if waitForStaleObjectsRemoval {
		return SyncStateNotReady, nil
	}

	if len(objs) == 0 {
		// GetManifestObjects returned no objects, this means that no objects need to be applied to the cluster
		// as (most likely) no Mellanox hardware is found (No mellanox labels where found).
		// Return SyncStateNotReady so we retry the Sync.
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

// RequeueAfter implements RequeueProvider. A deferred DaemonSet cleanup is the one thing this
// state waits on that produces no cluster event, so it has to ask to be synced again.
func (s *stateOFED) RequeueAfter() time.Duration {
	return s.staleRequeue
}

// ofedNodePools discovers the node pools the OFED driver should be deployed to.
//
// Nodes are filtered against everything the driver pod tolerates at runtime, not just the
// tolerations the CR asks for: a node excluded here loses its driver, so the filter must not be
// stricter than scheduling actually is.
func ofedNodePools(nodeInfo nodeinfo.Provider, cr mellanoxv1alpha1.NicPolicyCR) []nodeinfo.NodePool {
	labelFilter := nodeinfo.NewNodeLabelFilterBuilder().WithLabel(nodeinfo.NodeLabelMlnxNIC, "true").Build()
	taintFilter := nodeinfo.NewNodeTaintFilterBuilder().
		WithTolerations(schedulableNodeTolerations(cr.GetTolerations())).Build()
	return nodeInfo.GetNodePools(labelFilter, taintFilter)
}

// poolNameSet indexes node pools by name for membership tests.
func poolNameSet(nodePools []nodeinfo.NodePool) map[string]struct{} {
	names := make(map[string]struct{}, len(nodePools))
	for i := range nodePools {
		names[nodePools[i].Name] = struct{}{}
	}
	return names
}

// ofedPoolNameFromNodeSelector recovers the node pool a rendered OFED DaemonSet belongs to.
// Its nodeSelector pins the three NFD labels that define a pool, so the pool identity is already
// on the object. The DaemonSet name encodes the pool too, but only as a hash.
func ofedPoolNameFromNodeSelector(selector map[string]string) (string, bool) {
	osName, hasOSName := selector[nodeinfo.NodeLabelOSName]
	osVersion, hasOSVersion := selector[nodeinfo.NodeLabelOSVer]
	kernel, hasKernel := selector[nodeinfo.NodeLabelKernelVerFull]
	if !hasOSName || !hasOSVersion || !hasKernel {
		return "", false
	}
	return nodeinfo.PoolName(osName, osVersion, kernel), true
}

// ofedSelectorRetargeted reports whether a rendered OFED DaemonSet was built for nodes the policy
// no longer selects.
//
// The manifest renders the policy's nodeSelector over the NFD labels that pin a DaemonSet to one
// node pool, so a rendered selector always covers the policy's own entries. Asking whether it
// still covers them keeps the two sources apart without naming the manifest's labels. Subtracting
// a fixed set of label keys instead would be wrong, because a policy may select on those very
// labels — targeting one OS version is an ordinary thing to write — and its entries would then be
// mistaken for the manifest's and read back as an empty selector.
//
// Widening a selector is not retargeting and is deliberately not reported: every node matched
// before is matched still, so no node was taken away from this DaemonSet on purpose.
func ofedSelectorRetargeted(rendered, policy map[string]string) bool {
	for key, value := range policy {
		if renderedValue, ok := rendered[key]; !ok || renderedValue != value {
			return true
		}
	}
	return false
}

// staleOFEDRetention is one reconcile's decision about which objects of the OFED state the
// generic stale cleanup may delete.
type staleOFEDRetention struct {
	// protected holds the undesired DaemonSets that are kept because their node pool has no
	// eligible nodes right now.
	protected map[types.NamespacedName]struct{}
	// nextDeadline is the shortest remaining grace period across the protected DaemonSets, or 0
	// when none are protected.
	nextDeadline time.Duration
}

// retain implements retainStaleFunc.
func (r *staleOFEDRetention) retain(obj *unstructured.Unstructured) bool {
	if obj.GetKind() == "DaemonSet" {
		_, protected := r.protected[objectKey(obj)]
		return protected
	}
	// The ServiceAccount, RBAC and init container ConfigMap are shared by every pool, so they
	// only ever look stale once all pools are gone — which is when a protected DaemonSet still
	// needs them to be able to restart its pod.
	return len(r.protected) > 0
}

// protect keeps the DaemonSet out of the generic stale cleanup for the given remaining grace
// period, and tracks the soonest deadline the state has to be woken up for.
func (r *staleOFEDRetention) protect(key types.NamespacedName, remaining time.Duration) {
	r.protected[key] = struct{}{}
	if r.nextDeadline == 0 || remaining < r.nextDeadline {
		r.nextDeadline = remaining
	}
}

// objectKey identifies a state object across the retention bookkeeping.
func objectKey(obj client.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

// desiredDaemonSetKeys indexes the DaemonSets the current reconcile rendered. Every other object
// kind in the state is shared by all pools and so says nothing about an individual pool.
func desiredDaemonSetKeys(desiredObjs []*unstructured.Unstructured) map[types.NamespacedName]struct{} {
	desired := make(map[types.NamespacedName]struct{}, len(desiredObjs))
	for _, obj := range desiredObjs {
		if obj.GetKind() != "DaemonSet" {
			continue
		}
		desired[objectKey(obj)] = struct{}{}
	}
	return desired
}

// planStaleRetention decides the fate of the OFED DaemonSets the current reconcile did not
// render, and maintains the marker that bounds how long one can be kept.
func (s *stateOFED) planStaleRetention(ctx context.Context, cr mellanoxv1alpha1.NicPolicyCR,
	desiredObjs []*unstructured.Unstructured, livePools map[string]struct{}) (*staleOFEDRetention, error) {
	retention := &staleOFEDRetention{protected: map[types.NamespacedName]struct{}{}}
	desired := desiredDaemonSetKeys(desiredObjs)

	daemonSets := &appsv1.DaemonSetList{}
	if err := s.client.List(ctx, daemonSets, client.MatchingLabels(s.stateLabels())); err != nil {
		return nil, errors.Wrap(err, "failed to list OFED DaemonSets")
	}

	now := time.Now().UTC()
	for i := range daemonSets.Items {
		ds := &daemonSets.Items[i]

		if _, stillDesired := desired[objectKey(ds)]; stillDesired {
			// The pool came back. Cancel the pending cleanup: the desired revision is normally
			// unchanged across a pool outage, so createOrUpdateObjs left the live object alone
			// and the marker is still on it.
			if err := s.clearStaleSince(ctx, ds); err != nil {
				return nil, err
			}
			continue
		}

		remaining, err := s.deferralRemaining(ctx, cr, ds, livePools, now)
		if err != nil {
			return nil, err
		}
		if remaining > 0 {
			retention.protect(objectKey(ds), remaining)
		}
	}
	return retention, nil
}

// deferralRemaining reports how much longer an undesired OFED DaemonSet has to be kept, and 0
// when the generic stale cleanup may delete it now.
//
// An undesired DaemonSet has two unrelated causes that the object alone cannot tell apart: it is
// genuinely unwanted, or pool discovery found no eligible nodes for it. Only the second is
// recoverable, and it is recognized by asking whether the DaemonSet's own pool is still live —
// never by looking at node conditions or pod readiness, which say nothing about intent either.
//
// Intent the policy states outright needs no such inference and is never deferred: a DaemonSet
// rendered for a nodeSelector the policy has since changed was retargeted away on purpose.
func (s *stateOFED) deferralRemaining(ctx context.Context, cr mellanoxv1alpha1.NicPolicyCR,
	ds *appsv1.DaemonSet, livePools map[string]struct{}, now time.Time) (time.Duration, error) {
	reqLogger := log.FromContext(ctx)

	poolName, ok := ofedPoolNameFromNodeSelector(ds.Spec.Template.Spec.NodeSelector)
	if !ok {
		reqLogger.V(consts.LogLevelWarning).Info(
			"undesired OFED DaemonSet has no pool in its nodeSelector, leaving it to stale cleanup",
			"DaemonSet", objectKey(ds).String())
		return 0, nil
	}
	// A policy that no longer selects the nodes this DaemonSet was rendered for was retargeted
	// on purpose, and unlike an empty pool that is an unambiguous statement of intent. Deferring
	// it would hold the old driver pod on a node for the whole grace period, and the one-driver-
	// per-node anti-affinity would keep any policy that took the node over from starting there.
	if ofedSelectorRetargeted(ds.Spec.Template.Spec.NodeSelector, cr.GetNodeSelector()) {
		reqLogger.V(consts.LogLevelInfo).Info(
			"policy no longer selects the nodes of an undesired OFED DaemonSet, leaving it to stale cleanup",
			"DaemonSet", objectKey(ds).String(), "Pool", poolName,
			"RenderedNodeSelector", ds.Spec.Template.Spec.NodeSelector,
			"PolicyNodeSelector", cr.GetNodeSelector())
		return 0, nil
	}
	if _, live := livePools[poolName]; live {
		// The pool has eligible nodes and still does not want this DaemonSet, so something
		// changed on purpose.
		return 0, nil
	}

	staleSince, err := s.markStaleSince(ctx, ds, now)
	if err != nil {
		return 0, err
	}
	remaining := staleOFEDGracePeriod - now.Sub(staleSince)
	if remaining <= 0 {
		reqLogger.V(consts.LogLevelInfo).Info(
			"node pool of an OFED DaemonSet stayed absent for the whole grace period, reaping it",
			"DaemonSet", objectKey(ds).String(), "Pool", poolName, "StaleSince", staleSince)
		return 0, nil
	}

	reqLogger.V(consts.LogLevelInfo).Info(
		"node pool of an undesired OFED DaemonSet has no eligible nodes, deferring cleanup",
		"DaemonSet", objectKey(ds).String(), "Pool", poolName, "StaleSince", staleSince, "Remaining", remaining)
	return remaining, nil
}

// markStaleSince returns the time the DaemonSet was first seen stale, stamping it on the object
// when this is the first observation. The marker is stored on the object rather than in memory so
// an operator restart or a leader-election handover continues the same grace period instead of
// restarting it — which would leave a genuinely retired pool's DaemonSet behind forever.
//
// A marker is used in preference to starting a Kubernetes deletion with a finalizer because a
// deletion cannot be abandoned once deletionTimestamp is set, and recovery must be able to
// abandon it.
func (s *stateOFED) markStaleSince(
	ctx context.Context, ds *appsv1.DaemonSet, now time.Time) (time.Time, error) {
	if raw, ok := ds.GetAnnotations()[consts.StaleSinceAnnotation]; ok {
		if since, err := time.Parse(time.RFC3339, raw); err == nil {
			return since.UTC(), nil
		}
		log.FromContext(ctx).V(consts.LogLevelWarning).Info(
			"OFED DaemonSet has an unparsable stale-since annotation, restarting its grace period",
			"DaemonSet", ds.GetName(), "Value", raw)
	}

	patched := ds.DeepCopy()
	annotations := patched.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[consts.StaleSinceAnnotation] = now.Format(time.RFC3339)
	patched.SetAnnotations(annotations)
	if err := s.client.Patch(ctx, patched, client.MergeFrom(ds)); err != nil {
		return now, errors.Wrapf(err, "failed to mark DaemonSet %s/%s stale", ds.GetNamespace(), ds.GetName())
	}
	return now, nil
}

// clearStaleSince drops the stale marker from a DaemonSet that is desired again.
func (s *stateOFED) clearStaleSince(ctx context.Context, ds *appsv1.DaemonSet) error {
	if _, ok := ds.GetAnnotations()[consts.StaleSinceAnnotation]; !ok {
		return nil
	}
	patched := ds.DeepCopy()
	annotations := patched.GetAnnotations()
	delete(annotations, consts.StaleSinceAnnotation)
	patched.SetAnnotations(annotations)
	if err := s.client.Patch(ctx, patched, client.MergeFrom(ds)); err != nil {
		return errors.Wrapf(err, "failed to clear the stale marker on DaemonSet %s/%s",
			ds.GetNamespace(), ds.GetName())
	}
	log.FromContext(ctx).V(consts.LogLevelInfo).Info(
		"node pool of an OFED DaemonSet has eligible nodes again, canceled its deferred cleanup",
		"DaemonSet", ds.GetName())
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

// operatorDriverPodTolerations are the tolerations the operator always renders into the driver
// pod, on top of the ones the CR asks for.
//
// The NoSchedule halves of the not-ready and unreachable taints are here because the driver is
// part of what makes a node ready: its own openibd restart can flip the node NotReady for a
// moment. The DaemonSet controller tolerates only the NoExecute halves, which keeps an already
// running pod from being evicted but would leave a restarting one unschedulable exactly when it
// is needed.
var operatorDriverPodTolerations = []v1.Toleration{
	{
		// The taint the GPU Operator puts on GPU nodes, which still need the driver.
		Key:      "nvidia.com/gpu",
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
	{
		Key:      v1.TaintNodeNotReady,
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
	{
		Key:      v1.TaintNodeUnreachable,
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
}

// daemonSetControllerTolerations are injected into every DaemonSet pod by the DaemonSet
// controller at admission. They are deliberately absent from the pod template — the pod carries
// them at runtime either way — but node eligibility has to account for them.
// See: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/#taints-and-tolerations
var daemonSetControllerTolerations = []v1.Toleration{
	{
		Key:      v1.TaintNodeNotReady,
		Effect:   v1.TaintEffectNoExecute,
		Operator: v1.TolerationOpExists,
	},
	{
		Key:      v1.TaintNodeUnreachable,
		Effect:   v1.TaintEffectNoExecute,
		Operator: v1.TolerationOpExists,
	},
	{
		Key:      v1.TaintNodeDiskPressure,
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
	{
		Key:      v1.TaintNodeMemoryPressure,
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
	{
		Key:      v1.TaintNodePIDPressure,
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
	{
		Key:      v1.TaintNodeUnschedulable,
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
	{
		// Added only for DaemonSet pods that request host networking, which the driver does.
		Key:      v1.TaintNodeNetworkUnavailable,
		Effect:   v1.TaintEffectNoSchedule,
		Operator: v1.TolerationOpExists,
	},
}

// driverPodTolerations returns the tolerations rendered into the driver pod template.
func driverPodTolerations(crTolerations []v1.Toleration) []v1.Toleration {
	return mergeTolerations(crTolerations, operatorDriverPodTolerations)
}

// schedulableNodeTolerations returns every toleration the driver pod has in effect once it is
// running: what the pod template declares plus what the DaemonSet controller injects. A node
// belongs to a pool only if this set covers its taints.
//
// It is derived from driverPodTolerations rather than listed on its own so that the filter
// cannot come to claim a node the pod would never be scheduled onto.
func schedulableNodeTolerations(crTolerations []v1.Toleration) []v1.Toleration {
	return mergeTolerations(driverPodTolerations(crTolerations), daemonSetControllerTolerations)
}

// mergeTolerations returns base followed by every addition base does not already cover. The
// result is a fresh slice: base belongs to the CR, and appending onto it in place would write
// into the CR's backing array whenever it has spare capacity.
func mergeTolerations(base, additions []v1.Toleration) []v1.Toleration {
	merged := make([]v1.Toleration, len(base), len(base)+len(additions))
	copy(merged, base)
	for _, addition := range additions {
		if !coversToleration(merged, addition) {
			merged = append(merged, addition)
		}
	}
	return merged
}

// coversToleration reports whether the list already tolerates what the given toleration does.
// Value is not compared because every toleration added here uses the Exists operator, for which
// Kubernetes ignores it.
func coversToleration(tolerations []v1.Toleration, toleration v1.Toleration) bool {
	for _, existing := range tolerations {
		if existing.Key == toleration.Key &&
			existing.Effect == toleration.Effect &&
			existing.Operator == toleration.Operator {
			return true
		}
	}
	return false
}

//nolint:funlen
func (s *stateOFED) GetManifestObjects(
	ctx context.Context, cr mellanoxv1alpha1.NicPolicyCR,
	catalog InfoCatalog, reqLogger logr.Logger) ([]*unstructured.Unstructured, error) {
	if cr == nil || cr.GetOFEDDriverSpec() == nil {
		return nil, errors.New("failed to render objects: state spec is nil")
	}

	cr.GetOFEDDriverSpec().ImageSpec.ApplyGlobalConfig(cr.GetGlobalConfig())
	if err := cr.GetOFEDDriverSpec().ImageSpec.ValidateRequiredFields(); err != nil {
		return nil, errors.Wrap(err, "failed to validate ofedDriver image spec")
	}

	nodeInfo, clusterInfo, docaProvider, err := getProviders(catalog)
	if err != nil {
		return nil, err
	}

	nodePools := ofedNodePools(nodeInfo, cr)

	if len(nodePools) == 0 {
		reqLogger.V(consts.LogLevelInfo).Info("No nodes with Mellanox NICs and matching tolerations found")
		return []*unstructured.Unstructured{}, nil
	}

	setProbesDefaults(cr)
	// Update MOFED Env variables with defaults for the cluster
	cr.GetOFEDDriverSpec().Env = s.mergeWithDefaultEnvs(cr.GetOFEDDriverSpec().Env)

	objs := make([]*unstructured.Unstructured, 0)
	renderedObjsMap := stateObjects{}
	useDtk := clusterInfo.IsOpenshift() && config.FromEnv().State.OFEDState.UseDTK

	for _, np := range nodePools {
		nodePool := np
		// render objects, passing only user-defined tolerations to the renderer
		// as the GPU toleration is expected to be in the template.
		renderedObjs, err := renderObjects(ctx, &nodePool, useDtk, s, cr, reqLogger, clusterInfo, docaProvider)
		if err != nil {
			return nil, errors.Wrap(err, "failed to render objects")
		}
		for _, o := range renderedObjs {
			if !renderedObjsMap.Exist(o.GroupVersionKind(), types.NamespacedName{
				Name:      o.GetName(),
				Namespace: o.GetNamespace()}) {
				renderedObjsMap.Add(o.GroupVersionKind(), types.NamespacedName{Name: o.GetName(), Namespace: o.GetNamespace()})
				objs = append(objs, o)
			}
		}
	}
	reqLogger.V(consts.LogLevelDebug).Info("Rendered", "objects:", objs)
	return objs, nil
}

func renderObjects(ctx context.Context, nodePool *nodeinfo.NodePool, useDtk bool, s *stateOFED,
	cr mellanoxv1alpha1.NicPolicyCR, reqLogger logr.Logger,
	clusterInfo clustertype.Provider, docaProvider docadriverimages.Provider) ([]*unstructured.Unstructured, error) {
	isSha256 := strings.HasPrefix(cr.GetOFEDDriverSpec().Version, "sha256")
	if isSha256 {
		reqLogger.V(consts.LogLevelInfo).Info("DOCA OFED Driver is using sha256 tag",
			"tag", cr.GetOFEDDriverSpec().Version)
	}
	precompiledTag := fmt.Sprintf(precompiledTagFormat, cr.GetOFEDDriverSpec().Version, nodePool.Kernel,
		nodePool.OsName, nodePool.OsVersion, nodePool.Arch)
	precompiledExists, tagErr := docaProvider.TagExists(precompiledTag)
	reqLogger.V(consts.LogLevelDebug).Info("Precompiled tag",
		"tag", precompiledTag,
		"found", precompiledExists,
		"error", tagErr)
	if !isSha256 && !precompiledExists && cr.GetOFEDDriverSpec().ForcePrecompiled {
		if tagErr != nil {
			return nil, fmt.Errorf(
				"ForcePrecompiled is enabled, but failed to verify existence of precompiled tag: %s, %w", precompiledTag, tagErr)
		}
		return nil, fmt.Errorf("ForcePrecompiled is enabled and precompiled tag was not found: %s", precompiledTag)
	}

	if isSha256 || precompiledExists {
		useDtk = false
	}

	var dtkImageName string
	rhcosVersion := nodePool.RhcosVersion
	if useDtk {
		if rhcosVersion == "" {
			return nil, fmt.Errorf("required NFD Label missing: %s", nodeinfo.NodeLabelOSTreeVersion)
		}
		dtk, err := s.getOCPDriverToolkitImage(ctx, rhcosVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to get OpenShift DTK image : %v", err)
		}
		dtkImageName = dtk
	}

	additionalVolMounts := additionalVolumeMounts{}
	osname := nodePool.OsName

	// set any custom ssl key/certificate configuration provided
	err := s.handleCertConfig(ctx, cr, osname, &additionalVolMounts)
	if err != nil {
		return nil, err
	}

	// set any custom repo configuration provided
	err = s.handleRepoConfig(ctx, cr, osname, &additionalVolMounts)
	if err != nil {
		return nil, err
	}

	// set subscription volumes if needed
	err = s.handleSubscriptionVolumes(ctx, osname, nodePool.ContainerRuntime, &additionalVolMounts)
	if err != nil {
		return nil, err
	}

	renderData := &ofedManifestRenderData{
		CrSpec: cr.GetOFEDDriverSpec(),
		RuntimeSpec: &ofedRuntimeSpec{
			runtimeSpec:    runtimeSpec{config.FromEnv().State.NetworkOperatorResourceNamespace},
			CPUArch:        nodePool.Arch,
			OSName:         nodePool.OsName,
			OSVer:          nodePool.OsVersion,
			Kernel:         nodePool.Kernel,
			KernelHash:     getStringHash(nodePool.Kernel),
			MOFEDImageName: s.getMofedDriverImageName(cr, nodePool, precompiledExists, isSha256, reqLogger),
			InitContainerConfig: s.getInitContainerConfig(cr, reqLogger,
				config.FromEnv().State.OFEDState.InitContainerImage),
			IsOpenshift:        clusterInfo.IsOpenshift(),
			ContainerResources: createContainerResourcesMap(cr.GetOFEDDriverSpec().ContainerResources),
			UseDtk:             useDtk,
			DtkImageName:       dtkImageName,
			RhcosVersion:       rhcosVersion,
		},
		Tolerations:            driverPodTolerations(cr.GetTolerations()),
		NodeAffinity:           cr.GetNodeAffinity(),
		NodeSelector:           cr.GetNodeSelector(),
		DSOwner:                dsOwnerValue(cr),
		DaemonSetNameSuffix:    hashedNameSuffix(cr),
		AdditionalVolumeMounts: additionalVolMounts,
	}

	reqLogger.V(consts.LogLevelDebug).Info("Rendering objects", "data:", renderData)
	renderedObjs, err := s.renderer.RenderObjects(&render.TemplatingData{Data: renderData})
	return renderedObjs, err
}

func getProviders(catalog InfoCatalog) (nodeinfo.Provider, clustertype.Provider, docadriverimages.Provider, error) {
	nodeInfo := catalog.GetNodeInfoProvider()
	if nodeInfo == nil {
		return nil, nil, nil, errors.New("nodeInfo provider required")
	}
	clusterInfo := catalog.GetClusterTypeProvider()
	if clusterInfo == nil {
		return nil, nil, nil, errors.New("clusterInfo provider required")
	}
	docaProvider := catalog.GetDocaDriverImageProvider()
	if docaProvider == nil {
		return nil, nil, nil, errors.New("docaProvider provider required")
	}
	return nodeInfo, clusterInfo, docaProvider, nil
}

// prepare configuration for the init container,
// the init container will be disabled if the image is empty
func (s *stateOFED) getInitContainerConfig(
	cr mellanoxv1alpha1.NicPolicyCR, reqLogger logr.Logger, image string) initContainerConfig {
	var initContCfg initContainerConfig

	ofedDriverSpec := cr.GetOFEDDriverSpec()

	safeLoadEnable := ofedDriverSpec.OfedUpgradePolicy != nil &&
		ofedDriverSpec.OfedUpgradePolicy.AutoUpgrade &&
		ofedDriverSpec.OfedUpgradePolicy.SafeLoad

	if image != "" {
		initContCfg = initContainerConfig{
			InitContainerEnable:    true,
			InitContainerImageName: image,
			SafeLoadEnable:         safeLoadEnable,
			SafeLoadAnnotation:     upgrade.GetUpgradeDriverWaitForSafeLoadAnnotationKey(),
			ModuleDepCheckModules: `"mlx5_core", "mlx5_ib", "ib_umad",` +
				` "ib_uverbs", "ib_ipoib", "rdma_cm",` +
				` "rdma_ucm", "ib_core", "ib_cm"`,
		}
	}

	if safeLoadEnable && !initContCfg.InitContainerEnable {
		reqLogger.Error(nil, "safe driver loading feature is enabled, but init container is "+
			"disabled. It is required to enable init container to use safe driver loading feature.")
	}
	return initContCfg
}

// getMofedDriverImageName generates MOFED driver image name based on the driver version specified in CR
func (s *stateOFED) getMofedDriverImageName(cr mellanoxv1alpha1.NicPolicyCR,
	pool *nodeinfo.NodePool, precompiledExists bool, isSha256 bool, reqLogger logr.Logger) string {
	reqLogger.V(consts.LogLevelDebug).Info("Generating ofed driver image name for version: %v",
		"version", cr.GetOFEDDriverSpec().Version)

	if isSha256 {
		return fmt.Sprintf(sha256ImageFormat,
			cr.GetOFEDDriverSpec().Repository, cr.GetOFEDDriverSpec().Image,
			cr.GetOFEDDriverSpec().Version)
	}
	if precompiledExists {
		return fmt.Sprintf(precompiledImageFormat,
			cr.GetOFEDDriverSpec().Repository, cr.GetOFEDDriverSpec().Image,
			cr.GetOFEDDriverSpec().Version, pool.Kernel,
			pool.OsName, pool.OsVersion, pool.Arch)
	}
	return fmt.Sprintf(mofedImageFormat,
		cr.GetOFEDDriverSpec().Repository, cr.GetOFEDDriverSpec().Image,
		cr.GetOFEDDriverSpec().Version,
		pool.OsName,
		pool.OsVersion,
		pool.Arch)
}

// mergeWithDefaultEnvs returns env variables provided in currentEnvs merged with default
// env variables for MOFED container.
func (s *stateOFED) mergeWithDefaultEnvs(currentEnvs []v1.EnvVar) []v1.EnvVar {
	envs := currentEnvs

	// CREATE_IFNAMES_UDEV: should be set to true if not provided.
	if envVarsWithGet(currentEnvs).Get(envVarCreateIfNamesUdev) == nil {
		envs = append(envs, v1.EnvVar{Name: envVarCreateIfNamesUdev, Value: "true"})
	}
	// NVIDIA_NIC_DRIVERS_INVENTORY_PATH: should be set to true if not provided.
	if envVarsWithGet(currentEnvs).Get(envVarDriversInventoryPath) == nil {
		envs = append(envs, v1.EnvVar{Name: envVarDriversInventoryPath, Value: defaultDriversInventoryPath})
	}

	return envs
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
func setProbesDefaults(cr mellanoxv1alpha1.NicPolicyCR) {
	if cr.GetOFEDDriverSpec().StartupProbe == nil {
		probe := startupProbeSpec
		cr.GetOFEDDriverSpec().StartupProbe = &probe
	}
	sanitizeProbeSpec(cr.GetOFEDDriverSpec().StartupProbe, startupProbeSpec)

	if cr.GetOFEDDriverSpec().LivenessProbe == nil {
		probe := defaultProbeSpec
		cr.GetOFEDDriverSpec().LivenessProbe = &probe
	}
	sanitizeProbeSpec(cr.GetOFEDDriverSpec().LivenessProbe, defaultProbeSpec)

	if cr.GetOFEDDriverSpec().ReadinessProbe == nil {
		probe := defaultProbeSpec
		cr.GetOFEDDriverSpec().ReadinessProbe = &probe
	}
	sanitizeProbeSpec(cr.GetOFEDDriverSpec().ReadinessProbe, defaultProbeSpec)
}

// sanitizeProbeSpec checks and adjusts probe spec values to meet Kubernetes requirements
func sanitizeProbeSpec(probeSpec *mellanoxv1alpha1.PodProbeSpec, defaultProbeSpec mellanoxv1alpha1.PodProbeSpec) {
	if probeSpec.InitialDelaySeconds < 1 {
		probeSpec.InitialDelaySeconds = defaultProbeSpec.InitialDelaySeconds
	}
	if probeSpec.PeriodSeconds < 1 {
		probeSpec.PeriodSeconds = defaultProbeSpec.PeriodSeconds
	}
	if probeSpec.FailureThreshold < 1 {
		probeSpec.FailureThreshold = defaultProbeSpec.FailureThreshold
	}
	if probeSpec.TimeoutSeconds < 1 {
		probeSpec.TimeoutSeconds = defaultProbeSpec.TimeoutSeconds
	}
}

// handleCertConfig handles additional mounts required for Certificates if specified
func (s *stateOFED) handleCertConfig(
	ctx context.Context, cr mellanoxv1alpha1.NicPolicyCR, osname string, mounts *additionalVolumeMounts) error {
	if cr.GetOFEDDriverSpec().CertConfig != nil && cr.GetOFEDDriverSpec().CertConfig.Name != "" {
		destinationDir, err := getCertConfigPath(osname)
		if err != nil {
			return fmt.Errorf("failed to get destination directory for custom TLS certificates config: %v", err)
		}

		err = s.handleAdditionalMounts(ctx, mounts, cr.GetOFEDDriverSpec().CertConfig.Name, destinationDir)
		if err != nil {
			return fmt.Errorf("failed to mount volumes for custom TLS certificates: %v", err)
		}
	}
	return nil
}

// handleSubscriptionVolumes handles additional mounts required for subscriptions
func (s *stateOFED) handleSubscriptionVolumes(
	ctx context.Context, osname string, runtime string, mounts *additionalVolumeMounts) error {
	reqLogger := log.FromContext(ctx)
	if (osname == "rhel" && runtime != nodeinfo.CRIO) || osname == "sles" {
		reqLogger.V(consts.LogLevelDebug).Info("Setting subscription mounts for OS:%s, runtime:%s", osname, runtime)
		pathToVolumeSource, ok := SubscriptionPathMap[osname]
		if !ok {
			return fmt.Errorf("failed to find subscription volumes definition for os: %v", osname)
		}
		// sort host path volumes to ensure ordering is preserved when adding to pod spec
		mountPaths := make([]string, 0, len(pathToVolumeSource))
		for k := range pathToVolumeSource {
			mountPaths = append(mountPaths, k)
		}
		sort.Strings(mountPaths)

		for num, mountPath := range mountPaths {
			volMountSubscriptionName := fmt.Sprintf("subscription-config-%d", num)

			volMountSubscription := v1.VolumeMount{
				Name:      volMountSubscriptionName,
				MountPath: mountPath,
				ReadOnly:  true,
			}
			mounts.VolumeMounts = append(mounts.VolumeMounts, volMountSubscription)

			subscriptionVol := v1.Volume{Name: volMountSubscriptionName, VolumeSource: pathToVolumeSource[mountPath]}
			mounts.Volumes = append(mounts.Volumes, subscriptionVol)
		}
	}
	return nil
}

// handleRepoConfig handles additional mounts required for custom repo if specified
func (s *stateOFED) handleRepoConfig(
	ctx context.Context, cr mellanoxv1alpha1.NicPolicyCR, osname string, mounts *additionalVolumeMounts) error {
	if cr.GetOFEDDriverSpec().RepoConfig != nil && cr.GetOFEDDriverSpec().RepoConfig.Name != "" {
		destinationDir, err := getRepoConfigPath(osname)
		if err != nil {
			return fmt.Errorf("failed to get destination directory for custom repo config: %v", err)
		}

		err = s.handleAdditionalMounts(ctx, mounts, cr.GetOFEDDriverSpec().RepoConfig.Name, destinationDir)
		if err != nil {
			return fmt.Errorf("failed to mount volumes for custom repositories configuration: %v", err)
		}
	}
	return nil
}

// getOCPDriverToolkitImage gets the DTK ImageStream and return the DTK image according to OSTREE version
func (s *stateOFED) getOCPDriverToolkitImage(ctx context.Context, ostreeVersion string) (string, error) {
	reqLogger := log.FromContext(ctx)
	dtkImageStream := &apiimagev1.ImageStream{}
	name := "driver-toolkit"
	namespace := "openshift"
	err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, dtkImageStream)
	if err != nil {
		reqLogger.Error(err, "Couldn't get the driver-toolkit imagestream")
		return "", err
	}
	rhcosDriverToolkitImages := make(map[string]string)
	reqLogger.Info("ocpDriverToolkitImages: driver-toolkit imagestream found")
	for _, tag := range dtkImageStream.Spec.Tags {
		rhcosDriverToolkitImages[tag.Name] = tag.From.Name
	}

	image, ok := rhcosDriverToolkitImages[ostreeVersion]
	if !ok {
		return "", fmt.Errorf("failed to find DTK image for RHCOS version: %v", ostreeVersion)
	}
	return image, nil
}
