package config

import (
	"sync"
	"time"

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
	ManifestBaseDir string `env:"STATE_MANIFEST_BASE_DIR" envDefault:"./manifests"`
}

// Controller related configurations
type ControllerConfig struct {
	//nolint:stylecheck
	// Request requeue time(seconds) in case the system still needs to be reconciled
	RequeueTimeSeconds time.Duration `env:"CONTROLLER_REQUEST_REQUEUE_SECONDS" envDefault:5`
}

func FromEnv() *OperatorConfig {
	once.Do(func() {
		operatorConfig = &OperatorConfig{}
		_ = env.Parse(operatorConfig)
	})
	return operatorConfig
}
