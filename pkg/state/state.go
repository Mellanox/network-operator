package state

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type SyncState string

const (
	SyncStateReady    = "Ready"
	SyncStateNotReady = "NotReady"
	SyncStateReset    = "Reset"
	SyncStateError    = "Error"
)

// State Represents a single State that requires a set of k8s API operations to be performed.
// A state is associated with a set of resources, it checks the system state against the given set of resources
// and reconciles accordingly. It basically reconciles the system to the given state.
type State interface {
	// Name provides the State name
	Name() string
	// Description provides the State description
	Description() string
	// Sync attempt to get the system to match the desired state which State represent.
	// a sync operation must be relatively short and must not block the execution thread.
	Sync() (SyncState, error)
	// Reset performs a reset operation for the State as if the state never attempted to reconcile the system
	Reset() (SyncState, error)
	// Get a map of source kinds that should be watched for the state keyed by the source kind name
	GetWatchSources() map[string]source.Kind
}

// StateManager manages a collection of states and handles transitions from State to State.
// A state manager invokes states in order to get the system to its desired state
type Manager interface {
	// GetWatchSources gets Resources that should be watched by a Controller for this state manager
	GetWatchSources() []source.Kind
	// SyncState reconciles the state of the system for the custom resource
	SyncState(request *reconcile.Request) (reconcile.Result, error)
	// UpdateStatus updates status of the CR Synced in the last SyncState() call
	// TBD: Should we just update CR status in the SyncState method ?
	UpdateStatus() error
}
