# [WIP] Nvidia Mellanox NIC Operator

Nvidia Mellanox NIC Operator leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order to enable Fast networking, 
RDMA and GPUDirect for workloads in a Kubernetes cluster.

## Prerequisites

### Kubernetes Node Feature Discovery (NFD)
Mellanox NIC operator relies on Node labeling to get the cluster to the desired state.
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
      - "device"
```

>\* Required for GPUDirect driver container deployment 

## Resource Definitions
The Operator Acts on the following CRDs:

### NICClusterPolicy CRD
CRD that defines a Cluster state for Mellanox Network devices.

__The Cluster state includes:__
- OFED driver container to be deployed on Mellanox supporting nodes
- GPUDirect driver container to be deployed on RDMA & GPU supported nodes
- RDMA shared device plugin and related configurations

## System Requirements
As more driver containers are built the operator will be able to support additional platforms.
for Phase 1 Ubuntu20.04 is planned to be supported.

## Deployment Example

TBD

## Driver Containers
Driver containers are essentially containers that have or yield kernel modules compatible
with the underlying kernel.
An initialization script loads the modules when the container is run (in privileged mode)
making them available to the kernel.

While this approach may seem odd. It provides a way to deliver drivers to immutable systems.