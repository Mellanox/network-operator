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
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/network-operator/pkg/consts"
)

// NodeUpgradeStateProvider allows for synchronized operations on node objects and ensures that the node,
// got from the provider, always has the up-to-date upgrade state
type NodeUpgradeStateProvider interface {
	GetNode(ctx context.Context, nodeName string) (*v1.Node, error)
	ChangeNodeUpgradeState(ctx context.Context, node *v1.Node, newNodeState string) error
}

type NodeUpgradeStateProviderImpl struct {
	K8sClient client.Client
	Log       logr.Logger
	nodeMutex KeyedMutex
}

func NewNodeUpgradeStateProvider(k8sClient client.Client, log logr.Logger) NodeUpgradeStateProvider {
	return &NodeUpgradeStateProviderImpl{
		K8sClient: k8sClient,
		Log:       log,
		nodeMutex: KeyedMutex{},
	}
}

func (p *NodeUpgradeStateProviderImpl) GetNode(ctx context.Context, nodeName string) (*v1.Node, error) {
	defer p.nodeMutex.Lock(nodeName)()

	node := v1.Node{}
	err := p.K8sClient.Get(ctx, types.NamespacedName{Name: nodeName}, &node)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// ChangeNodeUpgradeState patches a given v1.Node object and updates its UpgradeStateAnnotation with a given value
// The function then waits for the operator cache to get updated
func (p *NodeUpgradeStateProviderImpl) ChangeNodeUpgradeState(
	ctx context.Context, node *v1.Node, newNodeState string) error {
	p.Log.V(consts.LogLevelInfo).Info("Updating node upgrade state",
		"node", node.Name,
		"new state", newNodeState)

	defer p.nodeMutex.Lock(node.Name)()

	patchString := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s": "%s"}}}`, UpgradeStateAnnotation, newNodeState))
	patch := client.RawPatch(types.StrategicMergePatchType, patchString)
	err := p.K8sClient.Patch(ctx, node, patch)
	if err != nil {
		p.Log.V(consts.LogLevelError).Error(err, "Failed to patch node state annotation on a node object",
			"node", node,
			"state", newNodeState)
		return err
	}

	// Upgrade controller is watching on a set of different resources (NicClusterPolicy, DaemonSet, Pods)
	// Because of that, when a new Reconcile event is triggered, the operator cache might not have the latest changes
	// For example, the node object might have a different upgrade-state value even though it was just changed here.
	// To fix that problem, after the state of the node has successfully been changed, we poll the same node object
	// until its state matches the newly changed one. Get request in that case takes objects from the operator cache,
	// so we wait until it's synced.
	// That way, since only one call to reconcile at a time is allowed for upgrade controller, each new update
	// will have the updated node object in the cache.
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	err = wait.PollImmediateUntil(time.Second, func() (bool, error) {
		p.Log.V(consts.LogLevelDebug).Info("Requesting node object to see if operator cache has updated",
			"node", node.Name)
		err := p.K8sClient.Get(timeoutCtx, types.NamespacedName{Name: node.Name}, node)
		if err != nil {
			return false, err
		}
		nodeState := node.Annotations[UpgradeStateAnnotation]
		if nodeState != newNodeState {
			p.Log.V(consts.LogLevelDebug).Info("upgrade state annotation for node doesn't match the expected",
				"node", node.Name, "expected", newNodeState, "actual", nodeState)
			return false, nil
		}
		return true, nil
	}, timeoutCtx.Done())

	if err != nil {
		p.Log.V(consts.LogLevelError).Error(err, "Error while waiting on node annotation update",
			"node", node,
			"state", newNodeState)
	} else {
		p.Log.V(consts.LogLevelInfo).Info("Successfully changed node upgrade state annotation",
			"node", node.Name,
			"new state", newNodeState)
	}

	return err
}
