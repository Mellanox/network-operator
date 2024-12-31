[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mellanox/network-operator)](https://goreportcard.com/report/github.com/Mellanox/network-operator)
[![Coverage Status](https://coveralls.io/repos/github/Mellanox/network-operator/badge.svg)](https://coveralls.io/github/Mellanox/network-operator)

- [NVIDIA Network Operator](#nvidia-network-operator)
  - [Documentation](#documentation)
  - [Prerequisites](#prerequisites)
    - [Kubernetes Node Feature Discovery (NFD)](#kubernetes-node-feature-discovery-nfd)
  - [Resource Definitions](#resource-definitions)
    - [NICClusterPolicy CRD](#nicclusterpolicy-crd)
      - [NICClusterPolicy spec:](#nicclusterpolicy-spec)
        - [Example for NICClusterPolicy resource:](#example-for-nicclusterpolicy-resource)
      - [NICClusterPolicy status](#nicclusterpolicy-status)
        - [Example Status field of a NICClusterPolicy instance](#example-status-field-of-a-nicclusterpolicy-instance)
    - [MacvlanNetwork CRD](#macvlannetwork-crd)
      - [MacvlanNetwork spec:](#macvlannetwork-spec)
        - [Example for MacvlanNetwork resource:](#example-for-macvlannetwork-resource)
    - [HostDeviceNetwork CRD](#hostdevicenetwork-crd)
      - [HostDeviceNetwork spec:](#hostdevicenetwork-spec)
        - [Example for HostDeviceNetwork resource:](#example-for-hostdevicenetwork-resource)
    - [IPoIBNetwork CRD](#ipoibnetwork-crd)
      - [IPoIBNetwork spec:](#ipoibnetwork-spec)
        - [Example for IPoIBNetwork resource:](#example-for-ipoibnetwork-resource)
  - [System Requirements](#system-requirements)
  - [Tested Network Adapters](#tested-network-adapters)
  - [Compatibility Notes](#compatibility-notes)
  - [Deployment Example](#deployment-example)
  - [Docker image](#docker-image)
  - [Driver Containers](#driver-containers)
  - [Upgrade](#upgrade)
  - [Externally Provided Configurations For Network Operator Sub-Components](#externally-provided-configurations-for-network-operator-sub-components)

<small><i><a href='http://ecotrust-canada.github.io/markdown-toc/'>Table of contents generated with markdown-toc</a></i></small>

# NVIDIA Network Operator
NVIDIA Network Operator leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order to enable Fast networking,
RDMA and GPUDirect for workloads in a Kubernetes cluster.

The Goal of Network Operator is to manage _all_ networking related components to enable execution of
RDMA and GPUDirect RDMA workloads in a kubernetes cluster including:
* Mellanox Networking drivers to enable advanced features
* Kubernetes device plugins to provide hardware resources for fast network
* Kubernetes secondary network for Network intensive workloads

## Documentation
For more information please visit the official [documentation](https://docs.nvidia.com/networking/software/cloud-orchestration/index.html).


## Prerequisites
### Kubernetes Node Feature Discovery (NFD)
NVIDIA Network operator relies on Node labeling to get the cluster to the desired state.
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) `v0.13.2` or newer is deployed by default via HELM chart installation.
NFD is used to label nodes with the following labels:

- PCI vendor and device information
- RDMA capability
- GPU features*

>__NOTE__: We use [nodeFeatureRules](https://kubernetes-sigs.github.io/node-feature-discovery/v0.13/usage/custom-resources.html#nodefeaturerule) to label PCI vendor and device.This is enabled via `nfd.deployNodeFeatureRules` chart parameter.

__Example NFD worker configurations:__

```yaml
    config:
      sources:
        pci:
          deviceClassWhitelist:
          - "0300"
          - "0302"
          deviceLabelFields:
          - vendor
```

>\* Required for GPUDirect driver container deployment

>__NOTE__: If NFD is already deployed in the cluster, make sure to pass `--set nfd.enabled=false` to the helm install command to avoid conflicts,
and if NFD is deployed from this repo the `enableNodeFeatureApi` flag is enabled by default to have the ability to create NodeFeatureRules.

## Resource Definitions
The Operator Acts on the following CRDs:

### NICClusterPolicy CRD
CRD that defines a Cluster state for Mellanox Network devices.

>__NOTE__: The operator will act on a NicClusterPolicy instance with a predefined name "nic-cluster-policy", instances with different names will be ignored.

#### NICClusterPolicy spec:
NICClusterPolicy CRD Spec includes the following sub-states:
- `ofedDriver`: [OFED driver container](https://github.com/Mellanox/ofed-docker) to be deployed on Mellanox supporting nodes.
- `rdmaSharedDevicePlugin`: [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)
and related configurations.
- `sriovDevicePlugin`: [SR-IOV Network Device Plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin)
    and related configurations.
- `ibKubernetes`: [InfiniBand Kubernetes](https://github.com/Mellanox/ib-kubernetes/) and related configurations. 
- `secondaryNetwork`: Specifies components to deploy in order to facilitate a secondary network in Kubernetes. It consists of the following optionally deployed components:
    - [Multus-CNI](https://github.com/intel/multus-cni): Delegate CNI plugin to support secondary networks in Kubernetes
    - CNI plugins: Currently only [containernetworking-plugins](https://github.com/containernetworking/plugins) is supported
    - [IP Over Infiniband (IPoIB) CNI Plugin](https://github.com/Mellanox/ipoib-cni): Allow users to create an IPoIB child link and move it to the pod.
    - IPAM CNI: [Whereabouts IPAM CNI](https://github.com/k8snetworkplumbingwg/whereabouts) and related configurations
- `nvIpam`: [NVIDIA Kubernetes IPAM](https://github.com/Mellanox/nvidia-k8s-ipam) and related configurations.

>__NOTE__: Any sub-state may be omitted if it is not required for the cluster.

>__NOTE__: NVIDIA IPAM and Whereabouts IPAM plugin can be deployed simultaneously in the same cluster


##### Example for NICClusterPolicy resource:
In the example below we request OFED driver to be deployed together with RDMA shared device plugin.

```
apiVersion: mellanox.com/v1alpha1
kind: NicClusterPolicy
metadata:
  name: nic-cluster-policy
spec:
  ofedDriver:
    image: mofed
    repository: nvcr.io/nvidia/mellanox
    version: 23.04-0.5.3.3.1
    startupProbe:
      initialDelaySeconds: 10
      periodSeconds: 10
    livenessProbe:
      initialDelaySeconds: 30
      periodSeconds: 30
    readinessProbe:
      initialDelaySeconds: 10
      periodSeconds: 30
  rdmaSharedDevicePlugin:
    image: k8s-rdma-shared-dev-plugin
    repository: ghcr.io/mellanox
    version: sha-fe7f371c7e1b8315bf900f71cd25cfc1251dc775
    # The config below directly propagates to k8s-rdma-shared-device-plugin configuration.
    # Replace 'devices' with your (RDMA capable) netdevice name.
    config: |
      {
        "configList": [
          {
            "resourceName": "rdma_shared_device_a",
            "rdmaHcaMax": 63,
            "selectors": {
              "vendors": ["15b3"],
              "deviceIDs": ["1017"],
              "ifNames": ["ens2f0"]
            }
          }
        ]
      }
  secondaryNetwork:
    cniPlugins:
      image: plugins
      repository: ghcr.io/k8snetworkplumbingwg
      version: v1.2.0-amd64
    multus:
      image: multus-cni
      repository: ghcr.io/k8snetworkplumbingwg
      version: v3.9.3
      # if config is missing or empty then multus config will be automatically generated from the CNI configuration file of the master plugin (the first file in lexicographical order in cni-conf-dir)
      # config: ''
    ipamPlugin:
      image: whereabouts
      repository: ghcr.io/k8snetworkplumbingwg
      version: v0.6.1-amd64
```

Can be found at: `example/crs/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`

NicClusterPolicy with [NVIDIA Kubernetes IPAM](https://github.com/Mellanox/nvidia-k8s-ipam) configuration

```
apiVersion: mellanox.com/v1alpha1
kind: NicClusterPolicy
metadata:
  name: nic-cluster-policy
spec:
  ofedDriver:
    image: mofed
    repository: nvcr.io/nvidia/mellanox
    version: 23.04-0.5.3.3.1
    startupProbe:
      initialDelaySeconds: 10
      periodSeconds: 10
    livenessProbe:
      initialDelaySeconds: 30
      periodSeconds: 30
    readinessProbe:
      initialDelaySeconds: 10
      periodSeconds: 30
  rdmaSharedDevicePlugin:
    image: k8s-rdma-shared-dev-plugin
    repository: ghcr.io/mellanox
    version: sha-fe7f371c7e1b8315bf900f71cd25cfc1251dc775
    # The config below directly propagates to k8s-rdma-shared-device-plugin configuration.
    # Replace 'devices' with your (RDMA capable) netdevice name.
    config: |
      {
        "configList": [
          {
            "resourceName": "rdma_shared_device_a",
            "rdmaHcaMax": 63,
            "selectors": {
              "vendors": ["15b3"],
              "deviceIDs": ["101b"]
            }
          }
        ]
      }
  secondaryNetwork:
    cniPlugins:
      image: plugins
      repository: ghcr.io/k8snetworkplumbingwg
      version: v1.2.0-amd64
    multus:
      image: multus-cni
      repository: ghcr.io/k8snetworkplumbingwg
      version: v3.9.3
  nvIpam:
    image: nvidia-k8s-ipam
    repository: ghcr.io/mellanox
    version: v0.1.2
    enableWebhook: false
```

Can be found at: `example/crs/mellanox.com_v1alpha1_nicclusterpolicy_cr-nvidia-ipam.yaml`

#### NICClusterPolicy status
NICClusterPolicy `status` field reflects the current state of the system.
It contains a per sub-state and a global state `status`.

The sub-state `status` indicates if the cluster has transitioned to the desired
state for that sub-state, e.g OFED driver container deployed and loaded on relevant nodes,
RDMA device plugin deployed and running on relevant nodes.

The global state reflects the logical _AND_ of each individual sub-state.

##### Example Status field of a NICClusterPolicy instance
```
status:
  appliedStates:
  - name: state-pod-security-policy
    state: ignore
  - name: state-multus-cni
    state: ready
  - name: state-container-networking-plugins
    state: ignore
  - name: state-ipoib-cni
    state: ignore
  - name: state-whereabouts-cni
    state: ready
  - name: state-OFED
    state: ready
  - name: state-SRIOV-device-plugin
    state: ignore
  - name: state-RDMA-device-plugin
    state: ready
  - name: state-ib-kubernetes
    state: ignore
  - name: state-nv-ipam-cni
    state: ready
  state: ready
```

>__NOTE__: An `ignore` State indicates that the sub-state was not defined in the custom resource
> thus it is ignored.

### MacvlanNetwork CRD
This CRD defines a MacVlan secondary network. It is translated by the Operator to a `NetworkAttachmentDefinition` instance as defined in [k8snetworkplumbingwg/multi-net-spec](https://github.com/k8snetworkplumbingwg/multi-net-spec).

#### MacvlanNetwork spec:
MacvlanNetwork CRD Spec includes the following fields:
- `networkNamespace`: Namespace for NetworkAttachmentDefinition related to this MacvlanNetwork CRD.
- `master`: Name of the host interface to enslave. Defaults to default route interface.
- `mode`: Mode of interface one of "bridge", "private", "vepa", "passthru", default "bridge".
- `mtu`: MTU of interface to the specified value. 0 for master's MTU.
- `ipam`: IPAM configuration to be used for this network.

##### Example for MacvlanNetwork resource:
In the example below we deploy MacvlanNetwork CRD instance with mode as bridge, MTU 1500, default route interface as master,
with resource "rdma/rdma_shared_device_a", that will be used to deploy NetworkAttachmentDefinition for macvlan to default namespace.


With [Whereabouts IPAM CNI](https://github.com/k8snetworkplumbingwg/whereabouts)

```
apiVersion: mellanox.com/v1alpha1
kind: MacvlanNetwork
metadata:
  name: example-macvlannetwork
spec:
  networkNamespace: "default"
  master: "ens2f0"
  mode: "bridge"
  mtu: 1500
  ipam: |
    {
      "type": "whereabouts",
      "datastore": "kubernetes",
      "kubernetes": {
        "kubeconfig": "/etc/cni/net.d/whereabouts.d/whereabouts.kubeconfig"
      },
      "range": "192.168.2.225/28",
      "exclude": [
       "192.168.2.229/30",
       "192.168.2.236/32"
      ],
      "log_file" : "/var/log/whereabouts.log",
      "log_level" : "info",
      "gateway": "192.168.2.1"
    }
```

Can be found at: `example/crs/mellanox.com_v1alpha1_macvlannetwork_cr.yaml`

With [NVIDIA Kubernetes IPAM](https://github.com/Mellanox/nvidia-k8s-ipam)

```
apiVersion: mellanox.com/v1alpha1
kind: MacvlanNetwork
metadata:
  name: example-macvlannetwork
spec:
  networkNamespace: "default"
  master: "ens2f0"
  mode: "bridge"
  mtu: 1500
  ipam: |
    {
      "type": "nv-ipam",
      "poolName": "my-pool"
    }
```

Can be found at: `example/crs/mellanox.com_v1alpha1_macvlannetwork_cr-nvidia-ipam.yaml`

### HostDeviceNetwork CRD
This CRD defines a HostDevice secondary network. It is translated by the Operator to a `NetworkAttachmentDefinition` instance as defined in [k8snetworkplumbingwg/multi-net-spec](https://github.com/k8snetworkplumbingwg/multi-net-spec).

#### HostDeviceNetwork spec:
HostDeviceNetwork CRD Spec includes the following fields:
- `networkNamespace`: Namespace for NetworkAttachmentDefinition related to this HostDeviceNetwork CRD.
- `resourceName`: Host device resource pool.
- `ipam`: IPAM configuration to be used for this network.

##### Example for HostDeviceNetwork resource:
In the example below we deploy HostDeviceNetwork CRD instance with "hostdev" resource pool, that will be used to deploy NetworkAttachmentDefinition for HostDevice network to default namespace.

```
apiVersion: mellanox.com/v1alpha1
kind: HostDeviceNetwork
metadata:
  name: example-hostdevice-network
spec:
  networkNamespace: "default"
  resourceName: "hostdev"
  ipam: |
    {
      "type": "whereabouts",
      "datastore": "kubernetes",
      "kubernetes": {
        "kubeconfig": "/etc/cni/net.d/whereabouts.d/whereabouts.kubeconfig"
      },
      "range": "192.168.3.225/28",
      "exclude": [
       "192.168.3.229/30",
       "192.168.3.236/32"
      ],
      "log_file" : "/var/log/whereabouts.log",
      "log_level" : "info"
    }
```

Can be found at: `example/crs/mellanox.com_v1alpha1_hostdevicenetwork_cr.yaml`

### IPoIBNetwork CRD
This CRD defines an IPoIBNetwork secondary network. It is translated by the Operator to a `NetworkAttachmentDefinition` instance as defined in [k8snetworkplumbingwg/multi-net-spec](https://github.com/k8snetworkplumbingwg/multi-net-spec).

#### IPoIBNetwork spec:
HostDeviceNetwork CRD Spec includes the following fields:
- `networkNamespace`: Namespace for NetworkAttachmentDefinition related to this HostDeviceNetwork CRD.
- `master`: Name of the host interface to enslave.
- `ipam`: IPAM configuration to be used for this network.

##### Example for IPoIBNetwork resource:
In the example below we deploy IPoIBNetwork CRD instance with "ibs3f1" host interface, that will be used to deploy NetworkAttachmentDefinition for IPoIBNetwork network to default namespace.

```
apiVersion: mellanox.com/v1alpha1
kind: IPoIBNetwork
metadata:
  name: example-ipoibnetwork
spec:
  networkNamespace: "default"
  master: "ibs3f1"
  ipam: |
    {
      "type": "whereabouts",
      "datastore": "kubernetes",
      "kubernetes": {
        "kubeconfig": "/etc/cni/net.d/whereabouts.d/whereabouts.kubeconfig"
      },
      "range": "192.168.5.225/28",
      "exclude": [
       "192.168.6.229/30",
       "192.168.6.236/32"
      ],
      "log_file" : "/var/log/whereabouts.log",
      "log_level" : "info",
      "gateway": "192.168.6.1"
    }

```

Can be found at: `example/crs/mellanox.com_v1alpha1_ipoibnetwork_cr.yaml`

## System Requirements
* RDMA capable hardware: Mellanox ConnectX-5 NIC or newer.
* NVIDIA GPU and driver supporting GPUDirect e.g Quadro RTX 6000/8000 or Tesla T4 or Tesla V100 or Tesla V100.
(GPU-Direct only)
* Operating Systems: Ubuntu 20.04 LTS

>__NOTE__: As more driver containers are built the operator will be able to support additional platforms.
>__NOTE__: ConnectX-6 Lx is not supported.

## Tested Network Adapters
The following Network Adapters have been tested with NVIDIA Network Operator:
* ConnectX-5
* ConnectX-6 Dx

## Compatibility Notes
* NVIDIA Network Operator is compatible with NVIDIA GPU Operator v1.5.2 and above
* Starting from v465 NVIDIA GPU driver includes a built-in nvidia_peermem module
  which is a replacement for nv_peer_mem module. NVIDIA GPU operator manages nvidia_peermem module loading.

## Deployment Example
Deployment of NVIDIA Network Operator consists of:
* Deploying NVIDIA Network Operator CRDs found under `./config/crd/bases`:
    * mellanox.com_nicclusterpolicies_crd.yaml
    * mellanox.com_macvlan_crds.yaml
    * k8s.cni.cncf.io-networkattachmentdefinitions-crd.yaml
* Deploying network operator resources by running `make deploy`
* Defining and deploying a NICClusterPolicy custom resource.
Example can be found under `./example/crs/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`
* Defining and deploying a MacvlanNetwork custom resource.
Example can be found under `./example/crs/mellanox.com_v1alpha1_macvlannetwork_cr.yaml`

A deployment example can be found under `example` folder [here](https://github.com/Mellanox/network-operator/blob/master/example/README.md).

## Docker image
To build a container image for Network Operator use:
```bash
make image
```

To build a multi-arch image and publish to a registry use:
```bash
export REGISTRY=example.com/registry 
export IMAGE_NAME=network-operator 
export VERSION=v1.1.1 
make image-build-multiarch image-push-multiarch
```

## Driver Containers
Driver containers are essentially containers that have or yield kernel modules compatible
with the underlying kernel.
An initialization script loads the modules when the container is run (in privileged mode)
making them available to the kernel.

While this approach may seem odd. It provides a way to deliver drivers to immutable systems.

[Mellanox OFED container](https://github.com/Mellanox/ofed-docker)

Mellanox OFED driver container supports customization of its behaviour via environment variables.
This is regarded as advanced functionallity and generally should not be needed.

check [MOFED Driver Container Environment Variables](docs/mofed-container-env-vars.md)

## Upgrade
Check [Upgrade section in Helm Chart documentation](deployment/network-operator/README.md#upgrade) for details.

## Externally Provided Configurations For Network Operator Sub-Components

In most cases, Network Operator will be deployed together with the related configurations
for the various sub-components it deploys e.g. Nvidia k8s IPAM plugin, RDMA shared device plugin
or SR-IOV Network device plugin.

Specifying configuration either via Helm values when installing NVIDIA
network operator, or by specifying them when directly creating NicClusterPolicy CR.
These configurations eventually trigger the creation of a ConfigMap object in K8s.

> __Note__: It is the responsibility of the user to delete any existing configurations (ConfigMaps) if
> they were already created by the Network Operator as well as deleting his own configuration when they
> are no longer required.
