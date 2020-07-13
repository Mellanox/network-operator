# Overview
This directory contains a collection of sample `.yml` files and scripts to facilitate the deployment
of network-operator and related components in order to prepare a cluster for running _RDMA_ workloads
and _GPU-Direct RDMA_ workloads.

We assume familiarity with RDMA, Kubernetes and related CNI project.

network-operator, at this stage, deploys and configures the follwoing components:
* [Mellanox OFED](https://www.mellanox.com/products/infiniband-drivers/linux/mlnx_ofed) driver container
* [RDMA device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin)
* [NVIDIA peer memory client](https://github.com/Mellanox/nv_peer_memory) driver container

There are still additional pieces that need to be setup before _RDMA_ or _GPU-Direct RDMA_ workloads
can run on a Kubernetes cluster.

__Those being:__
1. Configuration of persistent and predictable network device names
2. Network device initialization upon system boot via network init mechanism
(e.g `network-scripts`, `NetworkManager`, `netplan`)
3. Deployment of Kubernetes secondary network of type `macvlan` on the RDMA device's netdev.
4. Secondary network IPAM.
5. Proper definition of workloads to request RDMA device and its corresponding secondary network, GPU.

This example aims to address the points above in a basic manner.

## Content
* `deploy-operator.sh`: Deploy network operator and related resources.
* `deploy-rdma-net.sh`: Deploy secondary network for RDMA traffic with static IPAM.
* `deploy-rdma-net-ipam.sh`: Deploy secondary network for RDMA traffic with cluster IPAM
([wherabouts](https://github.com/openshift/whereabouts-cni)).
* `delete-operator.sh`: Delete network operator and related resources.
* `delete-rdma-net.sh`: Delete secondary network for RDMA traffic with static IPAM.
* `delete-rdma-net-ipam.sh`: Delete secondary network for RDMA traffic with cluster IPAM
([wherabouts](https://github.com/openshift/whereabouts-cni)).
* `deploy`: network-operator related deployment files
* `networking`: kubernetes secondary network and IPAM related deployment files 
* `network-init`: Example files on how to configure network device initialization.
* `udev`: Example udev rules for persistend and predictable network device names.
* `README.md`: This file.


## Example
__The example assumes the follwoing:__
1. [GPU operator](https://github.com/NVIDIA/gpu-operator) is already deployed in the cluster
2. Nodes with Mellanox Ethernet NICs are already labeled with `feature.node.kubernetes.io/pci-15b3.present=true`
3. RDMA network device name is `ens2f0` on all relevant nodes, interface is up with MTU of 9000

>__NOTE__: To achieve _3._ `udev` rules can be applied and init script can be configured on the setup.
>refer to `./udev` and `./network-init` for examples

### Setup cluster
 ```
# ./deploy-operator.sh
...
# ./deploy-rdma-net-ipam.sh
...
# kubectl apply -f ./deploy/crds/mellanox.com_v1alpha1_nicclusterpolicy_cr.yaml
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
# ib_write_bw -d <RDMA device e.g mlx5_0> -a -F --report_gbits -R -q 2
```

__Pod2:__ Run `ib_write_bw` as client
```bash
# ib_write_bw -d <RDMA device e.g mlx5_0> -a -F --report_gbits -R -q 2 <Pod1 IP address>
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
# ./delete-operator.sh
# ./delete-rdma-net-ipam.sh
```
