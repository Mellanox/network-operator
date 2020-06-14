package config

import (
	"sync"

	"github.com/caarlos0/env/v6"
)

var once sync.Once
var operatorConfig *OperatorConfig

// Operator related configurations
type OperatorConfig struct {
	State StateConfig
}

// state related configurations
type StateConfig struct {
	ManifestBaseDir string `env:"STATE_MANIFEST_BASE_DIR" envDefault:"./manifests"`
}

func FromEnv() *OperatorConfig {
	once.Do(func() {
		operatorConfig = &OperatorConfig{}
		_ = env.Parse(operatorConfig)
	})
	return operatorConfig
}
