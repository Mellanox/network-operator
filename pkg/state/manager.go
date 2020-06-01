package state

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// StateManager manages a collection of states and handles transitions from State to State.
// A state manager invokes states in order to get the system to its desired state
type Manager interface {
	// GetWatchSources gets Resources that should be watched by a Controller for this state manager
	GetWatchSources() []source.Kind
	// SyncState reconciles the state of the system and updates status for the custom resource
	SyncState(request *reconcile.Request) (reconcile.Result, error)
}
