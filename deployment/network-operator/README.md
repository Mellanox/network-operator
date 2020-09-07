# Nvidia Mellanox Network Operator Helm Chart

Nvidia Mellanox Network Operator Helm Chart provides an easy way to install, configure and manage
the lifecycle of Nvidia Mellanox network operator.

## Nvidia Mellanox Network Operator
Nvidia Mellanox Network Operator leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order to enable Fast networking, 
RDMA and GPUDirect for workloads in a Kubernetes cluster.

The Goal of Network Operator is to manage _all_ networking related components to enable execution of
RDMA and GPUDirect RDMA workloads in a kubernetes cluster including:
* Mellanox Networking drivers to enable advanced features
* Kubernetes device plugins to provide hardware resources for fast network
* Kubernetes secondary network for Network intensive workloads

## QuickStart

### Prerequisites

- Kubernetes v1.15+
- Helm v3
- Ubuntu 18.04LTS


### Install Helm

Helm provides an install script to copy helm binary to your system:
```
$ curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
$ chmod 700 get_helm.sh
$ ./get_helm.sh
```

For additional information and methods for installing Helm, refer to the official [helm website](https://helm.sh/)

### Deploy Network Operator

```
# Add Repo
$ helm repo add mellanox https://mellanox.github.io/network-operator
$ helm repo update

# Install Operator
$ helm install -n network-operator --create-namespace --wait mellanox/network-operator

# View deployed resources
$ kubectl -n network-operator get pods
```

>__Note:__ By default the operator is deployed without an instance of `NicClusterPolicy`
custom resource. The user is required to create it later with configuration matching the cluster or use
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
| `operator.tag` | string | `None` | Network Operator image tag |
| `deployCR` | bool | `false` | Deploy `NicClusterPolicy` custom resource according to provided parameters |

### Custom resource parameters

#### Mellanox OFED driver

| Name | Type | Default | description |
| ---- | ---- | ------- | ----------- |
| `ofedDriver.deploy` | bool | `false` | deploy Mellanox OFED driver container |
| `ofedDriver.repository` | string | `mellanox` | Mellanox OFED driver image repository |
| `ofedDriver.image` | string | `ofed-driver` | Mellanox OFED driver image name  |
| `ofedDriver.version` | string | `5.0-2.1.8.0` | Mellanox OFED driver version  |

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
| `devicePlugin.version` | string | `v1.0.0` | Device plugin version  |
| `devicePlugin.resources` | list | See below | Device plugin resources |

##### RDMA Device Plugin Resource configurations

Consists of a list of RDMA resources each with a name a a list of RDMA capable network devices
to be associated with the resource.

```
resources:
    - name: rdma_shared_device_a
      devices: [enp5s0f0]
    - name: rdma_shared_device_b
      devices: [ib0, ib1]
``` 

>__Note__: The parameter listed are non-exahustive, for the full list of chart parameters refer to
the file: `values.yaml`

## Deployment Examples

As there are several parameters that are required to be provided to create the custom resource during
operator deployment, it is recommended that a configuration file be used. While its possible to provide
override to the parameter via CLI it would simply be cumbersome.

Below are several deployment examples `values.yaml` provided to helm during installation
of the network operator in the following manner:

`$ helm install -f ./values.yaml -n network-operator --create-namespace --wait mellanox/network-operator `

#### Example 1
Network Operator deployment with a specific version of OFED driver and a single RDMA resource mapped to `enp1`
netdev.

__values.yaml:__
```:yaml
deployCR: true
ofedDriver:
  deploy: true
  version: 5.0-2.1.8.0
devicePlugin:
  deploy: true
  reources:
    - name: rdma_shared_device_a
      devices: [enp1]
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
      devices: [enp1, enp2]
    - name: rdma_shared_device_b
      devices: [ib0]
```
