package state

import (
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
	// Get a map of source kinds that should be watched for the state keyed by the source kind name
	GetWatchSources() map[string]source.Kind
}
