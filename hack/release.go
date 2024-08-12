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

// Package main creates release templates.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"

	"github.com/google/go-containerregistry/pkg/authn"
	containerregistryname "github.com/google/go-containerregistry/pkg/name"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ReleaseImageSpec contains ImageSpec in addition with Image SHA256.
type ReleaseImageSpec struct {
	// Shas is a list of SHA2256. A list is needed for DOCA drivers that have multiple images.
	Shas []SHA256ImageRef
	mellanoxv1alpha1.ImageSpec
}

// SHA256ImageRef contains container image in sha256 format and a description.
type SHA256ImageRef struct {
	// ImageRef is the image reference in "sha format" e.g repo/project/image-repo@sha256:abcdef
	ImageRef string
	// Name is a description of the image reference
	Name string
}

// Release contains versions for operator release templates.
type Release struct {
	NetworkOperator              *ReleaseImageSpec
	NetworkOperatorInitContainer *ReleaseImageSpec
	SriovNetworkOperator         *ReleaseImageSpec
	SriovNetworkOperatorWebhook  *ReleaseImageSpec
	SriovConfigDaemon            *ReleaseImageSpec
	SriovCni                     *ReleaseImageSpec
	SriovIbCni                   *ReleaseImageSpec
	Mofed                        *ReleaseImageSpec
	RdmaSharedDevicePlugin       *ReleaseImageSpec
	SriovDevicePlugin            *ReleaseImageSpec
	IbKubernetes                 *ReleaseImageSpec
	CniPlugins                   *ReleaseImageSpec
	Multus                       *ReleaseImageSpec
	Ipoib                        *ReleaseImageSpec
	IpamPlugin                   *ReleaseImageSpec
	NvIPAM                       *ReleaseImageSpec
	NicFeatureDiscovery          *ReleaseImageSpec
	DOCATelemetryService         *ReleaseImageSpec
	OVSCni                       *ReleaseImageSpec
	RDMACni                      *ReleaseImageSpec
}

func readDefaults(releaseDefaults string) Release {
	f, err := os.ReadFile(filepath.Clean(releaseDefaults))
	if err != nil {
		log.Fatal(err)
	}
	var release Release
	if err := yaml.Unmarshal(f, &release); err != nil {
		log.Fatal(err)
	}

	return release
}

func getEnviromnentVariableOrDefault(defaultValue, varName string) string {
	val := os.Getenv(varName)
	if val != "" {
		return val
	}
	return defaultValue
}

func initWithEnvVariale(name string, image *ReleaseImageSpec) {
	envName := name + "_IMAGE"
	image.Image = getEnviromnentVariableOrDefault(image.Image, envName)
	envName = name + "_REPO"
	image.Repository = getEnviromnentVariableOrDefault(image.Repository, envName)
	envName = name + "_VERSION"
	image.Version = getEnviromnentVariableOrDefault(image.Version, envName)
}

func readEnvironmentVariables(release *Release) {
	initWithEnvVariale("NETWORK_OPERATOR", release.NetworkOperator)
	initWithEnvVariale("NETWORK_OPERATOR_INIT_CONTAINER", release.NetworkOperatorInitContainer)
	initWithEnvVariale("MOFED", release.Mofed)
	initWithEnvVariale("RDMA_SHARED_DEVICE_PLUGIN", release.RdmaSharedDevicePlugin)
	initWithEnvVariale("SRIOV_DEVICE_PLUGIN", release.SriovDevicePlugin)
	initWithEnvVariale("IB_KUBERNEES", release.IbKubernetes)
	initWithEnvVariale("CNI_PLUGINS", release.CniPlugins)
	initWithEnvVariale("MULTUS", release.Multus)
	initWithEnvVariale("IPOIB", release.Ipoib)
	initWithEnvVariale("IPAM_PLUGIN", release.Ipoib)
	initWithEnvVariale("NV_IPAM", release.NvIPAM)
	initWithEnvVariale("NIC_FEATURE_DISCOVERY", release.NicFeatureDiscovery)
	initWithEnvVariale("DOCA_TELEMETRY_SERVICE", release.DOCATelemetryService)
	initWithEnvVariale("OVS_CNI", release.OVSCni)
	initWithEnvVariale("RDMA_CNI", release.RDMACni)
}

