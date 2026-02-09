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

// Package revision manages revision annotation for Kubernetes objects.
package revision

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerRevision is the data to be stored as checkpoint
type ControllerRevision uint32

// CalculateRevision calculates controller revision for the object
func CalculateRevision(o client.Object) (ControllerRevision, error) {
	rev, err := getHash(o)
	if err != nil {
		return 0, fmt.Errorf("failed to compute controller revision for the object: %v", err)
	}
	return ControllerRevision(rev), nil
}

// Get returns calculated checksum for the object
func getHash(o client.Object) (uint32, error) {
	h := fnv.New32a()
	data, err := json.Marshal(o)
	if err != nil {
		return 0, err
	}
	_, _ = h.Write(data)
	return h.Sum32(), nil
}
