package state

import (
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type SyncState string

// Represents the Sync state of a specific State or a collection of States
const (
	SyncStateReady    = "ready"
	SyncStateNotReady = "notReady"
	SyncStateIgnore   = "ignore"
	SyncStateReset    = "reset"
	SyncStateError    = "error"
)

var log = logf.Log.WithName("state")

// State Represents a single State that requires a set of k8s API operations to be performed.
// A state is associated with a set of resources, it checks the system state against the given set of resources
// and reconciles accordingly. It basically reconciles the system to the given state.
type State interface {
	// Name provides the State name
	Name() string
	// Description provides the State description
	Description() string
	// Sync attempt to get the system to match the desired state as depicted in the custom resource
	// for the bits related to the specific state, State represents.
	// a sync operation must be relatively short and must not block the execution thread.
	// ServiceCatalog is provided to optionally provide a State additional services required for it to perform
	// the Sync operation.
	Sync(customResource interface{}, serviceCatalog ServiceCatalog) (SyncState, error)
	// Get a map of source kinds that should be watched for the state keyed by the source kind name
	GetWatchSources() map[string]*source.Kind
}
