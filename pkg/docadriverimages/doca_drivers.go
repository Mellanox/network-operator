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
	"fmt"
	"reflect"
	"sync"
	"time"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Provider provides interface to check the DOCA driver images
type Provider interface {
	// TagExists returns true if DOCA driver image with provided tag exists
	TagExists(tag string) bool
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
}

// TagExists returns true if DOCA driver image with provided tag exists
func (p *provider) TagExists(tag string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range p.tags {
		if t == tag {
			return true
		}
	}
	return false
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
	if reflect.DeepEqual(p.docaImageSpec, spec) {
		p.mu.Unlock()
		return
	}
	p.docaImageSpec = spec
	p.mu.Unlock()
	p.retrieveTags()
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
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			logger.Error(err, "failed to get pull secret")
			return
		}
		pullSecrets = append(pullSecrets, *secret)
	}
	auth, err := kauth.NewFromPullSecrets(p.ctx, pullSecrets)
	if err != nil {
		logger.Error(err, "failed to create registry auth from secrets")
		return
	}
	image := fmt.Sprintf("%s/%s", p.docaImageSpec.Repository, p.docaImageSpec.Image)
	repo, err := name.NewRepository(image)
	if err != nil {
		logger.Error(err, "failed to create repo")
		return
	}
	tags, err := remote.List(repo, remote.WithAuthFromKeychain(auth))
	if err != nil {
		logger.Error(err, "failed to list tags")
		return
	}
	p.tags = tags
}
