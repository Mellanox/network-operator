/*
Copyright 2020 NVIDIA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package state

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/pkg/apis/mellanox/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
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
	manifestBaseDir := config.FromEnv().State.ManifestBaseDir
	ofedState, err := NewStateOFED(
		k8sAPIClient, scheme, filepath.Join(manifestBaseDir, "stage-ofed-driver"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create OFED driver State")
	}

	sharedDpState, err := NewStateSharedDp(
		k8sAPIClient, scheme, filepath.Join(manifestBaseDir, "stage-rdma-device-plugin"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Device plugin State")
	}
	nvPeerMemState, err := NewStateNVPeer(
		k8sAPIClient, scheme, filepath.Join(manifestBaseDir, "stage-nv-peer-mem-driver"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create NV peer memory driver State")
	}
	multusState, err := NewStateMultusCNI(
		k8sAPIClient, scheme, filepath.Join(manifestBaseDir, "stage-multus-cni"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Multus CNI State")
	}
	cniPluginsState, err := NewStateCNIPlugins(
		k8sAPIClient, scheme, filepath.Join(manifestBaseDir, "stage-container-networking-plugins"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Container Networking CNI Plugins State")
	}
	whereaboutState, err := NewStateWhereaboutsCNI(
		k8sAPIClient, scheme, filepath.Join(manifestBaseDir, "stage-whereabouts-cni"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Whereabouts CNI State")
	}

	return []Group{
		NewStateGroup([]State{ofedState}),
		NewStateGroup([]State{sharedDpState, nvPeerMemState}),
		NewStateGroup([]State{multusState, cniPluginsState, whereaboutState}),
	}, nil
}
