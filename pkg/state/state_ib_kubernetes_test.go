/*
2022 NVIDIA CORPORATION & AFFILIATES

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

package state

import (
	"context"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

var _ = Describe("IB Kubernetes state rendering tests", func() {

	Context("IB Kubernetes state", func() {
		It("Should Render NetworkAttachmentDefinition", func() {
			client := mocks.ControllerRuntimeClient{}
			manifestBaseDir := "../../manifests/state-ib-kubernetes"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			stateName := "state-ib-kubernetes"
			ibKubernetesState := stateIBKubernetes{
				stateSkel: stateSkel{
					name:        stateName,
					description: "ib-kubernetes deployed in the cluster",
					client:      &client,
					renderer:    renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(ibKubernetesState.Name()).To(Equal(stateName))

			envValues := map[string]string{
				"DAEMON_SM_PLUGIN":       "ufm",
				"DAEMON_PERIODIC_UPDATE": "6",
				"GUID_POOL_RANGE_START":  "rangestart",
				"GUID_POOL_RANGE_END":    "rangeend",
			}
			ufmSecret := "secret"
			ibKubernetesSpec := &mellanoxv1alpha1.IBKubernetesSpec{}
			ibKubernetesSpec.PeriodicUpdateSeconds, _ = strconv.Atoi(envValues["DAEMON_PERIODIC_UPDATE"])
			ibKubernetesSpec.PKeyGUIDPoolRangeStart = envValues["GUID_POOL_RANGE_START"]
			ibKubernetesSpec.PKeyGUIDPoolRangeEnd = envValues["GUID_POOL_RANGE_END"]
			ibKubernetesSpec.UfmSecret = ufmSecret
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"

			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Name = "nic-cluster-policy"
			cr.Spec.IBKubernetes = ibKubernetesSpec
			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})
			objs, err := ibKubernetesState.GetManifestObjects(context.TODO(), cr, catalog, testLogger)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			ds := objs[3]
			Expect(ds.GetKind()).To(Equal("Deployment"))
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})
			containers := templateSpec["containers"].([]interface{})
			env := containers[0].(map[string]interface{})["env"].([]interface{})

			for _, envVar := range env {
				if _, ok := envVar.(map[string]interface{})["value"]; ok {
					value := envVar.(map[string]interface{})["value"].(string)
					name := envVar.(map[string]interface{})["name"].(string)
					Expect(value).To(Equal(envValues[name]))
				}
			}
		})
		It("Should Render ContainerResources", func() {
			manifestBaseDir := "../../manifests/state-ib-kubernetes"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ibKubernetesState := stateIBKubernetes{
				stateSkel: stateSkel{
					renderer: renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())

			quantity, _ := resource.ParseQuantity("150Mi")
			ibKubernetesSpec := &mellanoxv1alpha1.IBKubernetesSpec{}
			ibKubernetesSpec.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{{
				Name: "ib-kubernetes",
				Requests: v1.ResourceList{
					"cpu": quantity,
				},
				Limits: v1.ResourceList{
					"cpu": quantity,
				},
			}}
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})

			objs, err := ibKubernetesState.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			ds := objs[3]
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})
			containers := templateSpec["containers"].([]interface{})
			resources := containers[0].(map[string]interface{})["resources"].(map[string]interface{})
			requests := resources["requests"].(map[string]interface{})
			cpuRequest := requests["cpu"].(string)
			Expect(cpuRequest).To(Equal(quantity.String()))
			limits := resources["limits"].(map[string]interface{})
			cpuLimit := limits["cpu"].(string)
			Expect(cpuLimit).To(Equal(quantity.String()))
		})
		It("Should NOT Render ContainerResources with the wrong container name", func() {
			manifestBaseDir := "../../manifests/state-ib-kubernetes"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ibKubernetesState := stateIBKubernetes{
				stateSkel: stateSkel{
					renderer: renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())

			quantity, _ := resource.ParseQuantity("150Mi")
			ibKubernetesSpec := &mellanoxv1alpha1.IBKubernetesSpec{}
			ibKubernetesSpec.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{{
				Name: "ib-kubernetes-wrong",
				Requests: v1.ResourceList{
					"cpu": quantity,
				},
				Limits: v1.ResourceList{
					"cpu": quantity,
				},
			}}
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})

			objs, err := ibKubernetesState.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			ds := objs[3]
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})
			containers := templateSpec["containers"].([]interface{})
			resources := containers[0].(map[string]interface{})["resources"]
			Expect(resources).To(BeNil())
		})
		It("Should not render limits if nil", func() {
			manifestBaseDir := "../../manifests/state-ib-kubernetes"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ibKubernetesState := stateIBKubernetes{
				stateSkel: stateSkel{
					renderer: renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())

			quantity, _ := resource.ParseQuantity("150Mi")
			ibKubernetesSpec := &mellanoxv1alpha1.IBKubernetesSpec{}
			ibKubernetesSpec.ContainerResources = []mellanoxv1alpha1.ResourceRequirements{{
				Name: "ib-kubernetes",
				Requests: v1.ResourceList{
					"cpu": quantity,
				},
			}}
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})

			objs, err := ibKubernetesState.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			ds := objs[3]
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})
			containers := templateSpec["containers"].([]interface{})
			resources := containers[0].(map[string]interface{})["resources"].(map[string]interface{})
			requests := resources["requests"].(map[string]interface{})
			cpuRequest := requests["cpu"].(string)
			Expect(cpuRequest).To(Equal(quantity.String()))
			limits := resources["limits"]
			Expect(limits).To(BeNil())
		})
		It("Should have image in SHA256 format", func() {
			manifestBaseDir := "../../manifests/state-ib-kubernetes"

			files, err := utils.GetFilesWithSuffix(manifestBaseDir, render.ManifestFileSuffix...)
			Expect(err).NotTo(HaveOccurred())
			renderer := render.NewRenderer(files)

			ibKubernetesState := stateIBKubernetes{
				stateSkel: stateSkel{
					renderer: renderer,
				},
			}

			Expect(err).NotTo(HaveOccurred())

			ibKubernetesSpec := &mellanoxv1alpha1.IBKubernetesSpec{}
			ibKubernetesSpec.Image = "myImage"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "sha256:1699d23027ea30c9fa"
			ibKubernetesSpec.Repository = "myRepo"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			catalog := NewInfoCatalog()
			catalog.Add(InfoTypeClusterType, &dummyProvider{})

			objs, err := ibKubernetesState.GetManifestObjects(context.TODO(), cr, catalog, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs)).To(Equal(4))
			ds := objs[3]
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})
			containers := templateSpec["containers"].([]interface{})
			image := containers[0].(map[string]interface{})["image"]
			Expect(image).To(Equal("myRepo/myImage@sha256:1699d23027ea30c9fa"))
		})
	})
})
