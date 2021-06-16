# Nvidia Network Operator Helm Chart

Nvidia Network Operator Helm Chart provides an easy way to install, configure and manage
the lifecycle of Nvidia Mellanox network operator.

## Nvidia Network Operator
Nvidia Network Operator leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order to enable Fast networking, 
RDMA and GPUDirect for workloads in a Kubernetes cluster.
Network Operator works in conjunction with [GPU-Operator](https://github.com/NVIDIA/gpu-operator) to enable GPU-Direct RDMA
on compatible systems.


The Goal of Network Operator is to manage _all_ networking related components to enable execution of
RDMA and GPUDirect RDMA workloads in a kubernetes cluster including:
* Mellanox Networking drivers to enable advanced features
* Kubernetes device plugins to provide hardware resources for fast network
* Kubernetes secondary network for Network intensive workloads

## Additional components

### Node Feature Discovery
Nvidia Network Operator relies on the existance of specific node labels to operate properly.
e.g label a node as having Nvidia networking hardware available.
This can be achieved by either manually labeling Kubernetes nodes or using
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) to perform the labeling.

To allow zero touch deployment of the Operator we provide a helm chart to be used to
optionally deploy Node Feature Discovery in the cluster. This is enabled via `nfd.enabled` chart parameter.

### SR-IOV Network Operator
Nvidia Network Operator can operate in unison with SR-IOV Network Operator
to enable SR-IOV workloads in a Kubernetes cluster. We provide a helm chart to be used to optionally
deploy [SR-IOV Network Operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator) in the cluster.
This is enabled via `sriovNetworkOperator.enabled` chart parameter.

For more information on how to configure SR-IOV in your Kubernetes cluster using SR-IOV Network Operator
refer to the project's github.

## QuickStart

### System Requirements
* RDMA capable hardware: Mellanox ConnectX-5 NIC or newer.
* NVIDIA GPU and driver supporting GPUDirect e.g Quadro RTX 6000/8000 or Tesla T4 or Tesla V100 or Tesla V100.
(GPU-Direct only)
* Operating Systems: Ubuntu 20.04 LTS.

### Prerequisites

- Kubernetes v1.17+
- Helm v3.5.3+
- Ubuntu 20.04 LTS

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
$ helm install --devel -n network-operator --create-namespace --wait network-operator mellanox/network-operator

# View deployed resources
$ kubectl -n network-operator get pods
```

#### Deploy Network Operator without Node Feature Discovery

By default the network operator deploys [Node Feature Discovery (NFD)](https://github.com/kubernetes-sigs/node-feature-discovery)
in order to perform node labeling in the cluster to allow proper scheduling of Network Operator resources.
If the nodes where already labeled by other means, it is possible to disable the deployment of NFD by setting
`nfd.enabled=false` chart parameter.

```
$ helm install --devel --set nfd.enabled=false -n network-operator --create-namespace --wait network-operator mellanox/network-operator
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

## Helm Tests

Network Operator has Helm tests to verify deployment. To run tests it is required to set the following chart parameters on helm install/upgrade: `deployCR`, `rdmaSharedDevicePlugin`, `secondaryNetwork` as the test depends on `NicClusterPolicy` instance being deployed by Helm.
Supported Tests:
- Device Plugin Resource: This test creates a pod that requests the first resource in `rdmaSharedDevicePlugin.resources`
- RDMA Traffic: This test creates a pod that test loopback RDMA traffic with `rping`

Run the helm test with following command after deploying network operator with helm
```
$ helm test -n network-operator network-operator --timeout=5m
```
Notes:
- Test will keeping running endlessly if pod creating failed so it is recommended to use `--timeout` which fails test after exceeding given timeout
- Default PF to run test is `ens2f0` to override it add `--set test.pf=<pf_name>` to `helm install/upgrade`
- Tests should be executed after `NicClusterPolicy` custom resource state is `Ready`
- In case of a test failed it is possible to collect the logs with `kubectl logs -n <namespace> <test-pod-name>`

## Chart parameters

In order to tailor the deployment of the network operator to your cluster needs
We have introduced the following Chart parameters.

### General parameters

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `nfd.enabled` | bool | `True` | deploy Node Feature Discovery |
| `sriovNetworkOperator.enabled` | bool | `False` | deploy SR-IOV Network Operator |
| `operator.repository` | string | `mellanox` | Network Operator image repository |
| `operator.image` | string | `network-operator` | Network Operator image name |
| `operator.tag` | string | `None` | Network Operator image tag, if `None`, then the Chart's `appVersion` will be used |
| `operator.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the Network Operator image |
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
| `ofedDriver.image` | string | `mofed` | Mellanox OFED driver image name |
| `ofedDriver.version` | string | `5.3-1.0.0.1` | Mellanox OFED driver version |
| `ofedDriver.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the Mellanox OFED driver image |
| `ofedDriver.startupProbe.initialDelaySeconds` | int | 10 | Mellanox OFED startup probe initial delay |
| `ofedDriver.startupProbe.periodSeconds` | int | 10 | Mellanox OFED startup probe interval |
| `ofedDriver.livenessProbe.initialDelaySeconds` | int | 30 | Mellanox OFED liveness probe initial delay |
| `ofedDriver.livenessProbe.periodSeconds` | int | 30 | Mellanox OFED liveness probe interval|
| `ofedDriver.readinessProbe.initialDelaySeconds` | int | 10 | Mellanox OFED readiness probe initial delay |
| `ofedDriver.readinessProbe.periodSeconds` | int | 30 | Mellanox OFED readiness probe interval |

