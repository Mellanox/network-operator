/*
2023 NVIDIA CORPORATION & AFFILIATES

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

// Package migrate handles one time migrations.
package migrate

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/network-operator/pkg/config"
)

// Migrator migrates from previous versions
type Migrator struct {
	K8sClient      client.Client
	MigrationCh    chan struct{}
	LeaderElection bool
	Logger         logr.Logger
}

// NeedLeaderElection implements manager.NeedLeaderElection
func (m *Migrator) NeedLeaderElection() bool {
	return m.LeaderElection
}

// Start implements manager.Runnable
func (m *Migrator) Start(ctx context.Context) error {
	err := Migrate(ctx, m.Logger, m.K8sClient)
	if err != nil {
		m.Logger.Error(err, "failed to migrate Network Operator")
		return err
	}
	close(m.MigrationCh)
	return nil
}

// Migrate contains logic which should run once during controller start.
// The main use case for this handler is network-operator upgrade
// for example, the handler can contain logic to change old data format to a new one or
// to remove unneeded resources from the cluster
func Migrate(ctx context.Context, log logr.Logger, c client.Client) error {
	if config.FromEnv().DisableMigration {
		log.Info("migration logic is disabled for the operator")
		return nil
	}
	return migrate(ctx, log, c)
}

func migrate(_ context.Context, _ logr.Logger, _ client.Client) error {
	// Add one-time migrations here.
	// Remove each one once the release it upgrades from is no longer in the supported upgrade path.
	return nil
}
