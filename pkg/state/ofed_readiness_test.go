/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/render"
	"github.com/Mellanox/network-operator/pkg/testing/mocks"
	"github.com/Mellanox/network-operator/pkg/utils"
)

func renderedOFEDReadiness(nodePolicy bool) []string {
	GinkgoHelper()
	files, err := utils.GetFilesWithSuffix("../../manifests/state-ofed-driver", render.ManifestFileSuffix...)
	Expect(err).NotTo(HaveOccurred())
	state := stateOFED{stateSkel: stateSkel{
		name: stateOFEDName, description: stateOFEDDescription,
		client: &mocks.ControllerRuntimeClient{}, renderer: render.NewRenderer(files),
	}}
	ofed := &v1alpha1.OFEDDriverSpec{ImageSpec: v1alpha1.ImageSpec{
		Image: "doca-driver", Repository: "nvcr.io/nvidia/mellanox", Version: "doca3.5.0-26.07-0.7.7.0-1",
	}}
	var policy v1alpha1.NicPolicyCR = &v1alpha1.NicClusterPolicy{
		Spec: v1alpha1.NicClusterPolicySpec{OFEDDriver: ofed},
	}
	if nodePolicy {
		policy = &v1alpha1.NicNodePolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "readiness-test"},
			Spec:       v1alpha1.NicNodePolicySpec{OFEDDriver: ofed},
		}
	}
	catalog := NewInfoCatalog()
	catalog.Add(InfoTypeClusterType, &dummyProvider{})
	catalog.Add(InfoTypeNodeInfo, nodeinfo.NewProvider([]*corev1.Node{getNode("node1", kernelFull1)}))
	catalog.Add(InfoTypeDocaDriverImage, &dummyOfedImageProvider{tagExists: true})
	objects, err := state.GetManifestObjects(context.Background(), policy, catalog, testLogger)
	Expect(err).NotTo(HaveOccurred())
	for _, object := range objects {
		if object.GetKind() != "DaemonSet" {
			continue
		}
		ds := appsv1.DaemonSet{}
		Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &ds)).To(Succeed())
		for i := range ds.Spec.Template.Spec.Containers {
			container := &ds.Spec.Template.Spec.Containers[i]
			if container.Name == "mofed-container" {
				Expect(container.ReadinessProbe).NotTo(BeNil())
				return container.ReadinessProbe.Exec.Command
			}
		}
	}
	Fail("rendered OFED DaemonSet has no driver readiness probe")
	return nil
}

