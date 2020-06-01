package state

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type fakeState struct {
	client            client.Client
	name, description string
	watchResources    map[string]source.Kind
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
func (s *fakeState) Sync() (SyncState, error) {
	return SyncStateReady, nil
}

// Get a map of source kinds that should be watched for the state keyed by the source kind name
func (s *fakeState) GetWatchSources() map[string]source.Kind {
	return s.watchResources
}
