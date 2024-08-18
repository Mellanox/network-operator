/*
2024 NVIDIA CORPORATION & AFFILIATES

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

type docaTelemetryServiceState struct {
	stateSkel
}

type dtsRuntimeSpec struct {
	runtimeSpec
	ContainerResources ContainerResourcesMap
	IsOpenshift        bool
}

const (
	docaTelemetryServiceName                 = "state-doca-telemetry-service"
	docaTelemetryServiceDefaultConfigMapName = "doca-telemetry-service"
	docaTelemetryServiceDescription          = "DOCA Telemetry Service deployed in the cluster"
)

// DOCATelemetryServiceManifestRenderData is used to render Kubernetes objects related to DOCA Telemetry Service.
type DOCATelemetryServiceManifestRenderData struct {
	CrSpec          *mellanoxv1alpha1.DOCATelemetryServiceSpec
	ConfigMapName   string
	DeployConfigMap bool
	RuntimeSpec     *dtsRuntimeSpec
	Tolerations     []v1.Toleration
	NodeAffinity    *v1.NodeAffinity
}

// Sync attempt to get the system to match the desired state which State represents.
// a sync operation must be relatively short and must not block the execution thread.
func (d docaTelemetryServiceState) Sync(
	ctx context.Context, customResource interface{}, infoCatalog InfoCatalog) (SyncState, error) {
	cr := customResource.(*mellanoxv1alpha1.NicClusterPolicy)

	reqLogger := log.FromContext(ctx)
	reqLogger.WithValues("State:", d.name, "Name:", cr.Name, "Namespace:", cr.Namespace)
	log.IntoContext(ctx, reqLogger)
	reqLogger.V(consts.LogLevelInfo).Info("Sync Custom resource")

	if cr.Spec.DOCATelemetryService == nil {
		// Either this state was not required to run or an update occurred and we need to remove
		// the resources that where created.
		return d.handleStateObjectsDeletion(ctx)
	}

	objs, err := d.GetManifestObjects(ctx, cr, infoCatalog, reqLogger)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create k8s objects from manifest")
	}
	if len(objs) == 0 {
		return SyncStateNotReady, nil
	}

	// Create objects if they don't exist, Update objects if they do exist
	err = d.createOrUpdateObjs(ctx, func(obj *unstructured.Unstructured) error {
		if err := controllerutil.SetControllerReference(cr, obj, d.client.Scheme()); err != nil {
			return errors.Wrap(err, "failed to set controller reference for object")
		}
		return nil
	}, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create/update objects")
	}
	waitForStaleObjectsRemoval, err := d.handleStaleStateObjects(ctx, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to handle state stale objects")
	}
	if waitForStaleObjectsRemoval {
		return SyncStateNotReady, nil
	}
	// Check objects status
	syncState, err := d.getSyncState(ctx, objs)
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to get sync state")
	}
	return syncState, nil
}

// GetManifestObjects returns the Unstructured objects to deploy for DOCA Telemetry Service.
func (d docaTelemetryServiceState) GetManifestObjects(
	_ context.Context, cr *mellanoxv1alpha1.NicClusterPolicy,
	catalog InfoCatalog, reqLogger logr.Logger) ([]*unstructured.Unstructured, error) {
	if cr == nil || cr.Spec.DOCATelemetryService == nil {
		return nil, errors.New("failed to render objects: state spec is nil")
	}
	dts := cr.Spec.DOCATelemetryService

	configMapName := docaTelemetryServiceDefaultConfigMapName
	if dts.Config != nil {
		configMapName = dts.Config.FromConfigMap
	}
	clusterInfo := catalog.GetClusterTypeProvider()
	if clusterInfo == nil {
		return nil, errors.New("clusterInfo provider required")
	}
	renderData := &DOCATelemetryServiceManifestRenderData{
		CrSpec:          dts,
		ConfigMapName:   configMapName,
		DeployConfigMap: shouldDeployConfigMap(cr.Spec.DOCATelemetryService),
		Tolerations:     cr.Spec.Tolerations,
		NodeAffinity:    cr.Spec.NodeAffinity,
		RuntimeSpec: &dtsRuntimeSpec{
			runtimeSpec: runtimeSpec{
				Namespace: config.FromEnv().State.NetworkOperatorResourceNamespace,
			},
			ContainerResources: createContainerResourcesMap(cr.Spec.DOCATelemetryService.ContainerResources),
			IsOpenshift:        clusterInfo.IsOpenshift(),
		},
	}

	// Render objects related to the DOCATelemetryService
	reqLogger.V(consts.LogLevelDebug).Info("Rendering objects", "data:", renderData)
	renderedObjects, err := d.renderer.RenderObjects(&render.TemplatingData{Data: renderData})
	if err != nil {
		return nil, errors.Wrap(err, "failed to render objects")
	}

	reqLogger.V(consts.LogLevelDebug).Info("Rendered", "objects:", renderedObjects)
	return renderedObjects, nil
}

// Name returns the name of the DOCA Telemetry Service.
func (d docaTelemetryServiceState) Name() string {
	return docaTelemetryServiceName
}

// Description returns the description of the DOCA Telemetry Service.
func (d docaTelemetryServiceState) Description() string {
	return docaTelemetryServiceDescription
}

// GetWatchSources returns the objects that should be watched to trigger events for the DOCA Telemetry Service state.
func (docaTelemetryServiceState) GetWatchSources() map[string]client.Object {
	wr := make(map[string]client.Object)
	wr["DaemonSet"] = &appsv1.DaemonSet{}
	return wr
}

// NewStateDOCATelemetryService creates a new state for DOCA Telemetry Service.
func NewStateDOCATelemetryService(
	c client.Client, manifestDir string) (State, ManifestRenderer, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	state := &docaTelemetryServiceState{
		stateSkel: stateSkel{
			name:        docaTelemetryServiceName,
			description: docaTelemetryServiceDescription,
			client:      c,
			renderer:    renderer,
		}}
	return state, state, nil
}

// If the DOCATelemetryService defines a configuration we should not deploy the configMap.
func shouldDeployConfigMap(dts *mellanoxv1alpha1.DOCATelemetryServiceSpec) bool {
	return dts.Config == nil
}
