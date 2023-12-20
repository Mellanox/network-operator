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

// Package clustertype provides information about the cluster type.
package clustertype

import (
	"context"
	"fmt"

	osconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Type is the type of Kubernetes cluster.
type Type string

const (
	// Openshift is the Openshift distribution.
	Openshift Type = "openshift"
	// Kubernetes is the vanilla Kubernetes distribution.
	Kubernetes Type = "kubernetes"
)

// Provider provides interface to safely check the cluster type
type Provider interface {
	// GetClusterType returns cluster type
	GetClusterType() Type
	// IsKubernetes returns true if cluster type is Kubernetes
	IsKubernetes() bool
	// IsOpenshift returns true if cluster type is Openshift
	IsOpenshift() bool
}

// NewProvider creates a provider for cluster type,
// queries the cluster API to detect the type of the cluster
func NewProvider(ctx context.Context, c client.Client) (Provider, error) {
	clusterType := Openshift
	osClusterVersion := &osconfigv1.ClusterVersionList{}
	err := c.List(ctx, osClusterVersion)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("can't detect cluster type: %v", err)
		}
		clusterType = Kubernetes
	}
	return &provider{clusterType: clusterType}, nil
}

// provider is a static implementation of the Provider interface
type provider struct {
	clusterType Type
}

// GetClusterType returns cluster type
func (p *provider) GetClusterType() Type {
	return p.clusterType
}

// IsKubernetes returns true if cluster type is Kubernetes
func (p *provider) IsKubernetes() bool {
	return p.clusterType == Kubernetes
}

// IsOpenshift returns true if cluster type is Openshift
func (p *provider) IsOpenshift() bool {
	return p.clusterType == Openshift
}
