package state

import (
	"github.com/Mellanox/mellanox-network-operator/pkg/consts"
)

// Group Represents a set of disjoint States that are Synced (Reconciled) together
type Group struct {
	states  []State
	results map[*State]Result
}

// NewStateGroup returns a new group of states
func NewStateGroup(states []State) Group {
	return Group{
		states:  states,
		results: make(map[*State]Result),
	}
}

// SyncGroup sync and update status for a list of states
func (sg *Group) Sync(customResource interface{}, infoCatalog InfoCatalog) (results []Result) {
	// sync and update status for the list of states
	for i := range sg.states {
		log.V(consts.LogLevelInfo).Info(
			"Sync State", "Name:", sg.states[i].Name(), "Description:", sg.states[i].Description())
		status, err := sg.states[i].Sync(customResource, infoCatalog)
		sg.results[&sg.states[i]] = Result{
			StateName: sg.states[i].Name(),
			Status:    status,
			ErrInfo:   err,
		}
	}
	results = sg.Results()
	log.V(consts.LogLevelDebug).Info("syncGroup", "results:", results)
	return results
}

// GroupDone returns whether or not all states in the group are ready, error in second arg in case
// one of the states returned with error
func (sg *Group) SyncDone() (done bool, err error) {
	done = false
	for _, result := range sg.results {
		if result.Status == SyncStateNotReady || result.Status == SyncStateError {
			err = result.ErrInfo
			return done, err
		}
	}
	// group done
	done = true
	return done, err
}

// Results return []Result of the last SyncGroup() invocation
func (sg *Group) Results() []Result {
	results := make([]Result, 0, len(sg.results))
	for _, result := range sg.results {
		results = append(results, result)
	}
	return results
}

// States returns the States in the State group
func (sg *Group) States() []State {
	return sg.states
}
