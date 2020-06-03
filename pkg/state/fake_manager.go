package state

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type fakeMananger struct {
	states         [][]State
	watchResources []source.Kind
}

// GetWatchSources gets Resources that should be watched by a Controller for this state manager
func (m *fakeMananger) GetWatchSources() []source.Kind {
	return m.watchResources
}

// SyncState reconciles the state of the system for the custom resource
func (m *fakeMananger) SyncState(request *reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}
