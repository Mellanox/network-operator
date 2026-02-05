/*
 2024 NVIDIA CORPORATION & AFFILIATES
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

// Package agenttestfeature provides a simple test constant and getter
// for validating the autonomous agent workflow.
package agenttestfeature

// TestConstant is a test constant for validating agent functionality.
const TestConstant = "agent-test"

// GetTestConstant returns the TestConstant value.
func GetTestConstant() string {
	return TestConstant
}
