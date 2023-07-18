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
	})
})
