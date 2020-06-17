package state

import (
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// nolint:unused
type fakeMananger struct {
	watchResources []*source.Kind
}

// GetWatchSources gets Resources that should be watched by a Controller for this state manager
func (m *fakeMananger) GetWatchSources() []*source.Kind {
	return m.watchResources
}

// SyncState reconciles the state of the system for the custom resource
func (m *fakeMananger) SyncState(customResource interface{}, infoCatalog InfoCatalog) (Results, error) {
	return Results{
		Status:       SyncStateNotReady,
		StatesStatus: nil,
	}, nil
}
