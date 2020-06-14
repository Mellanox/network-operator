package state

import (
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// StateManager manages a collection of states and handles transitions from State to State.
// A state manager invokes states in order to get the system to its desired state
type Manager interface {
	// GetWatchSources gets Resources that should be watched by a Controller for this state manager
	GetWatchSources() []*source.Kind
	// SyncState reconciles the state of the system and returns a list of status of the applied states
	// ServiceCatalog is provided to optionally provide a State additional services required for it to perform
	// the Sync operation.
	SyncState(customResource interface{}, serviceCatalog ServiceCatalog) (Results, error)
}

// Represent a Result of a single State.Sync() invocation
type Result struct {
	StateName string
	Status    SyncState
	// if SyncStateError then ErrInfo will contain additional error information
	ErrInfo error
}

// Represent the Results of a collection of State.Sync() invocations, Status reflects the global status of all states.
// If all are SyncStateReady then Status is SyncStateReady, if one is SyncStateNotReady, Status is SyncStateNotReady
type Results struct {
	Status       SyncState
	StatesStatus []Result
}
