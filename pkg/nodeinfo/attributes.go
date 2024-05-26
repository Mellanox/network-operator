/*
Copyright 2020 NVIDIA

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

package nodeinfo

import (
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("nodeinfo")

// Node labels used by nodeinfo package.
const (
	NodeLabelOSName           = "feature.node.kubernetes.io/system-os_release.ID"
	NodeLabelOSVer            = "feature.node.kubernetes.io/system-os_release.VERSION_ID"
	NodeLabelKernelVerFull    = "feature.node.kubernetes.io/kernel-version.full"
	NodeLabelHostname         = "kubernetes.io/hostname"
	NodeLabelCPUArch          = "kubernetes.io/arch"
	NodeLabelMlnxNIC          = "feature.node.kubernetes.io/pci-15b3.present"
	NodeLabelNvGPU            = "nvidia.com/gpu.present"
	NodeLabelWaitOFED         = "network.nvidia.com/operator.mofed.wait"
	NodeLabelCudaVersionMajor = "nvidia.com/cuda.driver.major"
	NodeLabelOSTreeVersion    = "feature.node.kubernetes.io/system-os_release.OSTREE_VERSION"
)

// Constants for Container Runtime
const (
	Docker     string = "docker"
	Containerd string = "containerd"
	CRIO       string = "cri-o"
)
