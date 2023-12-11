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

import "github.com/Mellanox/network-operator/api/v1alpha1"

// ContainerResourcesMap is a helper struct for states to consume container resources when rendering manifests
type ContainerResourcesMap map[string]v1alpha1.ResourceRequirements

func createContainerResourcesMap(resources []v1alpha1.ResourceRequirements) ContainerResourcesMap {
	containerResources := ContainerResourcesMap{}
	for _, val := range resources {
		containerResources[val.Name] = val
	}

	return containerResources
}
