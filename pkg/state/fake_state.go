/*
Copyright 2020 NVIDIA

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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeState struct {
	name, description string
	watchResources    map[string]client.Object
	syncState         SyncState
}

// Name provides the State name
func (s *fakeState) Name() string {
	return s.name
}

// Description provides the State description
func (s *fakeState) Description() string {
	return s.description
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
func (s *fakeState) Sync(_ context.Context, _ interface{}, _ InfoCatalog) (SyncState, error) {
	return s.syncState, nil
}

// Get a map of source kinds that should be watched for the state keyed by the source kind name
func (s *fakeState) GetWatchSources() map[string]client.Object {
	return s.watchResources
}
