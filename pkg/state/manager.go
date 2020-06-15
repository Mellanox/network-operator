package state

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/Mellanox/mellanox-network-operator/pkg/consts"
)

// StateManager manages a collection of states and handles transitions from State to State.
// A state manager invokes states in order to get the system to its desired state
type Manager interface {
	// GetWatchSources gets Resources that should be watched by a Controller for this state manager
	GetWatchSources() []*source.Kind
	// SyncState reconciles the state of the system and returns a list of status of the applied states
	// ServiceCatalog is provided to optionally provide a State additional services required for it to perform
	// the Sync operation.
	SyncState(customResource interface{}, serviceCatalog ServiceCatalog) (Results, error)
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
func (smgr *stateManager) SyncState(customResource interface{}, serviceCatalog ServiceCatalog) (Results, error) {
	// Sync groups of states, transition from one group to the other when a group finishes
	log.V(consts.LogLevelInfo).Info("Syncing system state")
	managerResult := Results{
		Status: SyncStateNotReady,
	}

	for i, stateGroup := range smgr.stateGroups {
		log.V(consts.LogLevelInfo).Info("Sync State group", "index", i)
		results := stateGroup.Sync(customResource, serviceCatalog)
		managerResult.StatesStatus = append(managerResult.StatesStatus, results...)

		done, err := stateGroup.SyncDone()
		if err != nil {
			log.V(consts.LogLevelError).Info("Error while syncing states", "Error:", err)
			return managerResult, err
		}
		if !done {
			log.V(consts.LogLevelError).Info("State Group Not ready")
			return managerResult, nil
		}
		log.V(consts.LogLevelInfo).Info("Sync Completed successfully for State group", "index", i)
	}
	// Done Syncing CR
	managerResult.Status = SyncStateReady
	log.V(consts.LogLevelInfo).Info("Sync Done for custom resource")
	return managerResult, nil
}
