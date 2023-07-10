# Overview
This directory contains a collection of sample `.yml` files and scripts to facilitate the deployment
of network-operator and related components in order to prepare a cluster for running _RDMA_ workloads
and _GPU-Direct RDMA_ workloads.

We assume familiarity with RDMA, Kubernetes and related CNI project.

network-operator, at this stage, deploys and configures the follwoing components:
* [Mellanox OFED](https://www.mellanox.com/products/infiniband-drivers/linux/mlnx_ofed) driver container
* [RDMA device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)
* SecondaryNetwork`: Specifies components to deploy in order to facilitate a secondary network in Kubernetes. It consists of the following optionally deployed components:
    - [Multus-CNI](https://github.com/intel/multus-cni): Delegate CNI plugin to support secondary networks in Kubernetes
    - CNI plugins: Currently only [containernetworking-plugins](https://github.com/containernetworking/plugins) is supported
    - IPAM CNI: Currently only [Whereabout IPAM CNI](https://github.com/dougbtv/whereabouts-cni) is supported
There are still additional    pieces that need to be setup before _RDMA_ or _GPU-Direct RDMA_ workloads
can run on a Kubernetes cluster.

__Those being:__
1. Configuration of persistent and predictable network device names
2. Network device initialization upon system boot via network init mechanism
(e.g `network-scripts`, `NetworkManager`, `netplan`)
3. Deployment of Kubernetes secondary network of type `macvlan` on the RDMA device's netdev.

This example aims to address the points above in a basic manner.

## Content
* `deploy`: network-operator related deployment files
* `networking`: kubernetes secondary network and IPAM related deployment files
* `network-init`: Example files on how to configure network device initialization.
* `udev`: Example udev rules for persistend and predictable network device names.
* `README.md`: This file.


## Example
__The example assumes the follwoing:__
1. [GPU operator](https://github.com/NVIDIA/gpu-operator) is already deployed in the cluster
>__NOTE__: network-operator is compatible with NVIDIA GPU Operator v1.5.2 and above
2. Nodes with Mellanox Ethernet NICs are already labeled with `feature.node.kubernetes.io/pci-15b3.present=true`
3. RDMA network device name is `ens2f0` on all relevant nodes, interface is up with MTU of 9000

>__NOTE__: To achieve _3._ `udev` rules can be applied and init script can be configured on the setup.
>refer to `./udev` and `./network-init` for examples

### Setup cluster
 ```
# make -C ./.. deploy
...
# kubectl apply -f ./crs/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml
```
Deploy a kubernetes secondary network from `networking` folder, there are `macvlan` and `host-device` networks each has example file for deploying with/without IPAM Whereabouts
```bash
# kubectl apply -f ./networking/<file-name>
```
At this point you should wait for the defined Custom resource to be ready.
Once ready, deploying RDMA workloads or GPU-Direct RDMA workloads should be possible.

### Deploy workloads
An example pod `.yml` are provided. `nodeSelector` would need to be modified in order
to deploy workload on specific nodes in your cluster.

Once deployed it is possible to run [pertest](https://github.com/linux-rdma/perftest)
tools to test RDMA and GPU-Direct RDMA traffic.

```
# kubectl create -f ./rdma-gpu-test-pod1.yml
# kubectl create -f ./rdma-gpu-test-pod2.yml
```

#### Run RDMA traffic between Pods

##### RDMA
__Pod1:__ Run `ib_write_bw` as server
```bash
# ib_write_bw -d <RDMA device e.g mlx5_0> -a -F --report_gbits -R
```

__Pod2:__ Run `ib_write_bw` as client
```bash
# ib_write_bw -d <RDMA device e.g mlx5_0> -a -F --report_gbits -R <Pod1 IP address>
```

##### GPU-Direct RDMA
__Pod1:__ Run `ib_write_bw` as server
```bash
# ib_write_bw -d <RDMA device e.g mlx5_0> -a -F --report_gbits -R -q 2 --use_cuda 0
```

__Pod2:__ Run `ib_write_bw` as client
```bash
# ib_write_bw -d <RDMA device e.g mlx5_0> -a -F --report_gbits -R -q 2 --use_cuda 0 <Pod1 IP address>
```

### Cleanup
```bash
# // delete workloads
# make -C ./.. undeploy
```
