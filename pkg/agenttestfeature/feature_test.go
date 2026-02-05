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

package agenttestfeature_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/network-operator/pkg/agenttestfeature"
)

var _ = Describe("Agent Test Feature", func() {
	Context("TestConstant", func() {
		It("should have the expected value", func() {
			Expect(agenttestfeature.TestConstant).To(Equal("agent-test"))
		})
	})

	Context("GetTestConstant", func() {
		It("should return the TestConstant value", func() {
			result := agenttestfeature.GetTestConstant()
			Expect(result).To(Equal("agent-test"))
		})

		It("should return a non-empty string", func() {
			result := agenttestfeature.GetTestConstant()
			Expect(result).NotTo(BeEmpty())
		})
	})
})
