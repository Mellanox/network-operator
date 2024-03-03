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

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/network-operator/pkg/clustertype/mocks"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

var _ = Describe("Utils tests", func() {

	Context("Testing CniBinDirectory retrieval", func() {
		It("Should return user set directory", func() {
			userSetDir := "/user/set/directory"
			staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: userSetDir})
			clusterTypeProvider := mocks.Provider{}
			clusterTypeProvider.On("IsOpenshift").Return(false)
			result := GetCniBinDirectory(staticConfigProvider, &clusterTypeProvider)
			Expect(result).To(Equal(userSetDir))
		})

		It("Should return user set directory for OCP cluster", func() {
			userSetDir := "/user/set/directory"
			staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: userSetDir})
			clusterTypeProvider := mocks.Provider{}
			clusterTypeProvider.On("IsOpenshift").Return(true)
			result := GetCniBinDirectory(staticConfigProvider, &clusterTypeProvider)
			Expect(result).To(Equal(userSetDir))
		})

		It("Should return default Openshift directory for OCP cluster", func() {
			staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: ""})
			clusterTypeProvider := mocks.Provider{}
			clusterTypeProvider.On("IsOpenshift").Return(true)
			result := GetCniBinDirectory(staticConfigProvider, &clusterTypeProvider)
			Expect(result).To(Equal(consts.OcpCniBinDirectory))
		})

		It("Should return default K8s directory for non-OCP cluster", func() {
			staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: ""})
			clusterTypeProvider := mocks.Provider{}
			clusterTypeProvider.On("IsOpenshift").Return(false)
			result := GetCniBinDirectory(staticConfigProvider, &clusterTypeProvider)
			Expect(result).To(Equal(consts.DefaultCniBinDirectory))
		})

		It("Should return default K8s directory if cluster info is nil", func() {
			staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniBinDirectory: ""})
			result := GetCniBinDirectory(staticConfigProvider, nil)
			Expect(result).To(Equal(consts.DefaultCniBinDirectory))
		})
	})
})
