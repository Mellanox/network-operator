# Nvidia Mellanox Network Operator Helm Chart

Nvidia Mellanox Network Operator Helm Chart provides an easy way to install, configure and manage
the lifecycle of Nvidia Mellanox network operator.

## Nvidia Mellanox Network Operator
Nvidia Mellanox Network Operator leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order to enable Fast networking, 
RDMA and GPUDirect for workloads in a Kubernetes cluster.
Network Operator works in conjunction with [GPU-Operator](https://github.com/NVIDIA/gpu-operator) to enable GPU-Direct RDMA
on compatible systems.


The Goal of Network Operator is to manage _all_ networking related components to enable execution of
RDMA and GPUDirect RDMA workloads in a kubernetes cluster including:
* Mellanox Networking drivers to enable advanced features
* Kubernetes device plugins to provide hardware resources for fast network
* Kubernetes secondary network for Network intensive workloads

## QuickStart

### System Requirements
* RDMA capable hardware: Mellanox ConnectX-4 NIC or newer.
* NVIDIA GPU and driver supporting GPUDirect e.g Quadro RTX 6000/8000 or Tesla T4 or Tesla V100 or Tesla V100.
(GPU-Direct only)
* Operating Systems: Ubuntu 18.04LTS, 20.04LTS.

### Prerequisites

- Kubernetes v1.17+
- Helm v3
- Ubuntu 18.04LTS, 20.04LTS

### Install Helm

Helm provides an install script to copy helm binary to your system:
```
$ curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
$ chmod 500 get_helm.sh
$ ./get_helm.sh
```

For additional information and methods for installing Helm, refer to the official [helm website](https://helm.sh/)

### Deploy Network Operator

```
# Add Repo
$ helm repo add mellanox https://mellanox.github.io/network-operator
$ helm repo update

# Install Operator
$ helm install -n network-operator --create-namespace --wait mellanox/network-operator network-operator

# View deployed resources
$ kubectl -n network-operator get pods
```

#### Deploy Network Operator without Node Feature Discovery

By default the network operator deploys [Node Feature Discovery (NFD)](https://github.com/kubernetes-sigs/node-feature-discovery)
in order to perform node labeling in the cluster to allow proper scheduling of Network Operator resources.
If the nodes where already labeled by other means, it is possible to disable the deployment of NFD by setting
`nfd.enabled=false` chart parameter.

```
$ helm install --set nfd.enabled=false -n network-operator --create-namespace --wait mellanox/network-operator network-operator
```

##### Currently the following NFD labels are used:

| Label | Where |
| ----- | ----- |
| `feature.node.kubernetes.io/pci-15b3.present` | Nodes bearing Nvidia Mellanox Networking hardware |
| `nvidia.com/gpu.present` | Nodes bearing Nvidia GPU hardware |

>__Note:__ The labels which Network Operator depends on may change between releases.

>__Note:__ By default the operator is deployed without an instance of `NicClusterPolicy` and `MacvlanNetwork`
custom resources. The user is required to create it later with configuration matching the cluster or use
chart parameters to deploy it together with the operator.

## Chart parameters

In order to tailor the deployment of the network operator to your cluster needs
We have introduced the following Chart parameters.

### General parameters

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `nfd.enabled` | bool | `True` | deploy Node Feature Discovery |
| `operator.repository` | string | `mellanox` | Network Operator image repository |
| `operator.image` | string | `network-operator` | Network Operator image name |
| `operator.tag` | string | `None` | Network Operator image tag, if `None`, then the Chart's `appVersion` will be used |
| `deployCR` | bool | `false` | Deploy `NicClusterPolicy` custom resource according to provided parameters |

### Proxy parameters
These proxy parameter will translate to HTTP_PROXY, HTTPS_PROXY, NO_PROXY environment variables to be used by the network operator and relevant resources it deploys.
Production cluster environment can deny direct access to the Internet and instead have an HTTP or HTTPS proxy available.

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `proxy.httpProxy` | string | `None` | proxy URL to use for creating HTTP connections outside the cluster. The URL scheme must be http |
| `proxy.httpsProxy` | string | `None` | proxy URL to use for creating HTTPS connections outside the cluster |
| `proxy.noProxy` | string | `None` | A comma-separated list of destination domain names, domains, IP addresses or other network CIDRs to exclude proxying |

### NicClusterPolicy Custom resource parameters

#### Mellanox OFED driver

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `ofedDriver.deploy` | bool | `false` | deploy Mellanox OFED driver container |
| `ofedDriver.repository` | string | `mellanox` | Mellanox OFED driver image repository |
| `ofedDriver.image` | string | `mofed` | Mellanox OFED driver image name  |
| `ofedDriver.version` | string | `5.2-1.0.4.0` | Mellanox OFED driver version  |

#### NVIDIA Peer memory driver

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `nvPeerDriver.deploy` | bool | `false` | deploy NVIDIA Peer memory driver container |
| `nvPeerDriver.repository` | string | `mellanox` | NVIDIA Peer memory driver image repository |
| `nvPeerDriver.image` | string | `nv-peer-mem-driver` | NVIDIA Peer memory driver image name  |
| `nvPeerDriver.version` | string | `1.0-9` | Mellanox OFED driver version  |
| `nvPeerDriver.gpuDriverSourcePath` | string | `/run/nvidia/driver` | GPU driver soruces root filesystem path(usually used in tandem with [gpu-operator](https://github.com/NVIDIA/gpu-operator)) |

#### RDMA Device Plugin

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `devicePlugin.deploy` | bool | `true` | Deploy device plugin  |
| `devicePlugin.repository` | string | `mellanox` | Device plugin image repository |
| `devicePlugin.image` | string | `k8s-rdma-shared-dev-plugin` | Device plugin image name  |
| `devicePlugin.version` | string | `v1.1.0` | Device plugin version  |
| `devicePlugin.resources` | list | See below | Device plugin resources |

##### RDMA Device Plugin Resource configurations

Consists of a list of RDMA resources each with a name and selector of RDMA capable network devices
to be associated with the resource. Refer to [RDMA Shared Device Plugin Selectors](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin#devices-selectors) for supported selectors.

```
resources:
    - name: rdma_shared_device_a
      vendors: [15b3]
      deviceIDs: [1017]
      ifNames: [enp5s0f0]
    - name: rdma_shared_device_b
      vendors: [15b3]
      deviceIDs: [1017]
      ifNames: [ib0, ib1]
``` 

>__Note__: The parameter listed are non-exhaustive, for the full list of chart parameters refer to
the file: `values.yaml`

#### Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `secondaryNetwork.deploy` | bool | `true` | Deploy Secondary Network  |

Specifies components to deploy in order to facilitate a secondary network in Kubernetes. It consists of the following optionally deployed components:
  - [Multus-CNI](https://github.com/intel/multus-cni): Delegate CNI plugin to support secondary networks in Kubernetes
  - CNI plugins: Currently only [containernetworking-plugins](https://github.com/containernetworking/plugins) is supported
  - IPAM CNI: Currently only [Whereabout IPAM CNI](https://github.com/dougbtv/whereabouts) is supported

##### CNI Plugin Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `cniPlugins.deploy` | bool | `true` | Deploy CNI Plugins Secondary Network  |
| `cniPlugins.image` | string | `containernetworking-plugins` | CNI Plugins image name  |
| `cniPlugins.repository` | string | `mellanox` | CNI Plugins image repository  |
| `cniPlugins.version` | string | `v0.8.7` | CNI Plugins image version  |

##### Multus CNI Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `multus.deploy` | bool | `true` | Deploy Multus Secondary Network  |
| `multus.image` | string | `multus` | Multus image name  |
| `multus.repository` | string | `nfvpe` | Multus image repository  |
| `multus.version` | string | `v3.4.1` | Multus image version  |
| `multus.config` | string | `` | Multus CNI config, if empty then config will be automatically generated from the CNI configuration file of the master plugin (the first file in lexicographical order in cni-conf-dir)  |

##### IPAM CNI Plugin Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `ipamPlugin.deploy` | bool | `true` | Deploy IPAM CNI Plugin Secondary Network  |
| `ipamPlugin.image` | string | `whereabouts` | IPAM CNI Plugin image name  |
| `ipamPlugin.repository` | string | `dougbtv` | IPAM CNI Plugin image repository  |
| `ipamPlugin.version` | string | `v0.3` | IPAM CNI Plugin image version  |

## Deployment Examples

As there are several parameters that are required to be provided to create the custom resource during
operator deployment, it is recommended that a configuration file be used. While its possible to provide
override to the parameter via CLI it would simply be cumbersome.

Below are several deployment examples `values.yaml` provided to helm during installation
of the network operator in the following manner:

`$ helm install -f ./values.yaml -n network-operator --create-namespace --wait mellanox/network-operator network-operator`

#### Example 1
Network Operator deployment with a specific version of OFED driver and a single RDMA resource mapped to `enp1`
netdev.

__values.yaml:__
```:yaml
deployCR: true
ofedDriver:
  deploy: true
  version: 5.2-1.0.4.0
devicePlugin:
  deploy: true
  reources:
    - name: rdma_shared_device_a
      ifNames: [enp1]
```

#### Example 2
Network Operator deployment with the default version of OFED and NV Peer mem driver, RDMA device
plugin with two RDMA resources, the first mapped to `enp1` and `enp2`, the second mapped to `ib0`.

__values.yaml:__
```:yaml
deployCR: true
ofedDriver:
  deploy: true
nvPeerDriver:
  deploy: true
devicePlugin:
  deploy: true
  reources:
    - name: rdma_shared_device_a
      ifNames: [enp1, enp2]
    - name: rdma_shared_device_b
      ifNames: [ib0]
```

#### Example 3
Network Operator deployment with:
- RDMA device plugin, single RDMA resource mapped to `ib0`
- Secondary network
    - Mutlus CNI
    - Containernetworking-plugins CNI plugins
    - Whereabouts IPAM CNI Plugin

__values.yaml:__
```:yaml
deployCR: true
devicePlugin:
  deploy: true
  reources:
    - name: rdma_shared_device_a
      ifNames: [ib0]
secondaryNetwork:
  deploy: true
  multus:
    deploy: true
  cniPlugins:
    deploy: true
  ipamPlugin:
    deploy: true
```

#### Example 4
Network Operator deployment with the default version of RDMA device plugin with RDMA resource
mapped to Mellanox ConnectX-5.

__values.yaml:__
```:yaml
deployCR: true
devicePlugin:
  deploy: true
  resources:
    - name: rdma_shared_device_a
      vendors: [15b3]
      deviceIDs: [1017]
```
