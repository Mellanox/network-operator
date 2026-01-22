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

	Context("Testing CniNetworkDirectory retrieval", func() {
		It("Should return user set directory when configured", func() {
			userSetDir := "/user/set/network/directory"
			staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniNetworkDirectory: userSetDir})
			result := GetCniNetworkDirectory(staticConfigProvider, nil)
			Expect(result).To(Equal(userSetDir))
		})

		It("Should return default directory when no user set directory is configured", func() {
			staticConfigProvider := staticconfig.NewProvider(staticconfig.StaticConfig{CniNetworkDirectory: ""})
			result := GetCniNetworkDirectory(staticConfigProvider, nil)
			Expect(result).To(Equal(consts.DefaultCniNetworkDirectory))
		})
	})

	Context("Testing GetStringHash", func() {
		It("Should return consistent hash for same input", func() {
			input := `{"resourceName": "rdma_shared_device_a"}`
			hash1 := GetStringHash(input)
			hash2 := GetStringHash(input)
			Expect(hash1).To(Equal(hash2))
		})

		It("Should return different hash for different input", func() {
			input1 := `{"resourceName": "rdma_shared_device_a"}`
			input2 := `{"resourceName": "rdma_shared_device_b"}`
			hash1 := GetStringHash(input1)
			hash2 := GetStringHash(input2)
			Expect(hash1).NotTo(Equal(hash2))
		})

		It("Should return non-empty hash for empty string", func() {
			hash := GetStringHash("")
			Expect(hash).NotTo(BeEmpty())
		})

		It("Should return valid base64 encoded string", func() {
			input := `{"config": "test"}`
			hash := GetStringHash(input)
			// Base64 encoded SHA256 hash should be 44 characters
			Expect(len(hash)).To(Equal(44))
		})
	})
})
