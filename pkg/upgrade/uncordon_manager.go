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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
)

type UncordonManagerImpl struct {
	k8sInterface kubernetes.Interface
	log          logr.Logger
}

// UncordonManager is an interface that allows to uncordon nodes
type UncordonManager interface {
	CordonOrUncordonNode(ctx context.Context, node *corev1.Node, desired bool) error
}

func (m *UncordonManagerImpl) CordonOrUncordonNode(ctx context.Context, node *corev1.Node, desired bool) error {
	helper := &drain.Helper{Ctx: ctx, Client: m.k8sInterface}
	return drain.RunCordonOrUncordon(helper, node, desired)
}

func NewUncordonManager(k8sInterface kubernetes.Interface, log logr.Logger) *UncordonManagerImpl {
	return &UncordonManagerImpl{
		k8sInterface: k8sInterface,
		log:          log,
	}
}
