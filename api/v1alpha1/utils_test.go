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
package v1alpha1 //nolint:dupl

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//nolint:dupl
var _ = Describe("API utils tests", func() {
	Context("GetDriverUpgradePolicy tests", func() {
		It("should return nil when input is nil", func() {
			var input *DriverUpgradePolicySpec
			result := GetDriverUpgradePolicy(input)
			Expect(result).To(BeNil())
		})

		It("should retrieve DriverUpgradePolicy", func() {
			input := &DriverUpgradePolicySpec{
				AutoUpgrade:         true,
				MaxParallelUpgrades: 3,
				WaitForCompletion:   nil,
				DrainSpec:           nil,
			}
			result := GetDriverUpgradePolicy(input)
			Expect(result.AutoUpgrade).To(Equal(input.AutoUpgrade))
			Expect(result.MaxParallelUpgrades).To(Equal(input.MaxParallelUpgrades))
			Expect(result.WaitForCompletion).To(BeNil())
			Expect(result.DrainSpec).To(BeNil())
		})
	})

	Context("getWaitForCompletionSpec tests", func() {
		It("should return nil when input is nil", func() {
			var input *WaitForCompletionSpec
			result := getWaitForCompletionSpec(input)
			Expect(result).To(BeNil())
		})

		It("should retrieve WaitForCompletionSpec", func() {
			input := &WaitForCompletionSpec{
				PodSelector:   "app=myapp",
				TimeoutSecond: 300,
			}
			result := getWaitForCompletionSpec(input)
			Expect(result.PodSelector).To(Equal(input.PodSelector))
			Expect(result.TimeoutSecond).To(Equal(input.TimeoutSecond))
		})
	})

	Context("getDrainSpec tests", func() {
		It("should return nil when input is nil", func() {
			var input *DrainSpec
			result := getDrainSpec(input)
			Expect(result).To(BeNil())
		})

		It("should retrieve DrainSpec", func() {
			input := &DrainSpec{
				Enable:         true,
				Force:          true,
				PodSelector:    "app=myapp",
				TimeoutSecond:  300,
				DeleteEmptyDir: true,
			}
			result := getDrainSpec(input)
			Expect(result.Enable).To(Equal(input.Enable))
			Expect(result.Force).To(Equal(input.Force))
			Expect(result.PodSelector).To(Equal(input.PodSelector))
			Expect(result.TimeoutSecond).To(Equal(input.TimeoutSecond))
			Expect(result.DeleteEmptyDir).To(Equal(input.DeleteEmptyDir))
		})
	})
})
