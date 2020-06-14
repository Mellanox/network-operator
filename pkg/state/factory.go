package state

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/mellanox-network-operator/pkg/consts"
)

// NewStateManager creates a state.Manager for the given CRD Kind
func NewManager(crdKind string, k8sAPIClient client.Client, scheme *runtime.Scheme) (Manager, error) {
	stateGroups, err := newStates(crdKind, k8sAPIClient, scheme)

	if err != nil {
		return nil, errors.Wrapf(err, "failed to create state manager")
	}

	if log.V(consts.LogLevelDebug).Enabled() {
		stateNames := make([][]string, len(stateGroups))
		for i, sg := range stateGroups {
			for _, state := range sg.States() {
				stateNames[i] = append(stateNames[i], state.Name())
			}
		}
		log.V(consts.LogLevelDebug).Info("Creating a new State manager with", "states:", stateNames)
	}

	return &stateManager{
		stateGroups: stateGroups,
		client:      k8sAPIClient,
	}, nil
}

// newStates creates States that compose a State manager
// nolint:unparam
func newStates(crdKind string, k8sAPIClient client.Client, scheme *runtime.Scheme) ([]Group, error) {
	return []Group{}, nil
}
