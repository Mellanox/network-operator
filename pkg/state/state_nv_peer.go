package state

import (
	"strconv"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

const stateNVPeerName = "state-NV-Peer"
const stateNVPeerDescription = "Nvidia Peer Memory driver deployed in the cluster"

// starting from this GPU driver version nvPeerMem driver should not deploy
const maxCudaVersionMajor = 465

//TODO: Refine a base struct that implements a driver container as this is pretty much identical to OFED state

// NewStateNVPeer creates a new NVPeer driver state
func NewStateNVPeer(k8sAPIClient client.Client, scheme *runtime.Scheme, manifestDir string) (State, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	return &stateNVPeer{
		stateSkel: stateSkel{
			name:        stateNVPeerName,
			description: stateNVPeerDescription,
			client:      k8sAPIClient,
			scheme:      scheme,
			renderer:    renderer,
		}}, nil
}

type stateNVPeer struct {
	stateSkel
}

type nvPeerRuntimeSpec struct {
	runtimeSpec
	CPUArch        string
	OSName         string
	OSVer          string
	UseHostOFED    bool
	MaxCudaVersion int
}

type nvPeerManifestRenderData struct {
	CrSpec       *mellanoxv1alpha1.NVPeerDriverSpec
	NodeAffinity *v1.NodeAffinity
	RuntimeSpec  *nvPeerRuntimeSpec
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
//nolint:dupl
func (s *stateNVPeer) Sync(customResource interface{}, infoCatalog InfoCatalog) (SyncState, error) {
	cr := customResource.(*mellanoxv1alpha1.NicClusterPolicy)
	log.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.Name, "Namespace:", cr.Namespace)

	if cr.Spec.NVPeerDriver == nil {
		// Either this state was not required to run or an update occurred and we need to remove
		// the resources that where created.
		// TODO: Support the latter case
		log.V(consts.LogLevelInfo).Info("NV Peer driver spec in CR is nil, no action required")
		return SyncStateIgnore, nil
	}
	// Fill ManifestRenderData and render objects
	nodeInfo := infoCatalog.GetNodeInfoProvider()
	if nodeInfo == nil {
		return SyncStateError, errors.New("unexpected state, catalog does not provide node information")
	}

	objs, err := s.getManifestObjects(cr, nodeInfo)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create k8s objects from manifest")
	}
	if len(objs) == 0 {
		// getManifestObjects returned no objects, this means that no objects need to be applied to the cluster
		// as (most likely) no Mellanox/Nvidia hardware is found (No Mellanox and Nvidia labels where found).
		// Return SyncStateNotReady so we retry the Sync.
		return SyncStateNotReady, nil
	}

	// Create objects if they dont exist, Update objects if they do exist
	err = s.createOrUpdateObjs(func(obj *unstructured.Unstructured) error {
		if err := controllerutil.SetControllerReference(cr, obj, s.scheme); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}
		return nil
	}, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create/update objects")
	}

	// check if nvPeerMem status should be ignored
	// workaround to support upgrade from nv_peer_mem to nvidia_peermem module
	// check function doc for details
	if s.shouldIgnoreStatus(nodeInfo) {
		log.V(consts.LogLevelInfo).Info("GPU driver version with builtin nvidia_peermem module detected." +
			"NV Peer driver status ignored")
		return SyncStateIgnore, nil
	}

	// Check objects status
	syncState, err := s.getSyncState(objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to get sync state")
	}
	return syncState, nil
}

// Get a map of source kinds that should be watched for the state keyed by the source kind name
func (s *stateNVPeer) GetWatchSources() map[string]*source.Kind {
	wr := make(map[string]*source.Kind)
	wr["DaemonSet"] = &source.Kind{Type: &appsv1.DaemonSet{}}
	return wr
}

func (s *stateNVPeer) getManifestObjects(
	cr *mellanoxv1alpha1.NicClusterPolicy,
	nodeInfo nodeinfo.Provider) ([]*unstructured.Unstructured, error) {
	attrs := nodeInfo.GetNodesAttributes(
		nodeinfo.NewNodeLabelFilterBuilder().
			WithLabel(nodeinfo.NodeLabelMlnxNIC, "true").
			WithLabel(nodeinfo.NodeLabelNvGPU, "true").
			Build())
	if len(attrs) == 0 {
		log.V(consts.LogLevelInfo).Info("No nodes with Mellanox NICs and Nvidia GPUs where found in the cluster.")
		return []*unstructured.Unstructured{}, nil
	}

	// TODO: Render daemonset multiple times according to CPUXOS matrix (ATM assume all nodes are the same)
	if err := s.checkAttributesExist(attrs[0],
		nodeinfo.AttrTypeCPUArch, nodeinfo.AttrTypeOSName, nodeinfo.AttrTypeOSVer); err != nil {
		return nil, err
	}

	renderData := &nvPeerManifestRenderData{
		CrSpec:       cr.Spec.NVPeerDriver,
		NodeAffinity: cr.Spec.NodeAffinity,
		RuntimeSpec: &nvPeerRuntimeSpec{
			runtimeSpec:    runtimeSpec{config.FromEnv().State.NetworkOperatorResourceNamespace},
			CPUArch:        attrs[0].Attributes[nodeinfo.AttrTypeCPUArch],
			OSName:         attrs[0].Attributes[nodeinfo.AttrTypeOSName],
			OSVer:          attrs[0].Attributes[nodeinfo.AttrTypeOSVer],
			UseHostOFED:    cr.Spec.OFEDDriver == nil,
			MaxCudaVersion: maxCudaVersionMajor,
		},
	}
	// render objects
	log.V(consts.LogLevelDebug).Info("Rendering objects", "data:", renderData)
	objs, err := s.renderer.RenderObjects(&render.TemplatingData{Data: renderData})
	if err != nil {
		return nil, errors.Wrap(err, "failed to render objects")
	}
	log.V(consts.LogLevelDebug).Info("Rendered", "objects:", objs)
	return objs, nil
}

// shouldIgnoreStatus check if nvPeerMem DS status should be ignored
// Workaround to support switching from nv_peer_mem module which is managed by network-operator
// to nvidia_peermem module which is a part of the GPU driver starting from v465.
// We should ignore nvPeerMem status in case all nodes in a cluster have GPU driver version > 465
// if some nodes still using GPU driver < 465 or there are no nodes with GPU driver, we should continue sync attempts
// we don't remove DS, just ignore its status. This is required to be able to recovery nv-peer-mem in situations when
// GPU driver version was downgraded or when we have an environment with mixed versions of the GPU drivers
// and node with older version appear after node with a newer version.
// nvPeerMem config should be removed from NicClusterPolicy explicitly to remove DS
func (s *stateNVPeer) shouldIgnoreStatus(nodeInfo nodeinfo.Provider) bool {
	attrs := nodeInfo.GetNodesAttributes(
		nodeinfo.NewNodeLabelNoValFilterBuilderr().
			WithLabel(nodeinfo.NodeLabelCudaVersionMajor).
			Build())
	if len(attrs) == 0 {
		// no driver info available, should continue sync
		return false
	}
	for _, attr := range attrs {
		cudaVersion, err := strconv.Atoi(attr.Attributes[nodeinfo.AttrTypeCudaVersionMajor])
		if err != nil {
			// unknown cuda version, continue sync
			log.V(consts.LogLevelInfo).Info("Fail to check GPU driver version")
			return false
		}
		if cudaVersion < maxCudaVersionMajor {
			// node with old GPU driver found, continue sync
			return false
		}
	}
	return true
}
