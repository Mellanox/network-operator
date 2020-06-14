package state

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/mellanox-network-operator/pkg/apis/mellanox/v1alpha1"
	"github.com/Mellanox/mellanox-network-operator/pkg/config"
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
	switch crdKind {
	case mellanoxv1alpha1.NicClusterPolicyCRDName:
		return newNicClusterPolicyStates(k8sAPIClient, scheme)
	default:
		break
	}
	return nil, fmt.Errorf("unsupported CRD for states factory: %s", crdKind)
}

// newNicClusterPolicyStates creates states that reconcile NicClusterPolicy CRD
func newNicClusterPolicyStates(k8sAPIClient client.Client, scheme *runtime.Scheme) ([]Group, error) {
	var states []State = make([]State, 0, 1)
	s, err := NewStateOFED(
		k8sAPIClient, scheme, filepath.Join(config.FromEnv().State.ManifestBaseDir, "stage-ofed-driver"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create OFED driver State")
	}
	states = append(states, s)

	return []Group{NewStateGroup(states)}, nil
}
