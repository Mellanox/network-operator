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

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
)

type StringList []string

func (s *StringList) String() string {
	return strings.Join(*s, ", ")
}

func (s *StringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var (
	crdsDir StringList
)

func initFlags() {
	flag.Var(&crdsDir, "crds-dir", "Path to the directory containing the CRD manifests")
	flag.Parse()

	if len(crdsDir) == 0 {
		log.Fatalf("CRDs directory is required")
	}

	for _, crdDir := range crdsDir {
		if _, err := os.Stat(crdDir); os.IsNotExist(err) {
			log.Fatalf("CRDs directory %s does not exist", crdsDir)
		}
	}
}

func main() {
	ctx := context.Background()

	initFlags()

	config, err := ctrl.GetConfig()
	if err != nil {
		log.Fatalf("Failed to get Kubernetes config: %v", err)
	}

	client, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create API extensions client: %v", err)
	}

	if err := walkCrdsDir(ctx, client); err != nil {
		log.Fatalf("Failed to apply CRDs: %v", err)
	}
}

// walkCrdsDir walks the CRDs directory and applies each YAML file.
func walkCrdsDir(ctx context.Context, client *clientset.Clientset) error {
	for _, crdDir := range crdsDir {
		// Walk the directory recursively and apply each YAML file.
		err := filepath.Walk(crdDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || filepath.Ext(path) != ".yaml" {
				return nil
			}

			log.Printf("Apply CRDs from file: %s", path)
			if err := applyCRDsFromFile(ctx, client, path); err != nil {
				return fmt.Errorf("apply CRD %s: %v", path, err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk the path %s: %v", crdsDir, err)
		}
	}
	return nil
}

// applyCRDsFromFile reads a YAML file, splits it into documents, and applies each CRD to the cluster.
func applyCRDsFromFile(ctx context.Context, client *clientset.Clientset, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file %q: %v", filePath, err)
	}
	defer file.Close()

	// Create a decoder that reads multiple YAML documents.
	decoder := yaml.NewYAMLOrJSONDecoder(file, 4096)
	var crdsToApply []*apiextensionsv1.CustomResourceDefinition
	for {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := decoder.Decode(crd); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode YAML: %v", err)
		}
		if crd.GetObjectKind().GroupVersionKind().Kind != "CustomResourceDefinition" {
			log.Printf("Skipping non-CRD object %s", crd.GetName())
			continue
		}
		crdsToApply = append(crdsToApply, crd)
	}

	// Apply each CRD separately.
	for _, crd := range crdsToApply {
		err := wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func(context.Context) (bool, error) {
			if err := applyCRD(ctx, client, crd); err != nil {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("apply CRD %s: %v", crd.Name, err)
		}
	}
	return nil
}

// applyCRD creates or updates the CRD.
func applyCRD(ctx context.Context, client *clientset.Clientset, crd *apiextensionsv1.CustomResourceDefinition) error {
	crdClient := client.ApiextensionsV1().CustomResourceDefinitions()

	// Check if CRD already exists in cluster and create if not found.
	curCRD, err := crdClient.Get(ctx, crd.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Printf("Create CRD %s", crd.Name)
		_, err = crdClient.Create(ctx, crd, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create CRD %s: %v", crd.Name, err)
		}
	} else {
		log.Printf("Update CRD %s", crd.Name)
		// Set resource version to update an existing CRD.
		crd.SetResourceVersion(curCRD.GetResourceVersion())
		_, err = crdClient.Update(ctx, crd, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update CRD %s: %v", crd.Name, err)
		}
	}
	return nil
}
