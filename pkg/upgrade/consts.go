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

const (
	UpgradeStateAnnotation = "nvidia.com/ofed-upgrade-state"

	OfedDriverLabel = "nvidia.com/ofed-driver"

	// UpgradeStateUnknown Node has this state when the upgrade flow is disabled or the node hasn't been processed yet
	UpgradeStateUnknown = ""
	// UpgradeStateDone is set when OFED POD is up to date and running on the node, the node is schedulable
	UpgradeStateDone = "upgrade-done"
	// UpgradeStateUpgradeRequired is set when OFED POD on the node is not up-to-date and required upgrade
	// No actions are performed at this stage
	UpgradeStateUpgradeRequired = "upgrade-required"
	// UpgradeStateDrain is set when the node is scheduled for drain. After the drain the state is changed
	// either to UpgradeStatePodRestart or UpgradeStateDrainFailed
	UpgradeStateDrain = "drain"
	// UpgradeStatePodRestart is set when the OFED POD on the node is scheduler for restart.
	// After the restart state is changed to UpgradeStateDone
	UpgradeStatePodRestart = "pod-restart"
	// UpgradeStateDrainFailed is set when drain on the node has failed. Manual interaction is required at this stage.
	UpgradeStateDrainFailed = "drain-failed"
	// UpgradeStateUncordonRequired is set when OFED POD on the node is up-to-date and has "Ready" status
	UpgradeStateUncordonRequired = "uncordon-required"
)
