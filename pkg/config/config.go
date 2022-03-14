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

package config

import (
	"sync"

	"github.com/caarlos0/env/v6"
)

var once sync.Once
var operatorConfig *OperatorConfig

// Operator related configurations
type OperatorConfig struct {
	State      StateConfig
	Controller ControllerConfig
}

// state related configurations
type StateConfig struct {
	NetworkOperatorResourceNamespace string `env:"POD_NAMESPACE" envDefault:"nvidia-network-operator"`
	ManifestBaseDir                  string `env:"STATE_MANIFEST_BASE_DIR" envDefault:"./manifests"`
}

// Controller related configurations
type ControllerConfig struct {
	//nolint:stylecheck
	// Request requeue time(seconds) in case the system still needs to be reconciled
	RequeueTimeSeconds uint `env:"CONTROLLER_REQUEST_REQUEUE_SECONDS" envDefault:"5"`
}

func FromEnv() *OperatorConfig {
	once.Do(func() {
		operatorConfig = &OperatorConfig{}
		_ = env.Parse(operatorConfig)
	})
	return operatorConfig
}
