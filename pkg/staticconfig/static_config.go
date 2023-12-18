/*
----------------------------------------------------

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

----------------------------------------------------
*/

package staticconfig

/*
 static package provides static cluster information, required by network-operator
*/

type StaticConfig struct {
	CniBinDirectory string
}

// Provider provides static cluster attributes
type Provider interface {
	GetStaticConfig() StaticConfig
}

// NewProvider creates a new Provider object
func NewProvider(staticConfig StaticConfig) Provider {
	return &provider{staticConfig}
}

// provider is an implementation of the Provider interface
type provider struct {
	staticConfig StaticConfig
}

func (p *provider) GetStaticConfig() StaticConfig {
	return p.staticConfig
}