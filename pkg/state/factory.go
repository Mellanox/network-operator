package state

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// NewStateManager creates a state.Manager for the given CRD Kind
func NewManager(crdKind string, k8sAPIClient client.Client) Manager {
	// TODO: Implement
	return &dummyMananger{states: newStatesForManager(crdKind, k8sAPIClient)}
}

// newStatesForManager creates States that compose a State manager
// nolint:unparam
func newStatesForManager(crdKind string, k8sAPIClient client.Client) [][]State {
	// TODO: Implement
	return [][]State{{&dummyState{client: k8sAPIClient}}}
}

type dummyMananger struct {
	states [][]State
}

// GetWatchSources gets Resources that should be watched by a Controller for this state manager
func (m *dummyMananger) GetWatchSources() []source.Kind {
	return []source.Kind{}
}

// SyncState reconciles the state of the system for the custom resource
func (m *dummyMananger) SyncState(request *reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

// UpdateStatus updates status of the CR Synced in the last SyncState() call
// TBD: Should we just update CR status in the SyncState method ?
func (m *dummyMananger) UpdateStatus() error {
	return nil
}

type dummyState struct {
	client client.Client
}

// Name provides the State name
func (s *dummyState) Name() string {
	return "dummy"
}

// Description provides the State description
func (s *dummyState) Description() string {
	return "description"
}

// Sync attempt to get the system to match the desired state which State represent.
// a sync operation must be relatively short and must not block the execution thread.
func (s *dummyState) Sync() (SyncState, error) {
	return SyncStateReady, nil
}

// Reset performs a reset operation for the State as if the state never attempted to reconcile the system
func (s *dummyState) Reset() (SyncState, error) {
	return SyncStateError, fmt.Errorf("reset Not implemented by the state")
}

// Get a map of source kinds that should be watched for the state keyed by the source kind name
func (s *dummyState) GetWatchSources() map[string]source.Kind {
	return make(map[string]source.Kind)
}
