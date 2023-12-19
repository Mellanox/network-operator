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
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/network-operator/pkg/consts"
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

// GetRevision returns controller revision which is saved in the object.
// returns 0 if revision is not set for the object
func GetRevision(o client.Object) ControllerRevision {
	val := o.GetAnnotations()[consts.ControllerRevisionAnnotation]
	rev, err := strconv.ParseUint(val, 10, 32)
	if err != nil {
		return 0
	}
	return ControllerRevision(rev)
}

// SetRevision saves controller revision to the object.
func SetRevision(o client.Object, rev ControllerRevision) {
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[consts.ControllerRevisionAnnotation] = strconv.FormatUint(uint64(rev), 10)
	o.SetAnnotations(annotations)
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
