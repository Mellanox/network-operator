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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/Mellanox/network-operator/pkg/consts"
)

// StateManager manages a collection of states and handles transitions from State to State.
// A state manager invokes states in order to get the system to its desired state
type Manager interface {
	// GetWatchSources gets Resources that should be watched by a Controller for this state manager
	GetWatchSources() []*source.Kind
	// SyncState reconciles the state of the system and returns a list of status of the applied states
	// InfoCatalog is provided to optionally provide a State additional information sources required for it to perform
	// the Sync operation.
	SyncState(customResource interface{}, infoCatalog InfoCatalog) (Results, error)
}

// Represent a Result of a single State.Sync() invocation
type Result struct {
	StateName string
	Status    SyncState
	// if SyncStateError then ErrInfo will contain additional error information
	ErrInfo error
}

// Represent the Results of a collection of State.Sync() invocations, Status reflects the global status of all states.
// If all are SyncStateReady then Status is SyncStateReady, if one is SyncStateNotReady, Status is SyncStateNotReady
type Results struct {
	Status       SyncState
	StatesStatus []Result
}

type stateManager struct {
	stateGroups []Group
	client      client.Client
}

func (smgr *stateManager) GetWatchSources() []*source.Kind {
	kindMap := make(map[string]*source.Kind)
	for _, stateGroup := range smgr.stateGroups {
		for _, state := range stateGroup.States() {
			wr := state.GetWatchSources()
			// append to kindMap
			for name, kind := range wr {
				if _, ok := kindMap[name]; !ok {
					kindMap[name] = kind
				}
			}
		}
	}

	kinds := make([]*source.Kind, 0, len(kindMap))
	kindNames := make([]string, 0, len(kindMap))
	for kindName, kind := range kindMap {
		kinds = append(kinds, kind)
		kindNames = append(kindNames, kindName)
	}
	log.V(consts.LogLevelDebug).Info("Watch resources for manager", "sources:", kindNames)
	return kinds
}

// SyncState attempts to reconcile the system by invoking Sync on each of the states
func (smgr *stateManager) SyncState(customResource interface{}, infoCatalog InfoCatalog) (Results, error) {
	// Sync groups of states, transition from one group to the other when a group finishes
	log.V(consts.LogLevelInfo).Info("Syncing system state")
	managerResult := Results{
		Status: SyncStateNotReady,
	}

	for i, stateGroup := range smgr.stateGroups {
		log.V(consts.LogLevelInfo).Info("Sync State group", "index", i)
		results := stateGroup.Sync(customResource, infoCatalog)
		managerResult.StatesStatus = append(managerResult.StatesStatus, results...)

		done, err := stateGroup.SyncDone()
		if err != nil {
			log.V(consts.LogLevelError).Info("Error while syncing states", "Error:", err)
			return managerResult, err
		}
		if !done {
			log.V(consts.LogLevelInfo).Info("State Group Not ready")
			return managerResult, nil
		}
		log.V(consts.LogLevelInfo).Info("Sync Completed successfully for State group", "index", i)
	}
	// Done Syncing CR
	managerResult.Status = SyncStateReady
	log.V(consts.LogLevelInfo).Info("Sync Done for custom resource")
	return managerResult, nil
}
