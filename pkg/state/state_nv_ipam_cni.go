/*
2023 NVIDIA CORPORATION & AFFILIATES

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

package state //nolint:dupl

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

// NewStateNVIPAMCNI creates a new state for Multus
func NewStateNVIPAMCNI(k8sAPIClient client.Client, scheme *runtime.Scheme, manifestDir string) (State, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	return &stateNVIPAMCNI{
		stateSkel: stateSkel{
			name:        "state-nv-ipam-cni",
			description: "nv-ipam IPAM CNI deployed in the cluster",
			client:      k8sAPIClient,
			scheme:      scheme,
			renderer:    renderer,
		}}, nil
}

type stateNVIPAMCNI struct {
	stateSkel
}

type nvipamRuntimeSpec struct {
	runtimeSpec
	OSName string
}

type NVIPAMManifestRenderData struct {
	CrSpec       *mellanoxv1alpha1.NVIPAMSpec
	NodeAffinity *v1.NodeAffinity
	Tolerations  []v1.Toleration
	RuntimeSpec  *nvipamRuntimeSpec
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
//
//nolint:dupl
func (s *stateNVIPAMCNI) Sync(
	ctx context.Context, customResource interface{}, infoCatalog InfoCatalog) (SyncState, error) {
	reqLogger := log.FromContext(ctx)
	cr := customResource.(*mellanoxv1alpha1.NicClusterPolicy)
	reqLogger.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.Name, "Namespace:", cr.Namespace)

	if cr.Spec.NvIpam == nil {
		// Either this state was not required to run or an update occurred and we need to remove
		// the resources that where created.
		return s.handleStateObjectsDeletion(ctx)
	}

	nodeInfo := infoCatalog.GetNodeInfoProvider()
	if nodeInfo == nil {
		return SyncStateError, errors.New("unexpected state, catalog does not provide node information")
	}

	// Fill ManifestRenderData and render objects
	objs, err := s.getManifestObjects(cr, nodeInfo, reqLogger)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create k8s objects from manifest")
	}
	if len(objs) == 0 {
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
	// Check objects status
	syncState, err := s.getSyncState(ctx, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to get sync state")
	}
	return syncState, nil
}

// Get a map of source kinds that should be watched for the state keyed by the source kind name
func (s *stateNVIPAMCNI) GetWatchSources() map[string]*source.Kind {
	wr := make(map[string]*source.Kind)
	wr["DaemonSet"] = &source.Kind{Type: &appsv1.DaemonSet{}}
	wr["Deployment"] = &source.Kind{Type: &appsv1.Deployment{}}
	wr["ConfigMap"] = &source.Kind{Type: &v1.ConfigMap{}}
	return wr
}

func (s *stateNVIPAMCNI) getManifestObjects(
	cr *mellanoxv1alpha1.NicClusterPolicy, nodeInfo nodeinfo.Provider,
	reqLogger logr.Logger) ([]*unstructured.Unstructured, error) {
	// TODO replace this code with reliable check for cluster type (Openshift or Kubernetes)
	var osName string
	attrs := nodeInfo.GetNodesAttributes()
	if len(attrs) != 0 {
		if err := s.checkAttributesExist(attrs[0], nodeinfo.AttrTypeOSName); err == nil {
			osName = attrs[0].Attributes[nodeinfo.AttrTypeOSName]
		}
	} else {
		reqLogger.V(consts.LogLevelInfo).Info("Can't detect Operating system in the cluster, " +
			"assume that operator runs in a Kubernetes cluster")
	}

	renderData := &NVIPAMManifestRenderData{
		CrSpec:       cr.Spec.NvIpam,
		NodeAffinity: cr.Spec.NodeAffinity,
		Tolerations:  cr.Spec.Tolerations,
		RuntimeSpec: &nvipamRuntimeSpec{
			runtimeSpec: runtimeSpec{Namespace: config.FromEnv().State.NetworkOperatorResourceNamespace},
			OSName:      osName,
		},
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
