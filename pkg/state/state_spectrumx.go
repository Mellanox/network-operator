/*
2025 NVIDIA CORPORATION & AFFILIATES

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

// NewStateSpectrumXOperator creates a new state for Spectrum-X Operator
func NewStateSpectrumXOperator(
	k8sAPIClient client.Client, manifestDir string) (State, ManifestRenderer, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	state := &stateSpectrumXOperator{
		stateSkel: stateSkel{
			name:        "state-spectrum-x-operator",
			description: "spectrum-x operator deployed in the cluster",
			client:      k8sAPIClient,
			renderer:    renderer,
		}}
	return state, state, nil
}

type stateSpectrumXOperator struct {
	stateSkel
}

// spectrumXOperatorRenderData is Spectrum-X Operator manifest rendering data
type spectrumXOperatorRenderData struct {
	CrSpec       *mellanoxv1alpha1.SpectrumXOperatorSpec
	NodeAffinity *v1.NodeAffinity
	Tolerations  []v1.Toleration
	RuntimeSpec  *spectrumXRuntimeSpec
}

type spectrumXRuntimeSpec struct {
	runtimeSpec
	IsOpenshift        bool
	ContainerResources ContainerResourcesMap
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
//
//nolint:dupl
func (s *stateSpectrumXOperator) Sync(
	ctx context.Context, customResource interface{}, infoCatalog InfoCatalog) (SyncState, error) {
	reqLogger := log.FromContext(ctx)
	cr := customResource.(*mellanoxv1alpha1.NicClusterPolicy)
	reqLogger.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.Name, "Namespace:", cr.Namespace)

	if cr.Spec.SpectrumXOperator == nil {
		// Either this state was not required to run or an update occurred and we need to remove
		// the resources that where created.
		return s.handleStateObjectsDeletion(ctx)
	}

	clusterInfo := infoCatalog.GetClusterTypeProvider()
	if clusterInfo == nil {
		return SyncStateError, errors.New("unexpected state, catalog does not provide cluster type info")
	}

	// Fill ManifestRenderData and render objects
	objs, err := s.GetManifestObjects(ctx, cr, infoCatalog, reqLogger)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create k8s objects from manifest")
	}
	if len(objs) == 0 {
		return SyncStateNotReady, nil
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

// GetWatchSources returns a map of source kinds that should be watched for the state keyed by the source kind name
func (s *stateSpectrumXOperator) GetWatchSources() map[string]client.Object {
	wr := make(map[string]client.Object)
	wr["Deployment"] = &appsv1.Deployment{}
	wr["DaemonSet"] = &appsv1.DaemonSet{}
	return wr
}

//nolint:dupl
func (s *stateSpectrumXOperator) GetManifestObjects(
	_ context.Context, cr *mellanoxv1alpha1.NicClusterPolicy,
	catalog InfoCatalog, reqLogger logr.Logger) ([]*unstructured.Unstructured, error) {
	if cr == nil || cr.Spec.SpectrumXOperator == nil {
		return nil, errors.New("failed to render objects: state spec is nil")
	}

	clusterInfo := catalog.GetClusterTypeProvider()
	if clusterInfo == nil {
		return nil, errors.New("clusterInfo provider required")
	}
	renderData := &spectrumXOperatorRenderData{
		CrSpec:       cr.Spec.SpectrumXOperator,
		NodeAffinity: cr.Spec.NodeAffinity,
		Tolerations:  cr.Spec.Tolerations,
		RuntimeSpec: &spectrumXRuntimeSpec{
			runtimeSpec:        runtimeSpec{config.FromEnv().State.NetworkOperatorResourceNamespace},
			IsOpenshift:        clusterInfo.IsOpenshift(),
			ContainerResources: createContainerResourcesMap(cr.Spec.SpectrumXOperator.ContainerResources),
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
