/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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

package config

import (
	"os"
	"sync"
	"testing"
)

func TestFromEnvDirectories(t *testing.T) {
	for _, mode := range []string{"unset", "empty", "custom"} {
		t.Run(mode, func(t *testing.T) {
			// These serial tests own the package singleton. Production loads it once.
			once = sync.Once{}
			operatorConfig = nil
			t.Cleanup(func() {
				once = sync.Once{}
				operatorConfig = nil
			})
			expected := map[string]string{
				"CNI_BIN_DIR":      "",
				"CNI_NETWORK_DIR":  "",
				"KUBELET_ROOT_DIR": "",
			}
			for name := range expected {
				if mode == "custom" {
					expected[name] = "/custom/" + name + "/"
				}
				t.Setenv(name, expected[name])
				if mode == "unset" {
					if err := os.Unsetenv(name); err != nil {
						t.Fatal(err)
					}
				}
			}
			cfg := FromEnv()
			originalState := cfg.State
			for name, actual := range map[string]string{
				"CNI_BIN_DIR":      cfg.State.CniBinDirectory,
				"CNI_NETWORK_DIR":  cfg.State.CniNetworkDirectory,
				"KUBELET_ROOT_DIR": cfg.State.KubeletRootDir,
			} {
				if actual != expected[name] {
					t.Errorf("%s = %q, want %q", name, actual, expected[name])
				}
				t.Setenv(name, "/changed/after/initialization")
			}
			if FromEnv() != cfg || FromEnv().State != originalState {
				t.Fatal("configuration was reloaded after initialization")
			}
		})
	}
}
