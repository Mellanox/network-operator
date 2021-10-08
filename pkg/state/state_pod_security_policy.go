/*
----------------------------------------------------

  2021 NVIDIA CORPORATION & AFFILIATES

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.

----------------------------------------------------
*/

package state //nolint:dupl

import (
	"github.com/pkg/errors"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/utils"
)

// NewStatePodSecurityPolicy creates a new pod security policy state
func NewStatePodSecurityPolicy(k8sAPIClient client.Client, scheme *runtime.Scheme, manifestDir string) (State, error) {
	files, err := utils.GetFilesWithSuffix(manifestDir, render.ManifestFileSuffix...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files from manifest dir")
	}

	renderer := render.NewRenderer(files)
	return &statePodSecurityPolicy{
		stateSkel: stateSkel{
			name:        "state-pod-security-policy",
			description: "Privileged pod security policy deployed in the cluster",
			client:      k8sAPIClient,
			scheme:      scheme,
			renderer:    renderer,
		}}, nil
}

type statePodSecurityPolicy struct {
	stateSkel
}

type podSecurityPolicyManifestRenderData struct {
	RuntimeSpec *runtimeSpec
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
//nolint:dupl
func (s *statePodSecurityPolicy) Sync(customResource interface{}, infoCatalog InfoCatalog) (SyncState, error) {
	cr := customResource.(*mellanoxv1alpha1.NicClusterPolicy)
	log.V(consts.LogLevelInfo).Info(
		"Sync Custom resource", "State:", s.name, "Name:", cr.Name, "Namespace:", cr.Namespace)

	if !cr.Spec.PSP.Enabled {
		// Either this state was not required to run or an update occurred and we need to remove
		// the resources that where created.
		log.V(consts.LogLevelInfo).Info("pod security policy is not enabled, no action required")
		return SyncStateIgnore, nil
	}

	objs, err := s.getManifestObjects()
	if err != nil {
		return SyncStateNotReady, errors.Wrap(err, "failed to create k8s objects from manifest")
	}
	if len(objs) == 0 {
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
func (s *statePodSecurityPolicy) GetWatchSources() map[string]*source.Kind {
	wr := make(map[string]*source.Kind)
	wr["PodSecurityPolicy"] = &source.Kind{Type: &policyv1beta1.PodSecurityPolicy{}}
	return wr
}

func (s *statePodSecurityPolicy) getManifestObjects() ([]*unstructured.Unstructured, error) {
	renderData := &podSecurityPolicyManifestRenderData{
		RuntimeSpec: &runtimeSpec{
			Namespace: consts.NetworkOperatorResourceNamespace,
		},
	}
	// render objects
	objs, err := s.renderer.RenderObjects(&render.TemplatingData{Data: renderData})
	if err != nil {
		return nil, errors.Wrap(err, "failed to render objects")
	}
	log.V(consts.LogLevelDebug).Info("Rendered", "objects:", objs)
	return objs, nil
}
