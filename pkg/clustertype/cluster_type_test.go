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

package clustertype_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Mellanox/network-operator/pkg/clustertype"
)

func newFakeClientWrapper(err error) *fakeClientWrapper {
	return &fakeClientWrapper{
		Client:  fake.NewClientBuilder().Build(),
		listErr: err,
	}
}

type fakeClientWrapper struct {
	client.Client
	listErr error
}

func (f *fakeClientWrapper) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return f.listErr
}

var _ = Describe("cluster type Provider tests", func() {
	Context("Basic", func() {
		It("Kubernetes", func() {
			p, err := clustertype.NewProvider(context.Background(),
				newFakeClientWrapper(&meta.NoResourceMatchError{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(p.IsKubernetes()).To(BeTrue())
			Expect(p.GetClusterType()).To(Equal(clustertype.Kubernetes))
		})
		It("Openshift", func() {
			p, err := clustertype.NewProvider(context.Background(),
				newFakeClientWrapper(nil))
			Expect(err).NotTo(HaveOccurred())
			Expect(p.IsOpenshift()).To(BeTrue())
			Expect(p.GetClusterType()).To(Equal(clustertype.Openshift))
		})
		It("Error", func() {
			_, err := clustertype.NewProvider(context.Background(),
				newFakeClientWrapper(fmt.Errorf("test")))
			Expect(err).To(HaveOccurred())
		})
	})
})
