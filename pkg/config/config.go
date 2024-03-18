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

// Package config contains configuration for the Operator.
package config

import (
	"sync"

	"github.com/caarlos0/env/v6"
)

var once sync.Once
var operatorConfig *OperatorConfig

// OperatorConfig holds configuration for the Operator.
type OperatorConfig struct {
	State      StateConfig
	Controller ControllerConfig
	// disable migration logic in the operator.
	DisableMigration bool `env:"DISABLE_MIGRATION" envDefault:"false"`
}

// StateConfig holds configuration for Operator State.
type StateConfig struct {
	NetworkOperatorResourceNamespace string `env:"POD_NAMESPACE" envDefault:"nvidia-network-operator"`
	ManifestBaseDir                  string `env:"STATE_MANIFEST_BASE_DIR" envDefault:"./manifests"`
	OFEDState                        OFEDStateConfig
	DocaDriverImagePollTimeMinutes   uint `env:"DOCA_DRIVER_IMAGE_POLL_TIME_MINUTES" envDefault:"30"`
}

// ControllerConfig holds configuration for Operator controllers.
type ControllerConfig struct {
	//nolint:stylecheck
	// Request requeue time(seconds) in case the system still needs to be reconciled
	RequeueTimeSeconds uint `env:"CONTROLLER_REQUEST_REQUEUE_SECONDS" envDefault:"5"`
}

// OFEDStateConfig contains extra configuration options for the OFED state which
// can't be configured via CRD
type OFEDStateConfig struct {
	// InitContainerImage is a full image name (registry, image name, tag) for the OFED init container.
	// The init container will not be deployed if this variable is empty/not set.
	InitContainerImage string `env:"OFED_INIT_CONTAINER_IMAGE"`
	// UseDTK Enable use of Driver ToolKit side car to compile OFED drivers (Relevant for OpenShift only)
	UseDTK bool `env:"USE_DTK" envDefault:"true"`
}

// FromEnv pulls the operator configuration from the environment.
func FromEnv() *OperatorConfig {
	once.Do(func() {
		operatorConfig = &OperatorConfig{}
		_ = env.Parse(operatorConfig)
	})
	return operatorConfig
}
