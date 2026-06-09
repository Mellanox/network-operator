/*
2026 NVIDIA CORPORATION & AFFILIATES

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

package docadriverimages

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

func TestDocaDriverImages(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "docadriverimages Suite")
}

var _ = Describe("isRetryableRegistryError", func() {
	It("returns false for nil error", func() {
		Expect(isRetryableRegistryError(nil)).To(BeFalse())
	})

	It("returns true for dial tcp i/o timeout", func() {
		err := fmt.Errorf(`failed to list tags: Get "https://registry:443/v2/": dial tcp 10.0.0.1:443: i/o timeout`)
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})

	It("returns true for transport 503 errors", func() {
		err := &transport.Error{StatusCode: http.StatusServiceUnavailable}
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})

	It("returns true for transport 429 errors", func() {
		err := &transport.Error{StatusCode: http.StatusTooManyRequests}
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})

	It("returns false for transport 401 errors", func() {
		err := &transport.Error{
			StatusCode: http.StatusUnauthorized,
			Errors:     []transport.Diagnostic{{Code: transport.UnauthorizedErrorCode}},
		}
		Expect(isRetryableRegistryError(err)).To(BeFalse())
	})

	It("returns false for transport 403 errors", func() {
		err := &transport.Error{
			StatusCode: http.StatusForbidden,
			Errors:     []transport.Diagnostic{{Code: transport.DeniedErrorCode}},
		}
		Expect(isRetryableRegistryError(err)).To(BeFalse())
	})

	It("returns false for transport 404 NAME_UNKNOWN errors", func() {
		err := &transport.Error{
			StatusCode: http.StatusNotFound,
			Errors:     []transport.Diagnostic{{Code: transport.NameUnknownErrorCode}},
		}
		Expect(isRetryableRegistryError(err)).To(BeFalse())
	})

	It("returns true for net timeout errors", func() {
		err := &net.DNSError{IsTimeout: true}
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})

	It("returns true for exact acme.io no such host error from cluster", func() {
		err := fmt.Errorf("failed to list tags: Get %q: dial tcp: lookup acme.io on 10.96.0.10:53: no such host",
			"https://acme.io/v2/")
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})

	It("returns false for invalid repository name errors", func() {
		err := fmt.Errorf("failed to create repo: invalid reference")
		Expect(isRetryableRegistryError(err)).To(BeFalse())
	})

	It("returns true for no such host wrapped in failed to list tags", func() {
		err := fmt.Errorf("failed to list tags: %w",
			fmt.Errorf(`Get "https://acme.io/v2/": dial tcp: lookup acme.io: no such host`))
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})

	It("returns true for generic failed to list tags network error", func() {
		err := fmt.Errorf("failed to list tags: %w", errors.New("unexpected EOF"))
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})

	It("returns true for DNS server misbehaving", func() {
		err := fmt.Errorf("failed to list tags: %w",
			&net.DNSError{Name: "acme.io", Err: "server misbehaving", IsTimeout: false})
		Expect(isRetryableRegistryError(err)).To(BeTrue())
	})
})

var _ = Describe("SetImageSpec retry behavior", func() {
	var (
		ctx  context.Context
		spec *mellanoxv1alpha1.ImageSpec
		p    *provider
	)

	BeforeEach(func() {
		ctx = context.Background()
		spec = &mellanoxv1alpha1.ImageSpec{
			Image:      "mofed",
			Repository: "nvcr.io/mellanox",
			Version:    "23.10-0.5.5.0",
		}
	})

	It("retries fetch when spec is unchanged and last error is retryable", func() {
		callCount := 0
		p = &provider{
			ctx:           ctx,
			docaImageSpec: spec,
			tags:          []string{},
			lastError: fmt.Errorf("failed to list tags: %w",
				fmt.Errorf(`Get "https://registry:443/v2/": dial tcp 10.0.0.1:443: i/o timeout`)),
			tagLister: func(_ name.Repository, _ ...remote.Option) ([]string, error) {
				callCount++
				return []string{"tag1"}, nil
			},
		}

		p.SetImageSpec(spec)

		Expect(callCount).To(Equal(1))
		Expect(p.lastError).To(BeNil())
		Expect(p.tags).To(Equal([]string{"tag1"}))
	})

	It("does not fetch when spec is unchanged and last fetch succeeded", func() {
		callCount := 0
		p = &provider{
			ctx:           ctx,
			docaImageSpec: spec,
			tags:          []string{"existing-tag"},
			lastError:     nil,
			tagLister: func(_ name.Repository, _ ...remote.Option) ([]string, error) {
				callCount++
				return nil, nil
			},
		}

		p.SetImageSpec(spec)

		Expect(callCount).To(Equal(0))
		Expect(p.tags).To(Equal([]string{"existing-tag"}))
	})

	It("does not fetch when spec is unchanged and last error is non-retryable", func() {
		callCount := 0
		p = &provider{
			ctx:           ctx,
			docaImageSpec: spec,
			tags:          []string{},
			lastError: fmt.Errorf("failed to list tags: %w", &transport.Error{
				StatusCode: http.StatusUnauthorized,
				Errors:     []transport.Diagnostic{{Code: transport.UnauthorizedErrorCode}},
			}),
			tagLister: func(_ name.Repository, _ ...remote.Option) ([]string, error) {
				callCount++
				return nil, nil
			},
		}

		p.SetImageSpec(spec)

		Expect(callCount).To(Equal(0))
		Expect(p.lastError).To(HaveOccurred())
	})
})
