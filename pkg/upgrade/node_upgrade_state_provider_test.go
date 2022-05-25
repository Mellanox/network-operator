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

package upgrade_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/network-operator/pkg/upgrade"
)

var _ = Describe("NodeUpgradeStateProvider tests", func() {
	It("NodeUpgradeStateProvider should change node upgrade state and retrieve the latest node object", func() {
		ctx := context.TODO()
		node := createNode("test-node")

		provider := upgrade.NewNodeUpgradeStateProvider(k8sClient, log)

		err := provider.ChangeNodeUpgradeState(ctx, node, upgrade.UpgradeStateUpgradeRequired)
		Expect(err).To(Succeed())

		node, err = provider.GetNode(ctx, node.Name)
		Expect(err).To(Succeed())
		Expect(node.Annotations[upgrade.UpgradeStateAnnotation]).To(Equal(upgrade.UpgradeStateUpgradeRequired))
	})
})
