/*
Copyright 2021 NVIDIA

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/network-operator/pkg/testing/mocks"
)

var _ = Describe("Manager tests", func() {

	Context("Sync states", func() {
		It("Should states be ready", func() {
			testState := &fakeState{
				name:        "test",
				description: "test description",
				syncState:   SyncStateReady,
			}
			client := mocks.ControllerRuntimeClient{}
			manager := &stateManager{
				states: []State{testState},
				client: &client,
			}
			results := manager.SyncState(context.TODO(), nil, nil)
			Expect(results.Status).To(Equal(SyncState(SyncStateReady)))
			Expect(results.StatesStatus[0].StateName).To(Equal("test"))
			Expect(results.StatesStatus[0].Status).To(Equal(SyncState(SyncStateReady)))
		})
		It("Should render all", func() {
			testStateNotReady := &fakeState{
				name:        "test not ready",
				description: "test description",
				syncState:   SyncStateNotReady,
			}
			testStateReady := &fakeState{
				name:        "test ready",
				description: "test description",
				syncState:   SyncStateReady,
			}
			client := mocks.ControllerRuntimeClient{}
			manager := &stateManager{
				states: []State{testStateNotReady, testStateReady},
				client: &client,
			}
			results := manager.SyncState(context.TODO(), nil, nil)
			Expect(results.Status).To(Equal(SyncState(SyncStateNotReady)))
			Expect(results.StatesStatus[0].StateName).To(Equal("test not ready"))
			Expect(results.StatesStatus[0].Status).To(Equal(SyncState(SyncStateNotReady)))
			Expect(results.StatesStatus[1].StateName).To(Equal("test ready"))
			Expect(results.StatesStatus[1].Status).To(Equal(SyncState(SyncStateReady)))
		})
		It("Should collect the soonest requeue a state asked for", func() {
			client := mocks.ControllerRuntimeClient{}
			manager := &stateManager{
				states: []State{
					&fakeState{name: "no deadline", syncState: SyncStateReady},
					&fakeState{name: "late deadline", syncState: SyncStateReady, requeueAfter: 10 * time.Minute},
					&fakeState{name: "soon deadline", syncState: SyncStateReady, requeueAfter: 2 * time.Minute},
				},
				client: &client,
			}
			results := manager.SyncState(context.TODO(), nil, nil)
			Expect(results.Status).To(Equal(SyncState(SyncStateReady)))
			Expect(results.RequeueAfter).To(Equal(2 * time.Minute))
		})
		It("Should not ask for a requeue when no state has a deadline", func() {
			client := mocks.ControllerRuntimeClient{}
			manager := &stateManager{
				states: []State{&fakeState{name: "test", syncState: SyncStateReady}},
				client: &client,
			}
			Expect(manager.SyncState(context.TODO(), nil, nil).RequeueAfter).To(BeZero())
		})
	})
})
