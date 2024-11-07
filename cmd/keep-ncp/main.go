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

// Package main adds Helm 'keep' annotation on NCP if needed
package main

import (
	"context"
	"log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/consts"
)

const (
	helmKeepValue         = "keep"
	helmResourcePolicyKey = "helm.sh/resource-policy"
)

func main() {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	utilruntime.Must(mellanoxv1alpha1.AddToScheme(scheme))

	config, err := ctrl.GetConfig()
	if err != nil {
		log.Fatalf("Failed to get Kubernetes config: %v", err)
	}

	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("Error creating controller-runtime client: %v", err)
	}

	if err := annotateNCP(ctx, c); err != nil {
		log.Fatalf("Failed to annotate NicClusterPolicy: %v", err)
	}
}

// annotateNCP annotate NicClusterPolicy with "helm.sh/resource-policy=keep" if needed
func annotateNCP(ctx context.Context, c client.Client) error {
	ncp := &mellanoxv1alpha1.NicClusterPolicy{}
	key := client.ObjectKey{
		Name: consts.NicClusterPolicyResourceName,
	}
	err := c.Get(ctx, key, ncp)
	if apierrors.IsNotFound(err) {
		log.Println("NicClusterPolicy does not exists. No annotation needed")
		return nil
	}
	if err != nil {
		log.Println("Failed to get NicClusterPolicy")
		return err
	}
	labels := ncp.GetLabels()
	if labels["app.kubernetes.io/managed-by"] != "Helm" {
		log.Println("NicClusterPolicy is not managed by Helm. No annotation needed")
		return nil
	}
	if ncp.Annotations != nil {
		val, ok := ncp.Annotations[helmResourcePolicyKey]
		if ok && val == helmKeepValue {
			log.Println("NicClusterPolicy already have keep annotation")
			return nil
		}
	}
	if ncp.Annotations == nil {
		ncp.Annotations = make(map[string]string)
	}
	ncp.Annotations[helmResourcePolicyKey] = helmKeepValue

	err = c.Update(ctx, ncp)
	if err != nil {
		log.Println("Error updating NicClusterPolicy with 'keep' annotation")
		return err
	}
	return nil
}
