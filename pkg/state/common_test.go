/*
2024 NVIDIA CORPORATION & AFFILIATES

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

package state_test

import (
	"encoding/json"

	clustertype_mocks "github.com/Mellanox/network-operator/pkg/clustertype/mocks"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
	staticconfig_mocks "github.com/Mellanox/network-operator/pkg/staticconfig/mocks"
)

func getTestCatalog() state.InfoCatalog {
	catalog := state.NewInfoCatalog()
	clusterTypeProvider := clustertype_mocks.Provider{}
	clusterTypeProvider.On("IsOpenshift").Return(false)
	staticConfigProvider := staticconfig_mocks.Provider{}
	staticConfigProvider.On("GetStaticConfig").Return(staticconfig.StaticConfig{CniBinDirectory: ""})
	catalog.Add(state.InfoTypeStaticConfig, &staticConfigProvider)
	catalog.Add(state.InfoTypeClusterType, &clusterTypeProvider)
	return catalog
}

type ipam struct {
	Type    string   `json:"type"`
	Range   string   `json:"range"`
	Exclude []string `json:"exclude"`
}

type nadConfig struct {
	CNIVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Master     string `json:"master"`
	Mode       string `json:"mode"`
	MTU        int    `json:"mtu"`
	IPAM       ipam   `json:"ipam"`
}

func getNADConfig(jsonData string) (*nadConfig, error) {
	config := &nadConfig{}
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
