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
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
)

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

func migrate(ctx context.Context, log logr.Logger, c client.Client) error {
	if err := removeWhereaboutsIPReconcileCronJob(ctx, log, c); err != nil {
		// not critical for the operator operation, safer to ignore
		log.V(consts.LogLevelWarning).Info("ignore error during whereabouts CronJob removal")
	}
	if err := removeStateLabelFromNVIpamConfigMap(ctx, log, c); err != nil {
		// critical for the operator operation, will fail NVIPAM migration
		log.V(consts.LogLevelError).Error(err, "error trying to remove state label on NV IPAM configmap")
		return err
	}
	return nil
}

// reason: update whereabouts to version v0.6.1 in network-operator v23.4.0
// remove whereabouts-ip-reconciler CronJob from network-operator namespace
// IP reconciliation logic is now built in whereabouts, and we don't need to deploy CronJob separately.
// The network-operator will not deploy CronJob for new deployments anymore, and we also need to remove the job
// which were deployed by the previous Network-operator version.
func removeWhereaboutsIPReconcileCronJob(ctx context.Context, log logr.Logger, c client.Client) error {
	namespace := config.FromEnv().State.NetworkOperatorResourceNamespace
	cronJobName := "whereabouts-ip-reconciler"
	err := c.Delete(ctx, &v1.CronJob{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: cronJobName}})
	if err == nil {
		log.V(consts.LogLevelDebug).Info("whereabouts IP reconciler CronJob removed")
		return nil
	}
	if !apiErrors.IsNotFound(err) {
		log.V(consts.LogLevelError).Error(err, "failed to remove whereabouts IP reconciler CronJob")
		return err
	}
	return nil
}

// reason: remove state label on NV IPAM config map if exists, to allow migration to IPPool CR
// If the state label is present, the config map will be removed by the NCP Controller as a stale object
func removeStateLabelFromNVIpamConfigMap(ctx context.Context, log logr.Logger, c client.Client) error {
	namespace := config.FromEnv().State.NetworkOperatorResourceNamespace
	cmName := "nvidia-k8s-ipam-config"
	cfg := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Name:      cmName,
		Namespace: namespace,
	}
	err := c.Get(ctx, key, cfg)
	if apiErrors.IsNotFound(err) {
		log.V(consts.LogLevelDebug).Info("NVIPAM config map not found, skip remove state label")
		return nil
	} else if err != nil {
		log.V(consts.LogLevelError).Error(err, "fail to get NVIPAM configmap")
		return err
	}
	_, ok := cfg.Labels[consts.StateLabel]
	if ok {
		log.V(consts.LogLevelDebug).Info("clear State label from NVIPAM configmap")
		patch := []byte(fmt.Sprintf("[{\"op\": \"remove\", \"path\": \"/metadata/labels/%s\"}]", consts.StateLabel))
		err = c.Patch(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: namespace,
			},
		}, client.RawPatch(types.JSONPatchType, patch))
		if err != nil {
			log.V(consts.LogLevelError).Error(err, "fail to remove State label from NVIPAM configmap")
			return err
		}
	} else {
		log.V(consts.LogLevelDebug).Info("state label not set on NVIPAM configmap")
	}
	return nil
}
