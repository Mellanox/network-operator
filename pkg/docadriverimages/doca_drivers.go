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

// Package docadriverimages package provides information about DOCA driver images
package docadriverimages

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Provider provides interface to check the DOCA driver images
type Provider interface {
	// TagExists returns true if DOCA driver image with provided tag exists
	// Returns an error if there was a problem listing tags from the registry
	TagExists(tag string) (bool, error)
	// SetImageSpec sets the Container registry details
	SetImageSpec(*mellanoxv1alpha1.ImageSpec)
}

// NewProvider creates a provider for  DOCA driver images,
// queries the container image registry to get the exiting tags
func NewProvider(ctx context.Context, c client.Client) Provider {
	p := &provider{c: c, docaImageSpec: nil, tags: make([]string, 0), ctx: ctx}

	ticker := time.NewTicker(time.Duration(config.FromEnv().State.DocaDriverImagePollTimeMinutes) * time.Minute)

	go func() {
		for ; ; <-ticker.C {
			p.retrieveTags()
		}
	}()
	return p
}

// provider is a static implementation of the Provider interface
type provider struct {
	c             client.Client
	tags          []string
	docaImageSpec *mellanoxv1alpha1.ImageSpec
	ctx           context.Context
	mu            sync.Mutex
	lastError     error // tracks the last error from tag retrieval
	// tagLister overrides remote.List when set (used in unit tests).
	tagLister func(name.Repository, ...remote.Option) ([]string, error)
}

// TagExists returns true if DOCA driver image with provided tag exists
func (p *provider) TagExists(tag string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range p.tags {
		if t == tag {
			return true, nil
		}
	}
	return false, p.lastError
}

// SetImageSpec sets the Container registry details
func (p *provider) SetImageSpec(spec *mellanoxv1alpha1.ImageSpec) {
	p.mu.Lock()
	if spec == nil {
		p.docaImageSpec = nil
		if len(p.tags) > 0 {
			p.tags = make([]string, 0)
		}
		p.mu.Unlock()
		return
	}
	specChanged := p.docaImageSpec == nil || !reflect.DeepEqual(p.docaImageSpec, spec)
	if specChanged {
		p.docaImageSpec = spec
	}
	lastErr := p.lastError
	shouldRefresh := specChanged || (lastErr != nil && isRetryableRegistryError(lastErr))
	retrying := !specChanged && shouldRefresh
	p.mu.Unlock()
	if shouldRefresh {
		if retrying {
			log.FromContext(p.ctx).Info("retrying DOCA driver image tag fetch after transient error",
				"repo", spec.Repository, "image", spec.Image, "error", lastErr)
		}
		p.retrieveTags()
	} else if !specChanged {
		log.FromContext(p.ctx).Info("DOCA tag refresh skipped on reconcile",
			"repo", spec.Repository, "image", spec.Image,
			"hasLastError", lastErr != nil, "retryable", isRetryableRegistryError(lastErr), "lastError", lastErr)
	}
}

func (p *provider) retrieveTags() {
	if p.docaImageSpec == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	logger := log.FromContext(p.ctx)
	logger.Info("fetching DOCA driver image tags", "repo", p.docaImageSpec.Repository, "image", p.docaImageSpec.Image)
	pullSecrets := make([]corev1.Secret, 0)
	for _, name := range p.docaImageSpec.ImagePullSecrets {
		secret := &corev1.Secret{}
		err := p.c.Get(p.ctx, types.NamespacedName{
			Name:      name,
			Namespace: config.FromEnv().State.NetworkOperatorResourceNamespace,
		}, secret)
		if apiErrors.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get pull secret")
			p.lastError = fmt.Errorf("failed to get pull secret: %w", err)
			return
		}
		pullSecrets = append(pullSecrets, *secret)
	}
	auth, err := kauth.NewFromPullSecrets(p.ctx, pullSecrets)
	if err != nil {
		logger.Error(err, "failed to create registry auth from secrets")
		p.lastError = fmt.Errorf("failed to create registry auth from secrets: %w", err)
		return
	}
	image := fmt.Sprintf("%s/%s", p.docaImageSpec.Repository, p.docaImageSpec.Image)
	repo, err := name.NewRepository(image)
	if err != nil {
		logger.Error(err, "failed to create repo")
		p.lastError = fmt.Errorf("failed to create repo: %w", err)
		return
	}
	var tags []string
	listOpts := []remote.Option{remote.WithAuthFromKeychain(auth)}
	if p.tagLister != nil {
		tags, err = p.tagLister(repo, listOpts...)
	} else {
		tags, err = remote.List(repo, listOpts...)
	}
	if err != nil {
		retryable := isRetryableRegistryError(fmt.Errorf("failed to list tags: %w", err))
		logger.Info("failed to list DOCA driver image tags", "repo", p.docaImageSpec.Repository,
			"image", p.docaImageSpec.Image, "retryable", retryable, "error", err)
		p.lastError = fmt.Errorf("failed to list tags: %w", err)
		return
	}
	p.tags = tags
	p.lastError = nil // clear error on successful tag retrieval
}

func isRetryableRegistryError(err error) bool {
	if err == nil {
		return false
	}

	var transportErr *transport.Error
	if errors.As(err, &transportErr) {
		if isNonRetryableTransportError(transportErr) {
			return false
		}
		if transportErr.Temporary() {
			return true
		}
	}

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "failed to create repo") ||
		strings.Contains(errMsg, "failed to create registry auth from secrets") {
		return false
	}

	// Treat tag-list failures as retryable unless classified non-retryable above
	// (covers timeouts, DNS, TLS, and other network errors with varying wrappers).
	if strings.Contains(errMsg, "failed to list tags") {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if apiErrors.IsTimeout(err) || apiErrors.IsServerTimeout(err) ||
		apiErrors.IsServiceUnavailable(err) || apiErrors.IsInternalError(err) {
		return true
	}

	if strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "server misbehaving") ||
		strings.Contains(errMsg, "temporary failure in name resolution") ||
		strings.Contains(errMsg, "tls:") {
		return true
	}

	return false
}

func isNonRetryableTransportError(err *transport.Error) bool {
	switch err.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	}
	for _, d := range err.Errors {
		switch d.Code {
		case transport.UnauthorizedErrorCode, transport.DeniedErrorCode, transport.NameUnknownErrorCode:
			return true
		}
	}
	return false
}
