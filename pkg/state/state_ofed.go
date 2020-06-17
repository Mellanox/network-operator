package state

import (
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/mellanox-network-operator/pkg/apis/mellanox/v1alpha1"
	"github.com/Mellanox/mellanox-network-operator/pkg/consts"
	"github.com/Mellanox/mellanox-network-operator/pkg/nodeinfo"
	"github.com/Mellanox/mellanox-network-operator/pkg/render"
	"github.com/Mellanox/mellanox-network-operator/pkg/utils"
)

const stateOFEDName = "state-OFED"
const stateOFEDDescription = "OFED driver deployed in the cluster"

// NewStateOFED creates a new OFED driver state
func NewStateOFED(k8sAPIClient client.Client, scheme *runtime.Scheme, manifestDir string) (State, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	return &stateOFED{
		stateSkel: stateSkel{
			name:        stateOFEDName,
			description: stateOFEDDescription,
			client:      k8sAPIClient,
			scheme:      scheme,
			renderer:    renderer,
		}}, nil
}

type stateOFED struct {
	stateSkel
}

type runtimeSpec struct {
	CPUArch       string
	OSNameFull    string
	KernelVerFull string
	Namespace     string
}

type ofedManifestRenderData struct {
	CrSpec      *mellanoxv1alpha1.OFEDDriverSpec
	RuntimeSpec *runtimeSpec
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
func (s *stateOFED) Sync(customResource interface{}, serviceCatalog ServiceCatalog) (SyncState, error) {
	cr := customResource.(*mellanoxv1alpha1.NicClusterPolicy)
	log.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.Name, "Namespace:", cr.Namespace)

	if cr.Spec.OFEDDriver == nil {
		// Either this state was not required to run or an update occurred and we need to remove
		// the resources that where created.
		// TODO: Support the latter case
		log.V(consts.LogLevelInfo).Info("OFED driver spec in CR is nil, no action required")
		return SyncStateIgnore, nil
	}
	// Fill ManifestRenderData and render objects
	nodeInfo := serviceCatalog.GetNodeInfoProvider()
	if nodeInfo == nil {
		return SyncStateError, errors.New("unexpected state, catalog does not provide node information")
	}

	objs, err := s.getManifestObjects(cr, nodeInfo)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create k8s objects from manifest")
	}
	if len(objs) == 0 {
		// getManifestObjects returned no objects, this means that no objects need to be applied to the cluster
		// as (most likely) no Mellanox hardware is found (No mellanox labels where found).
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
	// Check objects status
	syncState, err := s.getSyncState(objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to get sync state")
	}
	return syncState, nil
}

// Get a map of source kinds that should be watched for the state keyed by the source kind name
func (s *stateOFED) GetWatchSources() map[string]*source.Kind {
	wr := make(map[string]*source.Kind)
	wr["DaemonSet"] = &source.Kind{Type: &appsv1.DaemonSet{}}
	return wr
}

func (s *stateOFED) getManifestObjects(
	cr *mellanoxv1alpha1.NicClusterPolicy,
	nodeInfo nodeinfo.Provider) ([]*unstructured.Unstructured, error) {
	attrs := nodeInfo.GetNodesAttributes(
		nodeinfo.NewNodeLabelFilterBuilder().WithLabel(nodeinfo.NodeLabelMlnxNIC, "true").Build())
	if len(attrs) == 0 {
		log.V(consts.LogLevelInfo).Info("No nodes with Mellanox NICs where found in the cluster.")
		return []*unstructured.Unstructured{}, nil
	}

	// TODO: Render daemonset multiple times according to CPUXOSXKernel matrix (ATM assume all nodes are the same)
	if err := s.checkAttributesExist(attrs[0],
		nodeinfo.AttrTypeCPUArch, nodeinfo.AttrTypeOS, nodeinfo.AttrTypeKernel); err != nil {
		return nil, err
	}

	renderData := &ofedManifestRenderData{
		CrSpec: cr.Spec.OFEDDriver,
		RuntimeSpec: &runtimeSpec{
			Namespace:     cr.Namespace,
			CPUArch:       attrs[0].Attributes[nodeinfo.AttrTypeCPUArch],
			OSNameFull:    attrs[0].Attributes[nodeinfo.AttrTypeOS],
			KernelVerFull: attrs[0].Attributes[nodeinfo.AttrTypeKernel],
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
