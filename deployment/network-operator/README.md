# Nvidia Network Operator Helm Chart

Nvidia Network Operator Helm Chart provides an easy way to install, configure and manage the lifecycle of Nvidia
Mellanox network operator.

## Nvidia Network Operator

Nvidia Network Operator
leverages [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
and [Operator SDK](https://github.com/operator-framework/operator-sdk) to manage Networking related Components in order
to enable Fast networking, RDMA and GPUDirect for workloads in a Kubernetes cluster. Network Operator works in
conjunction with [GPU-Operator](https://github.com/NVIDIA/gpu-operator) to enable GPU-Direct RDMA on compatible systems.

The Goal of Network Operator is to manage _all_ networking related components to enable execution of RDMA and GPUDirect
RDMA workloads in a kubernetes cluster including:

* Mellanox Networking drivers to enable advanced features
* Kubernetes device plugins to provide hardware resources for fast network
* Kubernetes secondary network for Network intensive workloads

### Documentation

For more information please visit the official [documentation](https://docs.nvidia.com/networking/software/cloud-orchestration/index.html).

## Additional components

### Node Feature Discovery

Nvidia Network Operator relies on the existance of specific node labels to operate properly. e.g label a node as having
Nvidia networking hardware available. This can be achieved by either manually labeling Kubernetes nodes or using
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) to perform the labeling.

To allow zero touch deployment of the Operator we provide a helm chart to be used to optionally deploy Node Feature
Discovery in the cluster. This is enabled via `nfd.enabled` chart parameter.

### SR-IOV Network Operator

Nvidia Network Operator can operate in unison with SR-IOV Network Operator to enable SR-IOV workloads in a Kubernetes
cluster. We provide a helm chart to be used to optionally
deploy [SR-IOV Network Operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator) in the cluster. This is
enabled via `sriovNetworkOperator.enabled` chart parameter.

SR-IOV Network Operator can work in conjuction with [IB Kubernetes](#ib-kubernetes) to use InfiniBand PKEY Membership
Types

For more information on how to configure SR-IOV in your Kubernetes cluster using SR-IOV Network Operator refer to the
project's github.

## QuickStart

### System Requirements

* RDMA capable hardware: Mellanox ConnectX-5 NIC or newer.
* NVIDIA GPU and driver supporting GPUDirect e.g Quadro RTX 6000/8000 or Tesla T4 or Tesla V100 or Tesla V100.
  (GPU-Direct only)
* Operating Systems: Ubuntu 20.04 LTS.

> __NOTE__: ConnectX-6 Lx is not supported.

### Tested Network Adapters

The following Network Adapters have been tested with network-operator:

* ConnectX-5
* ConnectX-6 Dx

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
$ helm repo add nvidia https://helm.ngc.nvidia.com/nvidia
$ helm repo update

# Install Operator
$ helm install -n network-operator --create-namespace --wait network-operator nvidia/network-operator

# View deployed resources
$ kubectl -n network-operator get pods
```

#### Deploy Network Operator without Node Feature Discovery

By default the network operator
deploys [Node Feature Discovery (NFD)](https://github.com/kubernetes-sigs/node-feature-discovery)
in order to perform node labeling in the cluster to allow proper scheduling of Network Operator resources. If the nodes
where already labeled by other means (either deployed from upstream or deployed within another deployment), it is possible to disable the deployment of NFD by setting
`nfd.enabled=false` chart parameter and make sure that the installed version is `v0.13.2` or newer and has NodeFeatureApi enabled.

##### Deploy NFD from upstream with NodeFeatureApi enabled
```
$ export NFD_NS=node-feature-discovery
$ helm repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts
$ helm repo update
$ helm install nfd/node-feature-discovery --namespace $NFD_NS --create-namespace --generate-name --set enableNodeFeatureApi='true'
```
For additional information , refer to the official [NVD deployment with Helm](https://kubernetes-sigs.github.io/node-feature-discovery/v0.13/deployment/helm.html)

##### Deploy Network Operator without Node Feature Discovery
```
$ helm install --set nfd.enabled=false -n network-operator --create-namespace --wait network-operator nvidia/network-operator
```

##### Currently the following NFD labels are used:

| Label                                         | Where                                             |
|-----------------------------------------------|---------------------------------------------------|
| `feature.node.kubernetes.io/pci-15b3.present` | Nodes bearing Nvidia Mellanox Networking hardware |
| `nvidia.com/gpu.present`                      | Nodes bearing Nvidia GPU hardware                 |

> __Note:__ The labels which Network Operator depends on may change between releases.

> __Note:__ By default the operator is deployed without an instance of `NicClusterPolicy` and `MacvlanNetwork`
> custom resources. The user is required to create it later with configuration matching the cluster or use chart parameters to deploy it together with the operator.

#### Deploy development version of Network Operator

To install development version of Network Operator you need to clone repository first and install helm chart from the
local directory:

```
# Clone Network Operatro Repository
$ git clone https://github.com/Mellanox/network-operator.git

# Update chart dependencies
$ cd network-operator/deployment/network-operator && helm dependency update

# Install Operator
$ helm install -n network-operator --create-namespace --wait network-operator ./

# View deployed resources
$ kubectl -n network-operator get pods
```

#### Deploy Network Operator with Admission Controller

The Admission Controller can be optionally included as part of the Network Operator installation process.  
It has the capability to validate supported Custom Resource Definitions (CRDs), which currently include NicClusterPolicy and HostDeviceNetwork.  
By default, the deployment of the admission controller is disabled. To enable it, you must set `operator.admissionController.enabled` to `true`.
  
Enabling the admission controller provides you with two options for managing certificates.  
You can either utilize [cert-manager](https://cert-manager.io/docs/installation/) for generating a self-signed certificate automatically, or you can provide your own self-signed certificate.  
  
To use `cert-manager`, ensure that `operator.admissionController.useCertManager` is set to `true`. Additionally, make sure that you deploy cert-manager before initiating the Network Operator deployment.
  
If you prefer not to use `cert-manager`, set `operator.admissionController.useCertManager` to `false`, and then provide your custom certificate and key using `operator.admissionController.certificate.tlsCrt` and `operator.admissionController.certificate.tlsKey`.

> __NOTE__: When using your own certificate, the certificate must be valid for <Release_Name>-webhook-service.<
> Release_Namespace>.svc, e.g. network-operator-webhook-service.network-operator.svc  

> __NOTE__: When deploying network operator with admission controller using helm, you need to append `--wait` to helm install and helm upgrade commands
>

##### Generating self-signed certificate using OpenSSL

To generate a self-signed SSL certificate valid for a specific hostname, you can use the `openssl` command-line tool.
First, navigate to the directory where you want to store your certificate and key files. Then, run the following
command:

```bash
SVCNAME="network-operator-webhook-service.network-operator.svc"
openssl req -x509 -nodes -batch -newkey rsa:2048 -keyout server.key -out server.crt -days 365 -addext "subjectAltName=DNS:$SVCNAME"
```

Replace `SVCNAME` with the SVC name follows this convention <Release_Name>-webhook-service.<Release_Namespace>.svc.
This command will generate a new RSA key pair with 2048 bits and create a self-signed certificate (`server.crt`) and
private key (`server.key`) that are valid for 365 days.

## Helm Tests

Network Operator has Helm tests to verify deployment. To run tests it is required to set the following chart parameters
on helm install/upgrade: `deployCR`, `rdmaSharedDevicePlugin`, `secondaryNetwork` as the test depends
on `NicClusterPolicy` instance being deployed by Helm. Supported Tests:

- Device Plugin Resource: This test creates a pod that requests the first resource in `rdmaSharedDevicePlugin.resources`
- RDMA Traffic: This test creates a pod that test loopback RDMA traffic with `rping`

Run the helm test with following command after deploying network operator with helm

```
$ helm test -n network-operator network-operator --timeout=5m
```

Notes:

- Test will keeping running endlessly if pod creating failed so it is recommended to use `--timeout` which fails test
  after exceeding given timeout
- Default PF to run test is `ens2f0` to override it add `--set test.pf=<pf_name>` to `helm install/upgrade`
- Tests should be executed after `NicClusterPolicy` custom resource state is `Ready`
- In case of a test failed it is possible to collect the logs with `kubectl logs -n <namespace> <test-pod-name>`

## Upgrade

> __NOTE__: Upgrade capabilities are limited now. Additional manual actions required when containerized OFED driver is used

Before starting the upgrade to a specific release version, please, check release notes for this version to ensure that
no additional actions are required.


### Check available releases

```
helm search repo nvidia/network-operator -l
```

> __NOTE__: add `--devel` option if you want to list beta releases as well

### Upgrade CRDs to compatible version

The network-operator helm chart contains a hook(pre-install, pre-upgrade)
that will automatically upgrade required CRDs in the cluster.
The hook is enabled by default. If you don't want to upgrade CRDs with helm automatically, 
you can disable auto upgrade by setting `upgradeCRDs: false` in the helm chart values.
Then you can follow the guide below to download and apply CRDs for the concrete version of the network-operator.

It is possible to retrieve updated CRDs from the Helm chart or from the release branch on GitHub. Example bellow show
how to download and unpack Helm chart for specified release and then apply CRDs update from it.

```
helm pull nvidia/network-operator --version <VERSION> --untar --untardir network-operator-chart
```

> __NOTE__: `--devel` option required if you want to use the beta release

```
kubectl apply -f network-operator-chart/network-operator/crds \
              -f network-operator-chart/network-operator/charts/sriov-network-operator/crds
```

### Prepare Helm values for the new release

Download Helm values for the specific release

```
helm show values nvidia/network-operator --version=<VERSION> > values-<VERSION>.yaml
```

Edit `values-<VERSION>.yaml` file as required for your cluster. The network operator has some limitations about which
updates in NicClusterPolicy it can handle automatically. If the configuration for the new release is different from the
current configuration in the deployed release, then some additional manual actions may be required.

Known limitations:

- If component configuration was removed from the NicClusterPolicy, then manual clean up of the component's resources
  (DaemonSets, ConfigMaps, etc.) may be required
- If configuration for devicePlugin changed without image upgrade, then manual restart of the devicePlugin may be
  required

These limitations will be addressed in future releases.

> __NOTE__: changes which were made directly in NicClusterPolicy CR (e.g. with `kubectl edit`)
> will be overwritten by Helm upgrade due to the `force` flag.

### Apply Helm chart update

```
helm upgrade -n network-operator  network-operator nvidia/network-operator --version=<VERSION> -f values-<VERSION>.yaml --force
```

> __NOTE__: `--devel` option required if you want to use the beta release

### Enable automatic upgrade for containerized OFED driver (recommended)

> __NOTE__: this operation is required only if **containerized OFED** is in use

Check [Automatic OFED upgrade](../../docs/automatic-ofed-upgrade.md) document for more details.

### OR manually restart PODs with containerized OFED driver

> __NOTE__: this operation is required only if **containerized OFED** is in use

When containerized OFED driver reloaded on the node, all PODs which use secondary network based on NVIDIA Mellanox NICs
will lose network interface in their containers. To prevent outage you need to remove all PODs which use secondary
network from the node before you reload the driver POD on it.

Helm upgrade command will just upgrade DaemonSet spec of the OFED driver to point to the new driver version. The OFED
driver's DaemonSet will not automatically restart PODs with the driver on the nodes because it uses "OnDelete"
updateStrategy. The old OFED version will still run on the node until you explicitly remove the driver POD or reboot the
node.

It is possible to remove all PODs with secondary networks from all cluster nodes and then restart OFED PODs on all nodes
at once.

The alternative option is to do upgrade in a rolling manner to reduce the impact of the driver upgrade on the cluster.
The driver POD restart can be done on each node individually. In this case, PODs with secondary networks should be
removed from the single node only, no need to stop PODs on all nodes.

Recommended sequence to reload the driver on the node:

_For each node follow these steps_

- [Remove PODs with secondary network from the node](#remove-pods-with-secondary-network-from-the-node)
- [Restart OFED driver POD](#restart-ofed-driver-pod)
- [Return PODs with secondary network to the node](#return-pods-with-secondary-network-to-the-node)

_When the OFED driver becomes ready, proceed with the same steps for other nodes_

#### Remove PODs with secondary network from the node

This can be done with node drain command:

```
kubectl drain <NODE_NAME> --pod-selector=<SELECTOR_FOR_PODS>
```

> __NOTE__: replace <NODE_NAME> with `-l "network.nvidia.com/operator.mofed.wait=false"` if you
> want to drain all nodes at once

#### Restart OFED driver POD

Find OFED driver POD name for the node

```
kubectl get pod -l app=mofed-<OS_NAME> -o wide -A
```

_example for Ubuntu 20.04: `kubectl get pod -l app=mofed-ubuntu20.04 -o wide -A`_

Delete OFED driver POD from the node

```
kubectl delete pod -n <DRIVER_NAMESPACE> <OFED_POD_NAME>
```

> __NOTE__: replace <OFED_POD_NAME> with `-l app=mofed-ubuntu20.04` if you
> want to remove OFED PODs on all nodes at once

New version of the OFED POD will automatically start.

#### Return PODs with secondary network to the node

After OFED POD is ready on the node you can make node schedulable again.

The command below will uncordon (remove `node.kubernetes.io/unschedulable:NoSchedule` taint)
the node and return PODs to it.

```
kubectl uncordon -l "network.nvidia.com/operator.mofed.wait=false"
```

## Chart parameters

In order to tailor the deployment of the network operator to your cluster needs, Chart parameters are available. See official [documentation](https://docs.nvidia.com/networking/software/cloud-orchestration/index.html).

## Deployment Examples

As there are several parameters that are required to be provided to create the custom resource during operator
deployment, it is recommended that a configuration file be used. While its possible to provide override to the parameter
via CLI it would simply be cumbersome.

Below are several deployment examples `values.yaml` provided to helm during installation of the network operator in the
following manner:

`$ helm install -f ./values.yaml -n network-operator --create-namespace --wait network-operator nvidia/network-operator`

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

Network Operator deployment with the default version of OFED, RDMA device plugin with two RDMA
resources, the first mapped to `enp1` and `enp2`, the second mapped to `ib0`.

__values.yaml:__

```:yaml
deployCR: true
ofedDriver:
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
    - Multus CNI
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

Network Operator deployment with the default version of RDMA device plugin with RDMA resource mapped to Mellanox
ConnectX-5.

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
