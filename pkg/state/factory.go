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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
)

var envConfig = config.FromEnv()

// NewManager creates a state.Manager for the given CRD Kind
func NewManager(
	crdKind string, k8sAPIClient client.Client, setupLog logr.Logger) (Manager, error) {
	states, err := newStates(crdKind, k8sAPIClient)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create state manager")
	}

	if setupLog.V(consts.LogLevelDebug).Enabled() {
		stateNames := make([]string, 0, len(states))
		for _, s := range states {
			stateNames = append(stateNames, s.Name())
		}
		setupLog.V(consts.LogLevelDebug).Info("Creating a new State manager with", "states:", stateNames)
	}

	return &stateManager{
		states: states,
		client: k8sAPIClient,
	}, nil
}

// newStates creates States that compose a State manager
func newStates(crdKind string, k8sAPIClient client.Client) ([]State, error) {
	switch crdKind {
	case mellanoxv1alpha1.NicClusterPolicyCRDName:
		return newNicClusterPolicyStates(k8sAPIClient)
	case mellanoxv1alpha1.MacvlanNetworkCRDName:
		return newMacvlanNetworkStates(k8sAPIClient)
	case mellanoxv1alpha1.HostDeviceNetworkCRDName:
		return newHostDeviceNetworkStates(k8sAPIClient)
	case mellanoxv1alpha1.IPoIBNetworkCRDName:
		return newIPoIBNetworkStates(k8sAPIClient)
	default:
		break
	}
	return nil, fmt.Errorf("unsupported CRD for states factory: %s", crdKind)
}

// newNicClusterPolicyStates creates states that reconcile NicClusterPolicy CRD
func newNicClusterPolicyStates(k8sAPIClient client.Client) ([]State, error) {
	manifestBaseDir := envConfig.State.ManifestBaseDir
	ofedState, _, err := NewStateOFED(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-ofed-driver"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create OFED driver State")
	}

	sharedDpState, _, err := NewStateRDMASharedDevicePlugin(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-rdma-shared-device-plugin"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Shared Device plugin State")
	}
	sriovDpState, _, err := NewStateSriovDp(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-sriov-device-plugin"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create SR-IOV Device plugin State")
	}
	multusState, _, err := NewStateMultusCNI(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-multus-cni"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Multus CNI State")
	}
	cniPluginsState, _, err := NewStateCNIPlugins(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-container-networking-plugins"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Container Networking CNI Plugins State")
	}
	ipoibState, _, err := NewStateIPoIBCNI(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-ipoib-cni"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Container Networking CNI Plugins State")
	}
	whereaboutState, _, err := NewStateWhereaboutsCNI(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-whereabouts-cni"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Whereabouts CNI State")
	}
	ibKubernetesState, _, err := NewStateIBKubernetes(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-ib-kubernetes"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create ib-kubernetes State")
	}
	nvIpamCniState, _, err := NewStateNVIPAMCNI(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-nv-ipam-cni"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create nv-ipam-cni State")
	}
	nicFeatureDiscoveryState, _, err := NewStateNICFeatureDiscovery(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-nic-feature-discovery"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create nic-feature-discovery State")
	}
	docaTelemetryServiceState, _, err := NewStateDOCATelemetryService(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-doca-telemetry-service"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create doca-telemetry-service State")
	}
	nicConfigurationOperatorState, _, err := NewStateNicConfigurationOperator(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-nic-configuration-operator"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create NIC Configuration Operator State")
	}
	return []State{
		multusState, cniPluginsState, ipoibState, whereaboutState,
		ofedState, sriovDpState, sharedDpState, ibKubernetesState, nvIpamCniState,
		nicFeatureDiscoveryState, docaTelemetryServiceState, nicConfigurationOperatorState}, nil
}

// newMacvlanNetworkStates creates states that reconcile MacvlanNetwork CRD
func newMacvlanNetworkStates(k8sAPIClient client.Client) ([]State, error) {
	manifestBaseDir := config.FromEnv().State.ManifestBaseDir

	macvlanNetworkState, _, err := NewStateMacvlanNetwork(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-macvlan-network"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create MacvlanNetwork CRD State")
	}
	return []State{macvlanNetworkState}, nil
}

// newHostDeviceNetworkStates creates states that reconcile HostDeviceNetwork CRD
func newHostDeviceNetworkStates(k8sAPIClient client.Client) ([]State, error) {
	manifestBaseDir := config.FromEnv().State.ManifestBaseDir

	hostdeviceNetworkState, _, err := NewStateHostDeviceNetwork(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-hostdevice-network"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create HostDeviceNetwork CRD State")
	}
	return []State{hostdeviceNetworkState}, nil
}

// newIPoIBNetworkStates creates states that reconcile IPoIBNetwork CRD
func newIPoIBNetworkStates(k8sAPIClient client.Client) ([]State, error) {
	manifestBaseDir := config.FromEnv().State.ManifestBaseDir

	ipoibNetworkState, _, err := NewStateIPoIBNetwork(
		k8sAPIClient, filepath.Join(manifestBaseDir, "state-ipoib-network"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create HostDeviceNetwork CRD State")
	}
	return []State{ipoibNetworkState}, nil
}
