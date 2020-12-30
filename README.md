[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mellanox/network-operator)](https://goreportcard.com/report/github.com/Mellanox/network-operator)
[![Build Status](https://travis-ci.com/Mellanox/network-operator.svg?branch=master)](https://travis-ci.com/Mellanox/network-operator)

- [Nvidia Mellanox Network Operator](#nvidia-mellanox-network-operator)
  * [Prerequisites](#prerequisites)
    + [Kubernetes Node Feature Discovery (NFD)](#kubernetes-node-feature-discovery--nfd-)
  * [Resource Definitions](#resource-definitions)
    + [NICClusterPolicy CRD](#nicclusterpolicy-crd)
      - [NICClusterPolicy spec](#nicclusterpolicy-spec)
        * [Example for NICClusterPolicy resource:](#example-for-nicclusterpolicy-resource)
      - [NICClusterPolicy status](#nicclusterpolicy-status)
        * [Example Status field of a NICClusterPolicy instance:](#example-status-field-of-a-nicclusterpolicy-instance)
    + [MacvlanNetwork CRD](#macvlannetwork-crd)
      - [MacvlanNetwork spec](#macvlannetwork-spec)
        * [Example for MacvlanNetwork resource:](#example-for-macvlannetwork-resource)
  * [System Requirements](#system-requirements)
  * [Deployment Example](#deployment-example)
  * [Driver Containers](#driver-containers)

<small><i><a href='http://ecotrust-canada.github.io/markdown-toc/'>Table of contents generated with markdown-toc</a></i></small>

# Nvidia Mellanox Network Operator
Nvidia Mellanox Network Operator leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order to enable Fast networking,
RDMA and GPUDirect for workloads in a Kubernetes cluster.

The Goal of Network Operator is to manage _all_ networking related components to enable execution of
RDMA and GPUDirect RDMA workloads in a kubernetes cluster including:
* Mellanox Networking drivers to enable advanced features
* Kubernetes device plugins to provide hardware resources for fast network
* Kubernetes secondary network for Network intensive workloads

## Prerequisites
### Kubernetes Node Feature Discovery (NFD)
Mellanox Network operator relies on Node labeling to get the cluster to the desired state.
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) `v0.6.0` or newer is expected to be deployed to provide the appropriate labeling:

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
- `devicePlugin`: [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)
and related configurations.
- `nvPeerDriver`: [Nvidia Peer Memory client driver container](https://github.com/Mellanox/ofed-docker)
to be deployed on RDMA & GPU supporting nodes (required for GPUDirect workloads).
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
  namespace: mlnx-network-operator
spec:
  ofedDriver:
    image: ofed-driver
    repository: mellanox
    version: 5.0-2.1.8.0
  devicePlugin:
    image: k8s-rdma-shared-dev-plugin
    repository: mellanox
    version: v1.1.0
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

Can be found at: `example/deploy/crds/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`

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

Can be found at: `example/deploy/crds/mellanox.com_v1alpha1_macvlannetwork_cr.yaml`

## System Requirements
* RDMA capable hardware: Mellanox ConnectX-4 NIC or newer.
* NVIDIA GPU and driver supporting GPUDirect e.g Quadro RTX 6000/8000 or Tesla T4 or Tesla V100 or Tesla V100.
(GPU-Direct only)
* Operating Systems: Ubuntu 18.04LTS, 20.04.LTS

>__NOTE__: As more driver containers are built the operator will be able to support additional platforms.

## Deployment Example
Deployment of network-operator consists of:
* Deploying network-operator CRDs found under `./deploy/crds/`:
    * mellanox.com_nicclusterpolicies_crd.yaml
    * mellanox.com_macvlan_crds.yaml
    * k8s.cni.cncf.io-networkattachmentdefinitions-crd.yaml
* Deploying network operator resources found under `./deploy/` e.g operator namespace,
role, role binding, service account and the network-operator daemonset
* Defining and deploying a NICClusterPolicy custom resource.
Template can be found under `./deploy/crds/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`
* Defining and deploying a MacvlanNetwork custom resource.
Template can be found under `./deploy/crds/mellanox.com_v1alpha1_macvlannetwork_cr.yaml`

A deployment example can be found under `example` folder [here](https://github.com/Mellanox/network-operator/example/README.md).

## Driver Containers
Driver containers are essentially containers that have or yield kernel modules compatible
with the underlying kernel.
An initialization script loads the modules when the container is run (in privileged mode)
making them available to the kernel.

While this approach may seem odd. It provides a way to deliver drivers to immutable systems.

[Mellanox OFED and NV Peer Memory driver container](https://github.com/Mellanox/ofed-docker)

