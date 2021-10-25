[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mellanox/network-operator)](https://goreportcard.com/report/github.com/Mellanox/network-operator)
[![Build Status](https://travis-ci.com/Mellanox/network-operator.svg?branch=master)](https://travis-ci.com/Mellanox/network-operator)

- [Nvidia Network Operator](#nvidia-network-operator)
  * [Prerequisites](#prerequisites)
    + [Kubernetes Node Feature Discovery (NFD)](#kubernetes-node-feature-discovery-nfd)
  * [Resource Definitions](#resource-definitions)
    + [NICClusterPolicy CRD](#nicclusterpolicy-crd)
      - [NICClusterPolicy spec](#nicclusterpolicy-spec)
        * [Example for NICClusterPolicy resource:](#example-for-nicclusterpolicy-resource)
      - [NICClusterPolicy status](#nicclusterpolicy-status)
        * [Example Status field of a NICClusterPolicy instance:](#example-status-field-of-a-nicclusterpolicy-instance)
    + [MacvlanNetwork CRD](#macvlannetwork-crd)
      - [MacvlanNetwork spec](#macvlannetwork-spec)
        * [Example for MacvlanNetwork resource:](#example-for-macvlannetwork-resource)
    + [HostDeviceNetwork CRD](#hostdevicenetwork-crd)
      - [HostDeviceNetwork spec](#hostdevicenetwork-spec)
        * [Example for HostDeviceNetwork resource:](#example-for-hostdevicenetwork-resource)
  * [Pod Security Policy](#pod-security-policy)
  * [System Requirements](#system-requirements)
  * [Compatibility Notes](#compatibility-notes)
  * [Deployment Example](#deployment-example)
  * [Docker image](#docker-image)
  * [Driver Containers](#driver-containers)
  * [Upgrade](#upgrade)

<small><i><a href='http://ecotrust-canada.github.io/markdown-toc/'>Table of contents generated with markdown-toc</a></i></small>

# Nvidia Network Operator
Nvidia Network Operator leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order to enable Fast networking,
RDMA and GPUDirect for workloads in a Kubernetes cluster.

The Goal of Network Operator is to manage _all_ networking related components to enable execution of
RDMA and GPUDirect RDMA workloads in a kubernetes cluster including:
* Mellanox Networking drivers to enable advanced features
* Kubernetes device plugins to provide hardware resources for fast network
* Kubernetes secondary network for Network intensive workloads

## Documentation
For more information please visit the official [documentation](https://docs.mellanox.com/display/COKAN10).


## Prerequisites
### Kubernetes Node Feature Discovery (NFD)
Nvidia Network operator relies on Node labeling to get the cluster to the desired state.
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) `v0.6.0-233-g3e00bfb` or newer is expected to be deployed to provide the appropriate labeling:

- PCI vendor and device information
- RDMA capability
- GPU features*

__Example NFD worker configurations:__

```yaml
sources:
  custom:
  pci:
    deviceClassWhitelist:
      - "02"
      - "0200"
      - "0207"
      - "03"
      - "12"
    deviceLabelFields:
      - "vendor"
```

>\* Required for GPUDirect driver container deployment

## Resource Definitions
The Operator Acts on the following CRDs:

### NICClusterPolicy CRD
CRD that defines a Cluster state for Mellanox Network devices.

>__NOTE__: The operator will act on a NicClusterPolicy instance with a predefined name "nic-cluster-policy", instances with different names will be ignored.

#### NICClusterPolicy spec:
NICClusterPolicy CRD Spec includes the following sub-states/stages:
- `ofedDriver`: [OFED driver container](https://github.com/Mellanox/ofed-docker) to be deployed on Mellanox supporting nodes.
- `rdmaSharedDevicePlugin`: [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)
and related configurations.
- `nvPeerDriver`: [Nvidia Peer Memory client driver container](https://github.com/Mellanox/ofed-docker)
to be deployed on RDMA & GPU supporting nodes (required for GPUDirect workloads).
  For NVIDIA GPU driver version < 465. Check [compatibility notes](#compatibility-notes) for details
- `SecondaryNetwork`: Specifies components to deploy in order to facilitate a secondary network in Kubernetes. It consists of the folowing optionally deployed components:
    - [Multus-CNI](https://github.com/intel/multus-cni): Delegate CNI plugin to support secondary networks in Kubernetes
    - CNI plugins: Currently only [containernetworking-plugins](https://github.com/containernetworking/plugins) is supported
    - IPAM CNI: Currently only [Whereabout IPAM CNI](https://github.com/dougbtv/whereabouts-cni) is supported

>__NOTE__: Any sub-state may be omitted if it is not required for the cluster.

##### Example for NICClusterPolicy resource:
In the example below we request OFED driver to be deployed together with RDMA shared device plugin
but without NV Peer Memory driver.

```
apiVersion: mellanox.com/v1alpha1
kind: NicClusterPolicy
metadata:
  name: nic-cluster-policy
  namespace: nvidia-network-operator
spec:
  ofedDriver:
    image: mofed
    repository: mellanox
    version: 5.3-1.0.0.1
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
    repository: mellanox
    version: v1.2.1
    # The config below directly propagates to k8s-rdma-shared-device-plugin configuration.
    # Replace 'devices' with your (RDMA capable) netdevice name.
    config: |
      {
        "configList": [
          {
            "resourceName": "hca_shared_devices_a",
            "rdmaHcaMax": 1000,
            "selectors": {
              "vendors": ["15b3"],
              "deviceIDs": ["1017"],
              "ifNames": ["ens2f0"]
            }
          }
        ]
      }
  sriovDevicePlugin:
    image: sriov-device-plugin
    repository: docker.io/nfvpe
    version: v3.3
    config: |
      {
        "resourceList": [
            {
                "resourcePrefix": "nvidia.com",
                "resourceName": "hostdev",
                "selectors": {
                    "vendors": ["15b3"],
                    "isRdma": true
                }
            }
        ]
      }
  secondaryNetwork:
    cniPlugins:
      image: containernetworking-plugins
      repository: mellanox
      version: v0.8.7
    multus:
      image: multus
      repository: nfvpe
      version: v3.4.1
      # if config is missing or empty then multus config will be automatically generated from the CNI configuration file of the master plugin (the first file in lexicographical order in cni-conf-dir)
      config: ''
    ipamPlugin:
      image: whereabouts
      repository: dougbtv
      version: v0.3.0
```

Can be found at: `example/crs/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`

#### NICClusterPolicy status
NICClusterPolicy `status` field reflects the current state of the system.
It contains a per sub-state and a global state `status`.

The sub-state `status` indicates if the cluster has transitioned to the desired
state for that sub-state, e.g OFED driver container deployed and loaded on relevant nodes,
RDMA device plugin deployed and running on relevant nodes.

The global state reflects the logical _AND_ of each individual sub-state.

##### Example Status field of a NICClusterPolicy instance
```
Status:
  Applied States:
    Name:   state-OFED
    State:  ready
    Name:   state-RDMA-device-plugin
    State:  ready
    Name:   state-NV-Peer
    State:  ignore
    Name:   state-cni-plugins
    State:  ignore
    Name:   state-Multus
    State:  ready
    Name:   state-whereabouts
    State:  ready
  State:    ready
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
In the example below we deploy MacvlanNetwork CRD instance with mode as bridge, mtu 1500, default route interface as master,
with resouce "rdma/hca_shared_devices_a", that will be used to deploy NetworkAttachmentDefinition for macvlan to default namespace.

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

### HostDeviceNetwork CRD
This CRD defines a HostDevice secondary network. It is translated by the Operator to a `NetworkAttachmentDefinition` instance as defined in [k8snetworkplumbingwg/multi-net-spec](https://github.com/k8snetworkplumbingwg/multi-net-spec).

#### HostDeviceNetwork spec:
HostDeviceNetwork CRD Spec includes the following fields:
- `networkNamespace`: Namespace for NetworkAttachmentDefinition related to this HostDeviceNetwork CRD.
- `ResourceName`: Host device resource pool.
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
  ResourceName: "hostdev"
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

Can be found at: `mellanox.com_v1alpha1_hostdevicenetwork_cr.yaml`

## Pod Security Policy
Network-operator supports [Pod Security Policies](https://kubernetes.io/docs/concepts/policy/pod-security-policy/). When NicClusterPolicy is created with `psp.enabled=True`, privileged PSP is created and applied to all network-operator's pods. Requires [admission controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#how-do-i-turn-on-an-admission-control-plug-in) to be enabled.

## System Requirements
* RDMA capable hardware: Mellanox ConnectX-5 NIC or newer.
* NVIDIA GPU and driver supporting GPUDirect e.g Quadro RTX 6000/8000 or Tesla T4 or Tesla V100 or Tesla V100.
(GPU-Direct only)
* Operating Systems: Ubuntu 20.04 LTS

>__NOTE__: As more driver containers are built the operator will be able to support additional platforms.

## Compatibility Notes
* network-operator is compatible with NVIDIA GPU Operator v1.5.2 and above
* network-operator will deploy nvPeerDriver POD on a node only if NVIDIA GPU driver version < 465.
  Starting from v465 NVIDIA GPU driver includes a built-in nvidia_peermem module
  which is a replacement for nv_peer_mem module. NVIDIA GPU operator manages nvidia_peermem module loading.

## Deployment Example
Deployment of network-operator consists of:
* Deploying network-operator CRDs found under `./config/crd/bases`:
    * mellanox.com_nicclusterpolicies_crd.yaml
    * mellanox.com_macvlan_crds.yaml
    * k8s.cni.cncf.io-networkattachmentdefinitions-crd.yaml
* Deploying network operator resources found under `./deploy/` e.g operator namespace,
role, role binding, service account and the network-operator daemonset
* Defining and deploying a NICClusterPolicy custom resource.
Example can be found under `./example/crs/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`
* Defining and deploying a MacvlanNetwork custom resource.
Example can be found under `./example/crs/mellanox.com_v1alpha1_macvlannetwork_cr.yaml`

A deployment example can be found under `example` folder [here](https://github.com/Mellanox/network-operator/blob/master/example/README.md).

## Docker image
Network operator uses `alpine` base image by default. To build Network operator with
another base image you need to pass `BASE_IMAGE` argument:

```
docker build -t network-operator \
--build-arg BASE_IMAGE=registry.access.redhat.com/ubi8/ubi-minimal:latest \
.
```

## Driver Containers
Driver containers are essentially containers that have or yield kernel modules compatible
with the underlying kernel.
An initialization script loads the modules when the container is run (in privileged mode)
making them available to the kernel.

While this approach may seem odd. It provides a way to deliver drivers to immutable systems.

[Mellanox OFED and NV Peer Memory driver container](https://github.com/Mellanox/ofed-docker)

## Upgrade
Network operator provides limited upgrade capabilities which require additional
manual actions if containerized OFED driver is used.
Future releases of the network operator will provide automatic upgrade flow for the containerized driver.

Check [Upgrade section in Helm Chart documentation](deployment/network-operator/README.md#upgrade) for details.
