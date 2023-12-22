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

package state

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/render"
)

// ManifestRenderer is a common interface for states that render manifests based on NicClusterPolicy
type ManifestRenderer interface {
	GetManifestObjects(ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy,
		catalog InfoCatalog, reqLogger logr.Logger) ([]*unstructured.Unstructured, error)
	SetRenderer(renderer render.Renderer)
}

// ParseContainerNames renders the manifests using ManifestRenderer and extracts container names out of them
func ParseContainerNames(
	renderer ManifestRenderer, cr *mellanoxv1alpha1.NicClusterPolicy, reqLogger logr.Logger) ([]string, error) {
	catalog := getDummyCatalog()
	ctx := context.TODO()

	manifests, err := renderer.GetManifestObjects(ctx, cr, catalog, reqLogger)
	if err != nil {
		return nil, err
	}

	if renderer == nil {
		return nil, errors.New("renderer is nil")
	}

	var containerNames []string

	for _, u := range manifests {
		if u.GetKind() == "Deployment" || u.GetKind() == "DaemonSet" {
			containers, found, err := unstructured.NestedSlice(
				u.Object, "spec", "template", "spec", "containers")
			if err != nil || !found {
				continue
			}

			for _, c := range containers {
				containerMap := c.(map[string]interface{})
				containerName, found, err := unstructured.NestedString(containerMap, "name")
				if err != nil || !found {
					continue
				}
				containerNames = append(containerNames, containerName)
			}
		}
	}

	return containerNames, nil
}
