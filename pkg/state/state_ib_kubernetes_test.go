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
		It("Should Render DeploymentNodeAffinity", func() {
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
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			// Add custom deployment node affinity
			cr.Spec.DeploymentNodeAffinity = &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "custom-label",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"value1", "value2"},
								},
							},
						},
					},
				},
			}

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
			affinity := templateSpec["affinity"].(map[string]interface{})
			nodeAffinity := affinity["nodeAffinity"].(map[string]interface{})

			// Check that the default preferredDuringSchedulingIgnoredDuringExecution is present
			preferred := nodeAffinity["preferredDuringSchedulingIgnoredDuringExecution"].([]interface{})
			Expect(len(preferred)).To(Equal(2))

			// Check that the custom requiredDuringSchedulingIgnoredDuringExecution is present
			required := nodeAffinity["requiredDuringSchedulingIgnoredDuringExecution"].(map[string]interface{})
			nodeSelectorTerms := required["nodeSelectorTerms"].([]interface{})
			Expect(len(nodeSelectorTerms)).To(Equal(1))

			term := nodeSelectorTerms[0].(map[string]interface{})
			matchExpressions := term["matchExpressions"].([]interface{})
			Expect(len(matchExpressions)).To(Equal(1))

			expr := matchExpressions[0].(map[string]interface{})
			Expect(expr["key"]).To(Equal("custom-label"))
			Expect(expr["operator"]).To(Equal("In"))
			values := expr["values"].([]interface{})
			Expect(values).To(ContainElements("value1", "value2"))
		})
		It("Should Render DeploymentTolerations", func() {
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
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			// Add custom deployment tolerations
			cr.Spec.DeploymentTolerations = []v1.Toleration{
				{
					Key:      "custom-taint",
					Operator: v1.TolerationOpEqual,
					Value:    "custom-value",
					Effect:   v1.TaintEffectNoSchedule,
				},
				{
					Key:      "another-taint",
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectPreferNoSchedule,
				},
			}

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
			tolerations := templateSpec["tolerations"].([]interface{})

			// Check that default tolerations are present (3 default + 2 custom = 5 total)
			Expect(len(tolerations)).To(Equal(5))

			// Check that custom tolerations are present
			foundCustomTaint := false
			foundAnotherTaint := false

			for _, tol := range tolerations {
				tolMap := tol.(map[string]interface{})
				if key, exists := tolMap["key"]; exists && key == "custom-taint" {
					foundCustomTaint = true
					Expect(tolMap["operator"]).To(Equal("Equal"))
					Expect(tolMap["value"]).To(Equal("custom-value"))
					Expect(tolMap["effect"]).To(Equal("NoSchedule"))
				}
				if key, exists := tolMap["key"]; exists && key == "another-taint" {
					foundAnotherTaint = true
					Expect(tolMap["operator"]).To(Equal("Exists"))
					Expect(tolMap["effect"]).To(Equal("PreferNoSchedule"))
				}
			}

			Expect(foundCustomTaint).To(BeTrue())
			Expect(foundAnotherTaint).To(BeTrue())
		})
		It("Should Render both DeploymentNodeAffinity and DeploymentTolerations together", func() {
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
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			// Add both custom deployment node affinity and tolerations
			cr.Spec.DeploymentNodeAffinity = &v1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
					{
						Weight: 10,
						Preference: v1.NodeSelectorTerm{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "preferred-label",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"preferred-value"},
								},
							},
						},
					},
				},
			}

			cr.Spec.DeploymentTolerations = []v1.Toleration{
				{
					Key:      "combined-taint",
					Operator: v1.TolerationOpEqual,
					Value:    "combined-value",
					Effect:   v1.TaintEffectNoExecute,
				},
			}

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

			// Check affinity
			affinity := templateSpec["affinity"].(map[string]interface{})
			nodeAffinity := affinity["nodeAffinity"].(map[string]interface{})

			// Check that both default and custom preferredDuringSchedulingIgnoredDuringExecution are present
			preferred := nodeAffinity["preferredDuringSchedulingIgnoredDuringExecution"].([]interface{})
			Expect(len(preferred)).To(Equal(3)) // 2 default + 1 custom

			// Check tolerations
			tolerations := templateSpec["tolerations"].([]interface{})
			Expect(len(tolerations)).To(Equal(4)) // 3 default + 1 custom

			// Verify custom toleration is present
			foundCombinedTaint := false
			for _, tol := range tolerations {
				tolMap := tol.(map[string]interface{})
				if key, exists := tolMap["key"]; exists && key == "combined-taint" {
					foundCombinedTaint = true
					Expect(tolMap["operator"]).To(Equal("Equal"))
					Expect(tolMap["value"]).To(Equal("combined-value"))
					Expect(tolMap["effect"]).To(Equal("NoExecute"))
				}
			}
			Expect(foundCombinedTaint).To(BeTrue())
		})
		It("Should Render ImagePullSecrets", func() {
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
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{"secret1", "secret2", "secret3"}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
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

			// Check that imagePullSecrets is present
			imagePullSecrets, exists := templateSpec["imagePullSecrets"]
			Expect(exists).To(BeTrue())

			// Check that it's a slice
			imagePullSecretsSlice := imagePullSecrets.([]interface{})
			Expect(len(imagePullSecretsSlice)).To(Equal(3))

			// Check each secret name
			secretNames := []string{}
			for _, secret := range imagePullSecretsSlice {
				secretMap := secret.(map[string]interface{})
				name, exists := secretMap["name"]
				Expect(exists).To(BeTrue())
				secretNames = append(secretNames, name.(string))
			}

			Expect(secretNames).To(ContainElements("secret1", "secret2", "secret3"))
		})
		It("Should NOT Render ImagePullSecrets when empty", func() {
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
			Expect(ds.GetKind()).To(Equal("Deployment"))
			spec := ds.Object["spec"].(map[string]interface{})
			template := spec["template"].(map[string]interface{})
			templateSpec := template["spec"].(map[string]interface{})

			// Check that imagePullSecrets is NOT present when empty
			_, exists := templateSpec["imagePullSecrets"]
			Expect(exists).To(BeFalse())
		})
		It("Should Render ImagePullSecrets with other configurations", func() {
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
			ibKubernetesSpec.Image = "image"
			ibKubernetesSpec.ImagePullSecrets = []string{"my-secret"}
			ibKubernetesSpec.Version = "version"
			cr := &mellanoxv1alpha1.NicClusterPolicy{}
			cr.Spec.IBKubernetes = ibKubernetesSpec

			// Add custom deployment tolerations to test combination
			cr.Spec.DeploymentTolerations = []v1.Toleration{
				{
					Key:      "test-taint",
					Operator: v1.TolerationOpEqual,
					Value:    "test-value",
					Effect:   v1.TaintEffectNoSchedule,
				},
			}

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

			// Check that imagePullSecrets is present
			imagePullSecrets, exists := templateSpec["imagePullSecrets"]
			Expect(exists).To(BeTrue())

			imagePullSecretsSlice := imagePullSecrets.([]interface{})
			Expect(len(imagePullSecretsSlice)).To(Equal(1))

			secretMap := imagePullSecretsSlice[0].(map[string]interface{})
			name, exists := secretMap["name"]
			Expect(exists).To(BeTrue())
			Expect(name).To(Equal("my-secret"))

			// Check that tolerations are also present (4 total: 3 default + 1 custom)
			tolerations := templateSpec["tolerations"].([]interface{})
			Expect(len(tolerations)).To(Equal(4))

			// Verify custom toleration is present
			foundTestTaint := false
			for _, tol := range tolerations {
				tolMap := tol.(map[string]interface{})
				if key, exists := tolMap["key"]; exists && key == "test-taint" {
					foundTestTaint = true
					Expect(tolMap["operator"]).To(Equal("Equal"))
					Expect(tolMap["value"]).To(Equal("test-value"))
					Expect(tolMap["effect"]).To(Equal("NoSchedule"))
				}
			}
			Expect(foundTestTaint).To(BeTrue())
		})
	})
})