var _ = Describe("OFED module readiness probe", func() {
	var root, config string
	var command []string
	modules := []string{"mlx5_core", "mlx5_ib", "ib_core", "ib_uverbs", "ib_umad", "ib_ipoib", "rdma_cm", "rdma_ucm"}

	writeConfig := func(contents string) {
		GinkgoHelper()
		Expect(os.WriteFile(config, []byte(contents), 0o600)).To(Succeed())
	}
	writeModinfo := func(contents string) {
		GinkgoHelper()
		Expect(os.WriteFile(filepath.Join(root, "modinfo"), []byte("#!/bin/bash\n"+contents), 0o700)).To(Succeed())
	}
	runProbe := func(extraEnv ...string) (string, error) {
		GinkgoHelper()
		// Redirect only sysfs reads; execute the rendered script unchanged otherwise.
		// No test-only path override is exposed in the production probe.
		script := strings.ReplaceAll(command[2], "/sys/module/", root+"/modules/")
		cmd := exec.Command(command[0], command[1], script)
		cmd.Env = append([]string{"PATH=" + root + ":/usr/bin:/bin:/usr/sbin:/sbin", "OPENIBD_CONFIG=" + config}, extraEnv...)
		output, err := cmd.CombinedOutput()
		testLogger.V(2).Info("command output", "output", string(output))
		return string(output), err
	}

	BeforeEach(func() {
		root = GinkgoT().TempDir()
		config = filepath.Join(root, "openib.conf")
		command = renderedOFEDReadiness(false)
		Expect(command).To(HaveLen(3))
		Expect(command[:2]).To(Equal([]string{"bash", "-c"}))
		writeConfig("")
		writeModinfo("echo mlx_compat\n")
		for _, module := range modules {
			path := filepath.Join(root, "modules", module)
			Expect(os.MkdirAll(path, 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(path, "initstate"), []byte("live\n"), 0o600)).To(Succeed())
		}
	})

	It("uses the same readiness command for node and cluster policies", func() {
		Expect(renderedOFEDReadiness(true)).To(Equal(command))
	})

	It("accepts the healthy default RDMA stack", func() {
		output, err := runProbe()
		Expect(err).NotTo(HaveOccurred(), output)
	})

	for _, module := range modules {
		It("detects loss and recovery of "+module, func() {
			path := filepath.Join(root, "modules", module)
			Expect(os.Rename(path, path+"-unloaded")).To(Succeed())
			output, err := runProbe()
			Expect(err).To(HaveOccurred())
			Expect(output).To(ContainSubstring("required module " + module + " is absent"))
			Expect(os.Rename(path+"-unloaded", path)).To(Succeed())
			output, err = runProbe()
			Expect(err).NotTo(HaveOccurred(), output)
		})
	}

	DescribeTable("rejects modules that are not live", func(state string) {
		Expect(os.WriteFile(filepath.Join(root, "modules", "mlx5_ib", "initstate"), []byte(state), 0o600)).To(Succeed())
		output, err := runProbe()
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("required module mlx5_ib is not live"))
	}, Entry("loading", "coming\n"), Entry("unloading", "going\n"))

	It("accepts a present module when initstate is not exposed", func() {
		Expect(os.Remove(filepath.Join(root, "modules", "ib_core", "initstate"))).To(Succeed())
		output, err := runProbe()
		Expect(err).NotTo(HaveOccurred(), output)
	})

	DescribeTable("honors explicitly disabled optional modules", func(module, flag string) {
		writeConfig("export " + flag + "='no'\n")
		Expect(os.RemoveAll(filepath.Join(root, "modules", module))).To(Succeed())
		output, err := runProbe()
		Expect(err).NotTo(HaveOccurred(), output)
	},
		Entry("verbs", "ib_uverbs", "UVERBS_LOAD"),
		Entry("MAD", "ib_umad", "UMAD_LOAD"),
		Entry("IPoIB", "ib_ipoib", "IPOIB_LOAD"),
		Entry("CM", "rdma_cm", "RDMA_CM_LOAD"),
		Entry("userspace CM", "rdma_ucm", "RDMA_UCM_LOAD"),
	)

	It("requires explicitly enabled modules without consulting modinfo", func() {
		writeConfig("UVERBS_LOAD=yes\nUMAD_LOAD=yes\nIPOIB_LOAD=yes\nRDMA_CM_LOAD=yes\nRDMA_UCM_LOAD=yes\n")
		writeModinfo("exit 1\n")
		output, err := runProbe()
		Expect(err).NotTo(HaveOccurred(), output)
		Expect(os.RemoveAll(filepath.Join(root, "modules", "rdma_ucm"))).To(Succeed())
		output, err = runProbe()
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("required module rdma_ucm is absent"))
	})

	It("inherits load settings from the environment and lets configuration override them", func() {
		Expect(os.RemoveAll(filepath.Join(root, "modules", "rdma_ucm"))).To(Succeed())
		output, err := runProbe("RDMA_UCM_LOAD=no")
		Expect(err).NotTo(HaveOccurred(), output)
		writeConfig("RDMA_UCM_LOAD=yes\n")
		output, err = runProbe("RDMA_UCM_LOAD=no")
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("required module rdma_ucm is absent"))
	})

	It("honors the openibd SDP override enabling IPoIB", func() {
		writeConfig("SDP_LOAD=yes\nIPOIB_LOAD=no\n")
		Expect(os.RemoveAll(filepath.Join(root, "modules", "ib_ipoib"))).To(Succeed())
		output, err := runProbe()
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("required module ib_ipoib is absent"))
	})

	It("does not default to loading optional modules outside MLNX_OFED", func() {
		writeModinfo("echo ib_core\n")
		for _, module := range modules[3:] {
			Expect(os.RemoveAll(filepath.Join(root, "modules", module))).To(Succeed())
		}
		output, err := runProbe()
		Expect(err).NotTo(HaveOccurred(), output)
	})

	It("fails closed when default module configuration cannot be resolved", func() {
		writeModinfo("exit 1\n")
		output, err := runProbe()
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("cannot determine whether module ib_uverbs is enabled"))
	})

	It("rejects a missing configuration", func() {
		Expect(os.Remove(config)).To(Succeed())
		output, err := runProbe()
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("cannot read " + config))
	})

	It("rejects a configuration with invalid shell syntax", func() {
		writeConfig("if then\n")
		output, err := runProbe()
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring("cannot load " + config))
	})
})
