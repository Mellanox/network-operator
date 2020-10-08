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
    version: latest
    # The config below directly propagates to k8s-rdma-shared-device-plugin configuration.
    # Replace 'devices' with your (RDMA capable) netdevice name.
    config: |
      {
        "configList": [
          {
            "resourceName": "hca_shared_devices_a",
            "rdmaHcaMax": 1000,
            "devices": ["ens2f0"]
          }
        ]
      }
```

Can be found at: `example/deploy/crds/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`

#### NICClusterPolicy status
NICClusterPolicy `status` field reflects the current state of the system.
It contains a per sub-state and a global state `status`.

The sub-state `status` indicates if the cluster has transitioned to the desired
state for that sub-state, e.g OFED driver container deployed and loaded on relevant nodes,
RDMA device plugin deployed and running on relevant nodes.

The global state reflects the logical _AND_ of each inidvidual sub-state.

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
  State:    ready
```

>__NOTE__: An `ignore` State indicates that the sub-state was not defined in the custom resource
> thus it is ignored.

## System Requirements
* RDMA capable hardware: Mellanox ConnectX-4 NIC or newer.
* NVIDIA GPU and driver supporting GPUDirect e.g Quadro RTX 6000/8000 or Tesla T4 or Tesla V100 or Tesla V100.
(GPU-Direct only)
* Operating System: Ubuntu18.04 with kernel `4.15.0-109-generic`.

>__NOTE__: As more driver containers are built the operator will be able to support additional platforms.

## Deployment Example
Deployment of network-operator consists of:
* Deploying netowrk-operator CRD found under `./deploy/crds/mellanox.com_nicclusterpolicies_crd.yaml`
* Deploying network operator resources found under `./deploy/` e.g operator namespace, 
role, role binding, service account and the network-operator daemonset
* Defining and deploying a NICClusterPolicy custom resource.
Template can be found under `./deploy/crds/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml`

A deployment example can be found under `example` folder [here](https://github.com/Mellanox/network-operator/example/README.md).

## Driver Containers
Driver containers are essentially containers that have or yield kernel modules compatible
with the underlying kernel.
An initialization script loads the modules when the container is run (in privileged mode)
making them available to the kernel.

While this approach may seem odd. It provides a way to deliver drivers to immutable systems.

[Mellanox OFED and NV Peer Memory driver container](https://github.com/Mellanox/ofed-docker)

