/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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

package controllers

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func nodeWithReadyStatus(status corev1.ConditionStatus) *corev1.Node {
	node := &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{}}}
	if status != "" {
		node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}
	}
	return node
}

var _ = Describe("NodeReadyChangedPredicate", func() {
	p := NodeReadyChangedPredicate{}

	It("accepts only changes to Ready status or condition presence", func() {
		statuses := []corev1.ConditionStatus{"", corev1.ConditionTrue, corev1.ConditionFalse, corev1.ConditionUnknown}
		for _, oldStatus := range statuses {
			for _, newStatus := range statuses {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: nodeWithReadyStatus(oldStatus), ObjectNew: nodeWithReadyStatus(newStatus),
				})).To(Equal(oldStatus != newStatus), "Ready %q -> %q", oldStatus, newStatus)
			}
		}
	})

	It("ignores heartbeat, reason, metadata, and other condition changes", func() {
		old := nodeWithReadyStatus(corev1.ConditionTrue)
		updated := old.DeepCopy()
		updated.ResourceVersion = "2"
		updated.Labels = map[string]string{"unrelated": "value"}
		updated.Status.Conditions[0].LastHeartbeatTime = metav1.NewTime(time.Now())
		updated.Status.Conditions[0].Reason = "Heartbeat"
		updated.Status.Conditions = append(updated.Status.Conditions, corev1.NodeCondition{
			Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue,
		})
		Expect(p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeFalse())
	})

	It("rejects missing or non-node event objects", func() {
		var nilNode *corev1.Node
		for _, obj := range []client.Object{nil, nilNode, &corev1.Pod{}} {
			Expect(p.Update(event.UpdateEvent{ObjectOld: obj, ObjectNew: nodeWithReadyStatus(corev1.ConditionTrue)})).
				To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectOld: nodeWithReadyStatus(corev1.ConditionTrue), ObjectNew: obj})).
				To(BeFalse())
		}
	})

	It("does not broaden create, delete, or generic event handling", func() {
		node := nodeWithReadyStatus(corev1.ConditionTrue)
		Expect(p.Create(event.CreateEvent{Object: node})).To(BeFalse())
		Expect(p.Delete(event.DeleteEvent{Object: node})).To(BeFalse())
		Expect(p.Generic(event.GenericEvent{Object: node})).To(BeFalse())
	})
})
