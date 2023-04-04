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

package v1alpha1

import upgradeApi "github.com/NVIDIA/k8s-operator-libs/api/upgrade/v1alpha1"

func GetDriverUpgradePolicy(
	ofedUpgradePolicy DriverUpgradePolicySpec) *upgradeApi.DriverUpgradePolicySpec {
	var driverUpgradePolicy upgradeApi.DriverUpgradePolicySpec
	driverUpgradePolicy.AutoUpgrade = ofedUpgradePolicy.AutoUpgrade
	driverUpgradePolicy.MaxParallelUpgrades = ofedUpgradePolicy.MaxParallelUpgrades

	driverUpgradePolicy.PodDeletion = nil
	if ofedUpgradePolicy.WaitForCompletion != nil {
		driverUpgradePolicy.WaitForCompletion = getWaitForCompletionSpec(ofedUpgradePolicy.WaitForCompletion)
	}
	if ofedUpgradePolicy.DrainSpec != nil {
		driverUpgradePolicy.DrainSpec = getDrainSpec(ofedUpgradePolicy.DrainSpec)
	}
	return &driverUpgradePolicy
}

func getWaitForCompletionSpec(
	waitForCompletionSpec *WaitForCompletionSpec) *upgradeApi.WaitForCompletionSpec {
	var spec upgradeApi.WaitForCompletionSpec
	spec.PodSelector = waitForCompletionSpec.PodSelector
	spec.TimeoutSecond = waitForCompletionSpec.TimeoutSecond
	return &spec
}

func getDrainSpec(drainSpec *DrainSpec) *upgradeApi.DrainSpec {
	var spec upgradeApi.DrainSpec
	spec.Enable = drainSpec.Enable
	spec.Force = drainSpec.Force
	spec.PodSelector = drainSpec.PodSelector
	spec.TimeoutSecond = drainSpec.TimeoutSecond
	spec.DeleteEmptyDir = drainSpec.DeleteEmptyDir
	return &spec
}
