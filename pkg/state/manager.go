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
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/network-operator/pkg/consts"
)

// Manager manages a collection of states and handles transitions from State to State.
// A state manager invokes states in order to get the system to its desired state
type Manager interface {
	// GetWatchSources gets Resources that should be watched by a Controller for this state manager
	GetWatchSources() map[string]client.Object
	// SyncState reconciles the state of the system and returns a list of status of the applied states
	// InfoCatalog is provided to optionally provide a State additional information sources required for it to perform
	// the Sync operation.
	SyncState(ctx context.Context, customResource interface{}, infoCatalog InfoCatalog) Results
}

// Result is the result of a single State.Sync() invocation
type Result struct {
	StateName string
	Status    SyncState
	// if SyncStateError then ErrInfo will contain additional error information
	ErrInfo error
}

// Results is the result of a collection of State.Sync() invocations, Status reflects the global status of all states.
// If all are SyncStateReady then Status is SyncStateReady, if one is SyncStateNotReady, Status is SyncStateNotReady
type Results struct {
	Status       SyncState
	StatesStatus []Result
}

type stateManager struct {
	states []State
	client client.Client
}

func (smgr *stateManager) GetWatchSources() map[string]client.Object {
	kindMap := make(map[string]client.Object)
	for _, state := range smgr.states {
		wr := state.GetWatchSources()
		// append to kindMap
		for name, kind := range wr {
			if _, ok := kindMap[name]; !ok {
				kindMap[name] = kind
			}
		}
	}
	return kindMap
}

// SyncState attempts to reconcile the system by invoking Sync on each of the states
func (smgr *stateManager) SyncState(ctx context.Context, customResource interface{}, infoCatalog InfoCatalog) Results {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Syncing system state")

	managerResult := Results{
		Status: SyncStateNotReady,
	}
	statesReady := true

	for _, state := range smgr.states {
		reqLogger.V(consts.LogLevelInfo).Info("Sync State", "Name", state.Name(), "Description", state.Description())
		stateCtx := log.IntoContext(ctx, reqLogger.WithName("state").WithName(state.Name()))
		ss, err := state.Sync(stateCtx, customResource, infoCatalog)
		result := Result{StateName: state.Name(), Status: ss, ErrInfo: err}
		managerResult.StatesStatus = append(managerResult.StatesStatus, result)

		if result.Status == SyncStateNotReady || result.Status == SyncStateError {
			statesReady = false
		}

		if result.ErrInfo != nil {
			reqLogger.V(consts.LogLevelWarning).Error(result.ErrInfo, "Error while syncing state")
		}
	}

	if statesReady {
		// Done Syncing CR
		managerResult.Status = SyncStateReady
		reqLogger.V(consts.LogLevelInfo).Info("Sync Done for custom resource")
	} else {
		reqLogger.V(consts.LogLevelInfo).Info("Sync not Done for custom resource")
	}

	return managerResult
}
