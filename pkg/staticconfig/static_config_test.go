/*
  2023 NVIDIA CORPORATION & AFFILIATES

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package staticconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StaticConfig Provider Tests", func() {

	Context("Create provider and retrieve static config parameters", func() {
		var expectedConfig StaticConfig
		var provider Provider

		BeforeEach(func() {
			expectedConfig = StaticConfig{
				CniBinDirectory: "/test/path",
			}
			provider = NewProvider(expectedConfig)
		})

		It("Should create a valid provider", func() {
			Expect(provider).ToNot(BeNil())
		})

		It("Should retrieve the correct static configuration", func() {
			actualConfig := provider.GetStaticConfig()
			Expect(actualConfig).To(Equal(expectedConfig))
		})
	})
})