#### NVIDIA Peer memory driver

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `nvPeerDriver.deploy` | bool | `false` | deploy NVIDIA Peer memory driver container |
| `nvPeerDriver.repository` | string | `mellanox` | NVIDIA Peer memory driver image repository |
| `nvPeerDriver.image` | string | `nv-peer-mem-driver` | NVIDIA Peer memory driver image name  |
| `nvPeerDriver.version` | string | `1.1-0` | NVIDIA Peer memory driver version  |
| `nvPeerDriver.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the NVIDIA Peer memory driver image |
| `nvPeerDriver.gpuDriverSourcePath` | string | `/run/nvidia/driver` | GPU driver soruces root filesystem path(usually used in tandem with [gpu-operator](https://github.com/NVIDIA/gpu-operator)) |

#### RDMA Device Plugin

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `rdmaSharedDevicePlugin.deploy` | bool | `true` | Deploy RDMA Shared device plugin  |
| `rdmaSharedDevicePlugin.repository` | string | `mellanox` | RDMA Shared device plugin image repository |
| `rdmaSharedDevicePlugin.image` | string | `k8s-rdma-shared-dev-plugin` | RDMA Shared device plugin image name  |
| `rdmaSharedDevicePlugin.version` | string | `v1.1.0` | RDMA Shared device plugin version  |
| `rdmaSharedDevicePlugin.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the RDMA Shared device plugin image |
| `rdmaSharedDevicePlugin.resources` | list | See below | RDMA Shared device plugin resources |

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

#### SR-IOV Network Device plugin

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `sriovDevicePlugin.deploy` | bool | `true` | Deploy SR-IOV Network device plugin  |
| `sriovDevicePlugin.repository` | string | `docker.io/nfvpe` | SR-IOV Network device plugin image repository |
| `sriovDevicePlugin.image` | string | `sriov-device-plugin` | SR-IOV Network device plugin image name  |
| `sriovDevicePlugin.version` | string | `v3.3` | SR-IOV Network device plugin version  |
| `sriovDevicePlugin.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the SR-IOV Network device plugin image |
| `sriovDevicePlugin.resources` | list | See below | SR-IOV Network device plugin resources |

##### SR-IOV Network Device Plugin Resource configurations

Consists of a list of RDMA resources each with a name and selector of RDMA capable network devices
to be associated with the resource. Refer to [SR-IOV Network Device Plugin Selectors](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin#device-selectors) for supported selectors.

```
resources:
    - name: hostdev
      vendors: [15b3]
``` 

>__Note__: The parameter listed are non-exhaustive, for the full list of chart parameters refer to
the file: `values.yaml`

#### Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `secondaryNetwork.deploy` | bool | `true` | Deploy Secondary Network  |

Specifies components to deploy in order to facilitate a secondary network in Kubernetes. It consists of the following optionally deployed components:
  - [Multus-CNI](https://github.com/k8snetworkplumbingwg/multus-cni): Delegate CNI plugin to support secondary networks in Kubernetes
  - CNI plugins: Currently only [containernetworking-plugins](https://github.com/containernetworking/plugins) is supported
  - IPAM CNI: Currently only [Whereabout IPAM CNI](https://github.com/k8snetworkplumbingwg/whereabouts) is supported

##### CNI Plugin Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `cniPlugins.deploy` | bool | `true` | Deploy CNI Plugins Secondary Network  |
| `cniPlugins.image` | string | `containernetworking-plugins` | CNI Plugins image name  |
| `cniPlugins.repository` | string | `mellanox` | CNI Plugins image repository  |
| `cniPlugins.version` | string | `v0.8.7` | CNI Plugins image version  |
| `cniPlugins.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the CNI Plugins image |

##### Multus CNI Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `multus.deploy` | bool | `true` | Deploy Multus Secondary Network  |
| `multus.image` | string | `multus` | Multus image name  |
| `multus.repository` | string | `nfvpe` | Multus image repository  |
| `multus.version` | string | `v3.6` | Multus image version  |
| `multus.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the Multus image |
| `multus.config` | string | `` | Multus CNI config, if empty then config will be automatically generated from the CNI configuration file of the master plugin (the first file in lexicographical order in cni-conf-dir)  |

##### IPAM CNI Plugin Secondary Network

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `ipamPlugin.deploy` | bool | `true` | Deploy IPAM CNI Plugin Secondary Network  |
| `ipamPlugin.image` | string | `whereabouts` | IPAM CNI Plugin image name  |
| `ipamPlugin.repository` | string | `dougbtv` | IPAM CNI Plugin image repository  |
| `ipamPlugin.version` | string | `v0.3` | IPAM CNI Plugin image version  |
| `ipamPlugin.imagePullSecrets` | list | `[]` | An optional list of references to secrets to use for pulling any of the IPAM CNI Plugin image |
## Deployment Examples

As there are several parameters that are required to be provided to create the custom resource during
operator deployment, it is recommended that a configuration file be used. While its possible to provide
override to the parameter via CLI it would simply be cumbersome.

Below are several deployment examples `values.yaml` provided to helm during installation
of the network operator in the following manner:

`$ helm install --devel -f ./values.yaml -n network-operator --create-namespace --wait network-operator mellanox/network-operator`

#### Example 1
Network Operator deployment with a specific version of OFED driver and a single RDMA resource mapped to `enp1`
netdev.

__values.yaml:__
```:yaml
deployCR: true
ofedDriver:
  deploy: true
  version: 5.3-1.0.0.1
rdmaSharedDevicePlugin:
  deploy: true
  resources:
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
rdmaSharedDevicePlugin:
  deploy: true
  resources:
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
rdmaSharedDevicePlugin:
  deploy: true
  resources:
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
rdmaSharedDevicePlugin:
  deploy: true
  resources:
    - name: rdma_shared_device_a
      vendors: [15b3]
      deviceIDs: [1017]
```
