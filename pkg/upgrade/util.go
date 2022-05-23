/*
Copyright 2022 NVIDIA CORPORATION & AFFILIATES
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

package upgrade

import (
	"sync"
)

// KeyedMutex is a struct that provides a per-key synchronized access
type KeyedMutex struct {
	mutexes sync.Map // Zero value is empty and ready for use
}

type UnlockFunc = func()

// Lock locks a mutex, associated with a given key and returns an unlock function
func (m *KeyedMutex) Lock(key string) UnlockFunc {
	value, _ := m.mutexes.LoadOrStore(key, &sync.Mutex{})
	mtx := value.(*sync.Mutex)
	mtx.Lock()
	return func() { mtx.Unlock() }
}
