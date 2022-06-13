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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Mellanox/network-operator/pkg/consts"
)

// PodDeleteManagerImpl implements PodDeleteManager interface and can restart pods by deleting them
type PodDeleteManagerImpl struct {
	K8sClient client.Client

	Log logr.Logger
}

// PodDeleteManager is and interface that allows scheduling driver pod restarts
type PodDeleteManager interface {
	SchedulePodsRestart(context.Context, []*corev1.Pod) error
}

// SchedulePodsRestart receives a list of pods and schedules to delete them
func (m *PodDeleteManagerImpl) SchedulePodsRestart(ctx context.Context, pods []*corev1.Pod) error {
	m.Log.V(consts.LogLevelInfo).Info("Starting Pod Delete")
	if len(pods) == 0 {
		m.Log.V(consts.LogLevelInfo).Info("No pods scheduled to restart")
		return nil
	}
	for _, pod := range pods {
		m.Log.V(consts.LogLevelInfo).Info("Deleting pod", "pod", pod.Name)
		err := m.K8sClient.Delete(ctx, pod)
		if err != nil {
			m.Log.V(consts.LogLevelInfo).Error(err, "Failed to delete pod", "pod", pod.Name)
			return err
		}
	}
	return nil
}

func NewPodDeleteManager(k8sClient client.Client, log logr.Logger) *PodDeleteManagerImpl {
	mgr := &PodDeleteManagerImpl{K8sClient: k8sClient, Log: log}
	return mgr
}
