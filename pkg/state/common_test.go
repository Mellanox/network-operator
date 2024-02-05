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

package state_test

import (
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

type testProvider struct {
	isOpenshift bool
	cniBinDir   string
}

func (tp *testProvider) GetClusterType() clustertype.Type {
	if tp.isOpenshift {
		return clustertype.Openshift
	}
	return clustertype.Kubernetes
}

func (tp *testProvider) IsKubernetes() bool {
	return !tp.isOpenshift
}

func (tp *testProvider) IsOpenshift() bool {
	return tp.isOpenshift
}

func (tp *testProvider) GetStaticConfig() staticconfig.StaticConfig {
	return staticconfig.StaticConfig{CniBinDirectory: tp.cniBinDir}
}

func (tp *testProvider) GetNodesAttributes(...nodeinfo.Filter) []nodeinfo.NodeAttributes {
	nodeAttr := make(map[nodeinfo.AttributeType]string)
	nodeAttr[nodeinfo.AttrTypeCPUArch] = "amd64"
	nodeAttr[nodeinfo.AttrTypeOSName] = "ubuntu"
	nodeAttr[nodeinfo.AttrTypeOSVer] = "20.04"

	return []nodeinfo.NodeAttributes{{Attributes: nodeAttr}}
}

func getTestCatalog() state.InfoCatalog {
	catalog := state.NewInfoCatalog()
	tp := &testProvider{isOpenshift: false, cniBinDir: ""}
	catalog.Add(state.InfoTypeNodeInfo, tp)
	catalog.Add(state.InfoTypeStaticConfig, tp)
	catalog.Add(state.InfoTypeClusterType, tp)

	return catalog
}
