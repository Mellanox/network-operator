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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
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

func migrate(ctx context.Context, log logr.Logger, c client.Client) error {
	if err := handleSingleMofedDS(ctx, log, c); err != nil {
		// critical for the operator operation, will fail Mofed migration
		log.V(consts.LogLevelError).Error(err, "error trying to handle single MOFED DS")
		return err
	}
	return nil
}

func handleSingleMofedDS(ctx context.Context, log logr.Logger, c client.Client) error {
	ncp := &mellanoxv1alpha1.NicClusterPolicy{}
	key := types.NamespacedName{
		Name: consts.NicClusterPolicyResourceName,
	}
	err := c.Get(ctx, key, ncp)
	if apiErrors.IsNotFound(err) {
		log.V(consts.LogLevelDebug).Info("NIC ClusterPolicy not found, skip handling single MOFED DS")
		return nil
	} else if err != nil {
		return err
	}
	if ncp.Spec.OFEDDriver == nil {
		return nil
	}
	log.V(consts.LogLevelDebug).Info("Searching for single MOFED DS")
	dsList := &appsv1.DaemonSetList{}
	err = c.List(ctx, dsList, client.MatchingLabels{"nvidia.com/ofed-driver": ""})
	if err != nil {
		log.V(consts.LogLevelError).Error(err, "fail to list MOFED DS")
		return err
	}
	var ds *appsv1.DaemonSet
	for i := range dsList.Items {
		mofedDs := &dsList.Items[i]
		// The single MOFED DS does not contain the label "mofed-ds-format-version"
		_, ok := mofedDs.Labels["mofed-ds-format-version"]
		if ok {
			continue
		}
		log.V(consts.LogLevelDebug).Info("Found single MOFED DS", "name", mofedDs.Name)
		ds = mofedDs
		break
	}
	if ds != nil {
		err = markNodesAsUpgradeRequested(ctx, log, c, ds)
		if err != nil {
			return err
		}
		policy := metav1.DeletePropagationOrphan
		err = c.Delete(ctx, ds, &client.DeleteOptions{PropagationPolicy: &policy})
		if err != nil {
			return err
		}
		log.V(consts.LogLevelDebug).Info("Deleted single MOFED DS with orphaned", "name", ds.Name)
		return nil
	}
	log.V(consts.LogLevelDebug).Info("Single MOFED DS not found")
	return nil
}

func markNodesAsUpgradeRequested(ctx context.Context, log logr.Logger, c client.Client, ds *appsv1.DaemonSet) error {
	nodes, err := getDaemonSetNodes(ctx, c, ds)
	if err != nil {
		return err
	}
	for _, nodeName := range nodes {
		node := &corev1.Node{}
		nodeKey := client.ObjectKey{
			Name: nodeName,
		}
		if err := c.Get(context.Background(), nodeKey, node); err != nil {
			return err
		}
		patchString := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q: "true"}}}`,
			upgrade.GetUpgradeRequestedAnnotationKey()))
		patch := client.RawPatch(types.MergePatchType, patchString)
		err = c.Patch(ctx, node, patch)
		if err != nil {
			return err
		}
		log.V(consts.LogLevelDebug).Info("Node annotated with upgrade-requested", "name", nodeName)
	}

	return nil
}

func getDaemonSetNodes(ctx context.Context, c client.Client, ds *appsv1.DaemonSet) ([]string, error) {
	nodeNames := make([]string, 0)
	pods := &corev1.PodList{}
	err := c.List(ctx, pods, client.MatchingLabels(ds.Spec.Selector.MatchLabels))
	if err != nil {
		return nil, err
	}
	for i := range pods.Items {
		nodeName := pods.Items[i].Spec.NodeName
		if nodeName != "" {
			nodeNames = append(nodeNames, nodeName)
		}
	}
	return nodeNames, nil
}
