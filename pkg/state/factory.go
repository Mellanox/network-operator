package state

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewStateManager creates a state.Manager for the given CRD Kind
func NewManager(crdKind string, k8sAPIClient client.Client) Manager {
	// TODO: Implement
	return &fakeMananger{states: newStates(crdKind, k8sAPIClient)}
}

// newStates creates States that compose a State manager
// nolint:unparam
func newStates(crdKind string, k8sAPIClient client.Client) [][]State {
	// TODO: Implement
	return [][]State{{&fakeState{client: k8sAPIClient}}}
}
