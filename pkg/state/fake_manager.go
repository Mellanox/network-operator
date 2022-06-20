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
	"sigs.k8s.io/controller-runtime/pkg/source"
)

//nolint:unused
type fakeMananger struct {
	watchResources []*source.Kind
}

//nolint:unused
// GetWatchSources gets Resources that should be watched by a Controller for this state manager
func (m *fakeMananger) GetWatchSources() []*source.Kind {
	return m.watchResources
}

//nolint:unused
// SyncState reconciles the state of the system for the custom resource
func (m *fakeMananger) SyncState(customResource interface{}, infoCatalog InfoCatalog) (Results, error) {
	return Results{
		Status:       SyncStateNotReady,
		StatesStatus: nil,
	}, nil
}
