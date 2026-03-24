/*
Copyright 2025 NVIDIA

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

package conditions_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/conditions"
	"github.com/Mellanox/network-operator/pkg/state"
)

func TestConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conditions Suite")
}

// generation used in all test cases
const testGeneration int64 = 5

// ── helpers ──────────────────────────────────────────────────────────────────

func resultReady(name string) state.Result {
	return state.Result{StateName: name, Status: state.SyncStateReady}
}

func resultNotReady(name string) state.Result {
	return state.Result{StateName: name, Status: state.SyncStateNotReady}
}

func resultNotReadyWithErr(name string, err error) state.Result {
	return state.Result{StateName: name, Status: state.SyncStateNotReady, ErrInfo: err}
}

func resultIgnore(name string) state.Result {
	return state.Result{StateName: name, Status: state.SyncStateIgnore}
}

func resultError(name string, err error) state.Result {
	return state.Result{StateName: name, Status: state.SyncStateError, ErrInfo: err}
}

func findCondition(conds []metav1.Condition, condType string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == condType {
			return &conds[i]
		}
	}
	return nil
}

// ── ComponentConditionFromResult ─────────────────────────────────────────────

var _ = Describe("ComponentConditionFromResult", func() {
	Context("when state name is not recognized", func() {
		It("returns false and a zero condition", func() {
			cond, ok := conditions.ComponentConditionFromResult(
				state.Result{StateName: "state-unknown-thing", Status: state.SyncStateReady},
				testGeneration,
			)
			Expect(ok).To(BeFalse())
			Expect(cond).To(Equal(metav1.Condition{}))
		})
	})

	Context("when SyncStateReady", func() {
		It("returns True / ComponentReady with empty message", func() {
			cond, ok := conditions.ComponentConditionFromResult(resultReady("state-OFED"), testGeneration)
			Expect(ok).To(BeTrue())
			Expect(cond.Type).To(Equal(mellanoxv1alpha1.ConditionTypeOFEDDriverReady))
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentReady))
			Expect(cond.Message).To(BeEmpty())
			Expect(cond.ObservedGeneration).To(Equal(testGeneration))
		})
	})

	Context("when SyncStateIgnore (component not in spec)", func() {
		It("returns True / ComponentNotRequired", func() {
			cond, ok := conditions.ComponentConditionFromResult(resultIgnore("state-nv-ipam-cni"), testGeneration)
			Expect(ok).To(BeTrue())
			Expect(cond.Type).To(Equal(mellanoxv1alpha1.ConditionTypeNVIPAMReady))
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentNotRequired))
			Expect(cond.Message).NotTo(BeEmpty())
		})
	})

	Context("when SyncStateNotReady without an error", func() {
		It("returns False / ComponentNotReady with empty message", func() {
			cond, ok := conditions.ComponentConditionFromResult(resultNotReady("state-multus-cni"), testGeneration)
			Expect(ok).To(BeTrue())
			Expect(cond.Type).To(Equal(mellanoxv1alpha1.ConditionTypeMultusCNIReady))
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentNotReady))
			Expect(cond.Message).To(BeEmpty())
		})
	})

	Context("when SyncStateNotReady with an error", func() {
		It("returns False / ComponentNotReady with error message", func() {
			err := fmt.Errorf("daemonset not available yet")
			cond, ok := conditions.ComponentConditionFromResult(
				resultNotReadyWithErr("state-RDMA-device-plugin", err), testGeneration)
			Expect(ok).To(BeTrue())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentNotReady))
			Expect(cond.Message).To(Equal(err.Error()))
		})
	})

	Context("when SyncStateError without error info", func() {
		It("returns False / ComponentError with empty message", func() {
			cond, ok := conditions.ComponentConditionFromResult(
				state.Result{StateName: "state-SRIOV-device-plugin", Status: state.SyncStateError},
				testGeneration,
			)
			Expect(ok).To(BeTrue())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentError))
			Expect(cond.Message).To(BeEmpty())
		})
	})

	Context("when SyncStateError with error info", func() {
		It("returns False / ComponentError with the error message", func() {
			err := fmt.Errorf("image pull failed")
			cond, ok := conditions.ComponentConditionFromResult(resultError("state-ib-kubernetes", err), testGeneration)
			Expect(ok).To(BeTrue())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentError))
			Expect(cond.Message).To(Equal(err.Error()))
		})
	})

	Context("when an unrecognized SyncState is returned", func() {
		It("returns Unknown / ComponentNotReady", func() {
			cond, ok := conditions.ComponentConditionFromResult(
				state.Result{StateName: "state-doca-telemetry-service", Status: "reset"},
				testGeneration,
			)
			Expect(ok).To(BeTrue())
			Expect(cond.Status).To(Equal(metav1.ConditionUnknown))
			Expect(cond.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentNotReady))
		})
	})

	Context("verifying all known state names are mapped", func() {
		knownStates := []struct {
			stateName     string
			expectedCType string
		}{
			{"state-OFED", mellanoxv1alpha1.ConditionTypeOFEDDriverReady},
			{"state-RDMA-device-plugin", mellanoxv1alpha1.ConditionTypeRDMASharedDevicePluginReady},
			{"state-SRIOV-device-plugin", mellanoxv1alpha1.ConditionTypeSRIOVDevicePluginReady},
			{"state-ib-kubernetes", mellanoxv1alpha1.ConditionTypeIBKubernetesReady},
			{"state-multus-cni", mellanoxv1alpha1.ConditionTypeMultusCNIReady},
			{"state-container-networking-plugins", mellanoxv1alpha1.ConditionTypeCNIPluginsReady},
			{"state-ipoib-cni", mellanoxv1alpha1.ConditionTypeIPoIBCNIReady},
			{"state-nv-ipam-cni", mellanoxv1alpha1.ConditionTypeNVIPAMReady},
			{"state-nic-feature-discovery", mellanoxv1alpha1.ConditionTypeNICFeatureDiscoveryReady},
			{"state-doca-telemetry-service", mellanoxv1alpha1.ConditionTypeDOCATelemetryServiceReady},
			{"state-nic-configuration-operator", mellanoxv1alpha1.ConditionTypeNICConfigurationOperatorReady},
			{"state-spectrum-x-operator", mellanoxv1alpha1.ConditionTypeSpectrumXOperatorReady},
		}
		for _, tc := range knownStates {
			It(fmt.Sprintf("maps %s → %s", tc.stateName, tc.expectedCType), func() {
				cond, ok := conditions.ComponentConditionFromResult(resultReady(tc.stateName), testGeneration)
				Expect(ok).To(BeTrue(), "state name %q not in stateNameToConditionType map", tc.stateName)
				Expect(cond.Type).To(Equal(tc.expectedCType))
			})
		}
	})
})

// ── UpdateConditions ─────────────────────────────────────────────────────────

var _ = Describe("UpdateConditions", func() {
	Context("when all components are ready", func() {
		It("sets all per-component conditions to True and all aggregates healthy", func() {
			results := state.Results{
				Status: state.SyncStateReady,
				StatesStatus: []state.Result{
					resultReady("state-OFED"),
					resultReady("state-multus-cni"),
					resultReady("state-nv-ipam-cni"),
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			By("checking per-component conditions")
			for _, condType := range []string{
				mellanoxv1alpha1.ConditionTypeOFEDDriverReady,
				mellanoxv1alpha1.ConditionTypeMultusCNIReady,
				mellanoxv1alpha1.ConditionTypeNVIPAMReady,
			} {
				c := findCondition(conds, condType)
				Expect(c).NotTo(BeNil(), "condition %s not found", condType)
				Expect(c.Status).To(Equal(metav1.ConditionTrue))
				Expect(c.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentReady))
			}

			By("checking aggregate Ready=True")
			ready := findCondition(conds, mellanoxv1alpha1.ConditionTypeReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			Expect(ready.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonAllComponentsReady))

			By("checking aggregate Progressing=False")
			prog := findCondition(conds, mellanoxv1alpha1.ConditionTypeProgressing)
			Expect(prog).NotTo(BeNil())
			Expect(prog.Status).To(Equal(metav1.ConditionFalse))
			Expect(prog.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonIdle))

			By("checking aggregate Degraded=False")
			deg := findCondition(conds, mellanoxv1alpha1.ConditionTypeDegraded)
			Expect(deg).NotTo(BeNil())
			Expect(deg.Status).To(Equal(metav1.ConditionFalse))
			Expect(deg.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonHealthy))
		})
	})

	Context("when one component is not configured (SyncStateIgnore)", func() {
		It("sets that component to True/ComponentNotRequired and aggregates remain healthy", func() {
			results := state.Results{
				Status: state.SyncStateReady,
				StatesStatus: []state.Result{
					resultReady("state-OFED"),
					resultIgnore("state-nv-ipam-cni"),
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			ipam := findCondition(conds, mellanoxv1alpha1.ConditionTypeNVIPAMReady)
			Expect(ipam).NotTo(BeNil())
			Expect(ipam.Status).To(Equal(metav1.ConditionTrue))
			Expect(ipam.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentNotRequired))

			ready := findCondition(conds, mellanoxv1alpha1.ConditionTypeReady)
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("when one component is still deploying (SyncStateNotReady)", func() {
		It("sets Progressing=True and Ready=False, Degraded=False", func() {
			results := state.Results{
				Status: state.SyncStateNotReady,
				StatesStatus: []state.Result{
					resultReady("state-OFED"),
					resultNotReady("state-multus-cni"),
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			multus := findCondition(conds, mellanoxv1alpha1.ConditionTypeMultusCNIReady)
			Expect(multus.Status).To(Equal(metav1.ConditionFalse))
			Expect(multus.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentNotReady))

			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeReady).Status).To(Equal(metav1.ConditionFalse))
			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeProgressing).Status).To(Equal(metav1.ConditionTrue))
			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeDegraded).Status).To(Equal(metav1.ConditionFalse))
		})

		It("includes the deploying component name in the Progressing message", func() {
			results := state.Results{
				StatesStatus: []state.Result{
					resultNotReady("state-multus-cni"),
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			prog := findCondition(conds, mellanoxv1alpha1.ConditionTypeProgressing)
			Expect(prog.Message).To(ContainSubstring(mellanoxv1alpha1.ConditionTypeMultusCNIReady))
		})
	})

	Context("when one component has an error (SyncStateError)", func() {
		It("sets Degraded=True and Ready=False, Progressing=False", func() {
			err := fmt.Errorf("template render failed")
			results := state.Results{
				Status: state.SyncStateNotReady,
				StatesStatus: []state.Result{
					resultReady("state-OFED"),
					resultError("state-nv-ipam-cni", err),
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			ipam := findCondition(conds, mellanoxv1alpha1.ConditionTypeNVIPAMReady)
			Expect(ipam.Status).To(Equal(metav1.ConditionFalse))
			Expect(ipam.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentError))
			Expect(ipam.Message).To(Equal(err.Error()))

			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeReady).Status).To(Equal(metav1.ConditionFalse))
			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeProgressing).Status).To(Equal(metav1.ConditionFalse))

			deg := findCondition(conds, mellanoxv1alpha1.ConditionTypeDegraded)
			Expect(deg.Status).To(Equal(metav1.ConditionTrue))
			Expect(deg.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentError))
			Expect(deg.Message).To(ContainSubstring(err.Error()))
		})
	})

	Context("when both notReady and error components coexist", func() {
		It("sets Degraded=True, Progressing=True, Ready=False with ComponentError reason on Ready", func() {
			results := state.Results{
				StatesStatus: []state.Result{
					resultNotReady("state-multus-cni"),
					resultError("state-nv-ipam-cni", fmt.Errorf("boom")),
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			// Error takes priority on Ready reason
			ready := findCondition(conds, mellanoxv1alpha1.ConditionTypeReady)
			Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			Expect(ready.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentError))

			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeProgressing).Status).To(Equal(metav1.ConditionTrue))
			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeDegraded).Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("when a component has Unknown status (unrecognized SyncState)", func() {
		It("treats it as unhealthy: Ready=False, Progressing=True, Degraded=False", func() {
			results := state.Results{
				StatesStatus: []state.Result{
					resultReady("state-OFED"),
					{StateName: "state-multus-cni", Status: "some-unexpected-state"},
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			multus := findCondition(conds, mellanoxv1alpha1.ConditionTypeMultusCNIReady)
			Expect(multus).NotTo(BeNil())
			Expect(multus.Status).To(Equal(metav1.ConditionUnknown))
			Expect(multus.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonComponentNotReady))

			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeReady).Status).To(Equal(metav1.ConditionFalse))
			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeProgressing).Status).To(Equal(metav1.ConditionTrue))
			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeDegraded).Status).To(Equal(metav1.ConditionFalse))
		})
	})

	Context("when a result has an unrecognized state name", func() {
		It("silently skips it without panicking", func() {
			results := state.Results{
				StatesStatus: []state.Result{
					resultReady("state-OFED"),
					resultReady("state-unknown-component"),
				},
			}
			var conds []metav1.Condition
			Expect(func() {
				conditions.UpdateConditions(&conds, results, testGeneration)
			}).NotTo(Panic())

			// Only the known component should appear as a per-component condition
			Expect(findCondition(conds, mellanoxv1alpha1.ConditionTypeOFEDDriverReady)).NotTo(BeNil())
		})
	})

	Context("when called twice (idempotency)", func() {
		It("does not duplicate conditions and only updates LastTransitionTime on status change", func() {
			results := state.Results{
				StatesStatus: []state.Result{resultReady("state-OFED")},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)
			firstTransition := findCondition(conds, mellanoxv1alpha1.ConditionTypeOFEDDriverReady).LastTransitionTime

			// Call again with same results — status unchanged
			conditions.UpdateConditions(&conds, results, testGeneration)
			Expect(conds).To(HaveLen(1 + 3)) // 1 component + 3 aggregates

			secondTransition := findCondition(conds, mellanoxv1alpha1.ConditionTypeOFEDDriverReady).LastTransitionTime
			Expect(secondTransition).To(Equal(firstTransition), "LastTransitionTime must not change when status is unchanged")
		})

		It("updates LastTransitionTime when status flips", func() {
			ready := state.Results{StatesStatus: []state.Result{resultReady("state-OFED")}}
			notReady := state.Results{StatesStatus: []state.Result{resultNotReady("state-OFED")}}

			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, ready, testGeneration)
			firstTransition := findCondition(conds, mellanoxv1alpha1.ConditionTypeOFEDDriverReady).LastTransitionTime

			conditions.UpdateConditions(&conds, notReady, testGeneration)
			secondTransition := findCondition(conds, mellanoxv1alpha1.ConditionTypeOFEDDriverReady).LastTransitionTime

			Expect(secondTransition).NotTo(Equal(firstTransition), "LastTransitionTime must update when status changes")
		})
	})

	Context("ObservedGeneration", func() {
		It("is set on every condition", func() {
			results := state.Results{
				StatesStatus: []state.Result{
					resultReady("state-OFED"),
					resultNotReady("state-nv-ipam-cni"),
				},
			}
			var conds []metav1.Condition
			conditions.UpdateConditions(&conds, results, testGeneration)

			for _, c := range conds {
				Expect(c.ObservedGeneration).To(Equal(testGeneration),
					"condition %s has wrong ObservedGeneration", c.Type)
			}
		})
	})
})

// ── SetPolicyNotSupportedConditions ──────────────────────────────────────────

var _ = Describe("SetPolicyNotSupportedConditions", func() {
	It("sets Ready=False, Progressing=False, Degraded=True all with PolicyNotSupported reason", func() {
		var conds []metav1.Condition
		msg := "Unsupported NicClusterPolicy instance foo. Only instance with name nic-cluster-policy is supported"
		conditions.SetPolicyNotSupportedConditions(&conds, testGeneration, msg)

		ready := findCondition(conds, mellanoxv1alpha1.ConditionTypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonPolicyNotSupported))
		Expect(ready.Message).To(Equal(msg))

		prog := findCondition(conds, mellanoxv1alpha1.ConditionTypeProgressing)
		Expect(prog).NotTo(BeNil())
		Expect(prog.Status).To(Equal(metav1.ConditionFalse))
		Expect(prog.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonIdle))

		deg := findCondition(conds, mellanoxv1alpha1.ConditionTypeDegraded)
		Expect(deg).NotTo(BeNil())
		Expect(deg.Status).To(Equal(metav1.ConditionTrue))
		Expect(deg.Reason).To(Equal(mellanoxv1alpha1.ConditionReasonPolicyNotSupported))
		Expect(deg.Message).To(Equal(msg))
	})

	It("sets ObservedGeneration on all conditions", func() {
		var conds []metav1.Condition
		conditions.SetPolicyNotSupportedConditions(&conds, testGeneration, "reason")
		for _, c := range conds {
			Expect(c.ObservedGeneration).To(Equal(testGeneration))
		}
	})
})