func main() {
	templateDir := flag.String("templateDir", ".", "Directory with templates to render")
	outputDir := flag.String("outputDir", ".", "Destination directory to render templates to")
	releaseDefaults := flag.String("releaseDefaults", "release.yaml", "Destination of the release defaults definition")
	retrieveSha := flag.Bool("with-sha256", false, "retrieve SHA256 for container images references")
	flag.Parse()
	release := readDefaults(*releaseDefaults)
	readEnvironmentVariables(&release)
	if *retrieveSha {
		err := resolveImagesSha(&release)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	var files []string
	err := filepath.Walk(*templateDir, func(path string, info os.FileInfo, err error) error {
		// Error during traversal
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip non suffix files
		base := info.Name()
		if strings.HasSuffix(base, ".template") {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	for _, file := range files {
		tmpl, err := template.New(filepath.Base(file)).Funcs(template.FuncMap{
			"imageAsSha": func(obj interface{}) string {
				imageSpec := obj.(*ReleaseImageSpec)
				return imageSpec.Shas[0].ImageRef
			},
		}).ParseFiles(file)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Generate new file full path
		outputFile := filepath.Join(*outputDir, strings.Replace(filepath.Base(file), ".template", ".yaml", 1))
		f, err := os.Create(filepath.Clean(outputFile))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		err = tmpl.Execute(f, release)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func resolveImagesSha(release *Release) error {
	nvcrToken := os.Getenv("NGC_CLI_API_KEY")
	if nvcrToken == "" {
		return fmt.Errorf("NGC_CLI_API_KEY is unset")
	}
	auth := &authn.Basic{
		Username: "$oauthtoken",
		Password: nvcrToken,
	}
	v := reflect.ValueOf(*release)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.IsNil() {
			releaseImageSpec := field.Interface().(*ReleaseImageSpec)
			if strings.Contains(releaseImageSpec.Image, "doca-driver") {
				digests, err := resolveDocaDriversShas(releaseImageSpec.Repository, releaseImageSpec.Image,
					releaseImageSpec.Version, auth)
				if err != nil {
					return err
				}
				releaseImageSpec.Shas = make([]SHA256ImageRef, len(digests))
				for i, digest := range digests {
					sha := fmt.Sprintf("%s/%s@%s", releaseImageSpec.Repository, releaseImageSpec.Image, digest)
					releaseImageSpec.Shas[i] = SHA256ImageRef{ImageRef: sha, Name: fmt.Sprintf("doca-driver-%d", i)}
				}
			} else {
				digest, err := resolveImageSha(releaseImageSpec.Repository, releaseImageSpec.Image,
					releaseImageSpec.Version, auth)
				if err != nil {
					return err
				}
				releaseImageSpec.Shas = make([]SHA256ImageRef, 1)
				sha := fmt.Sprintf("%s/%s@%s", releaseImageSpec.Repository, releaseImageSpec.Image, digest)
				releaseImageSpec.Shas[0] = SHA256ImageRef{ImageRef: sha}
			}
		}
	}
	return nil
}

func resolveImageSha(repo, image, tag string, auth *authn.Basic) (string, error) {
	ref, err := containerregistryname.ParseReference(fmt.Sprintf("%s/%s:%s", repo, image, tag))
	if err != nil {
		return "", err
	}
	var desc *remote.Descriptor
	if strings.Contains(repo, "nvstaging") {
		desc, err = remote.Get(ref, remote.WithAuth(auth))
		if err != nil {
			return "", err
		}
	} else {
		// Container registry might fail if providing unneeded auth
		desc, err = remote.Get(ref)
		if err != nil {
			return "", err
		}
	}
	digest, err := containerregistryv1.NewHash(desc.Descriptor.Digest.String())
	if err != nil {
		return "", err
	}
	return digest.String(), nil
}

func resolveDocaDriversShas(repoName, imageName, ver string, auth *authn.Basic) ([]string, error) {
	shaArray := make([]string, 0)
	image := fmt.Sprintf("%s/%s", repoName, imageName)
	repo, err := containerregistryname.NewRepository(image)
	if err != nil {
		return shaArray, err
	}
	var tags []string
	if strings.Contains(repoName, "nvstaging") {
		tags, err = remote.List(repo, remote.WithAuth(auth))
		if err != nil {
			return shaArray, err
		}
	} else {
		// Container registry might fail if providing unneeded auth
		tags, err = remote.List(repo)
		if err != nil {
			return shaArray, err
		}
	}
	sort.Strings(tags)
	shaSet := make(map[string]interface{})
	for _, tag := range tags {
		if strings.Contains(tag, ver) && (strings.Contains(tag, "rhcos") || strings.Contains(tag, "rhel")) {
			digest, err := resolveImageSha(repoName, imageName, tag, auth)
			if err != nil {
				return shaArray, err
			}
			if _, ok := shaSet[digest]; !ok {
				shaArray = append(shaArray, digest)
				shaSet[digest] = struct{}{}
			}
		}
	}
	return shaArray, nil
}
