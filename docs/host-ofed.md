# NVIDIA Mellanox OFED in Kubernetes

This document describes how to safely install or upgrade the NVIDIA Mellanox OFED driver
on the host which is a part of the Kubernetes cluster.
This guide assumes that the NVIDIA network-operator is used to manage secondary networking in the Kubernetes cluster.

>__NOTE__:  To upgrade containerized OFED driver,
> you should follow the [network-operator upgrade guide](../README.md#upgrade) instead.

- [NVIDIA Mellanox OFED in Kubernetes](#nvidia-mellanox-ofed-in-kubernetes)
    * [When no special actions required](#when-no-special-actions-required)
      * [Install NVIDIA Mellanox OFED](#install-nvidia-mellanox-ofed)
      * [Upgrade NVIDIA Mellanox OFED](#upgrade-nvidia-mellanox-ofed)
      * [Remove NVIDIA Mellanox OFED](#remove-nvidia-mellanox-ofed)
      * [Examples](#examples)
        + [Remove all PODs which using secondary network from the host](#remove-all-pods-which-using-secondary-network-from-the-host)
          + [Remove network-operator and sriov-network-operator components from the host](#remove-network-operator-and-sriov-network-operator-components-from-the-host)
          + [Return network-operator and sriov-network-operator components to the host](#return-network-operator-and-sriov-network-operator-components-to-the-host)


## When no special actions required

It is safe to do any operation (install, remove, upgrade) with NVIDIA Mellanox OFED  on the host if it:
- is not a part of Kubernetes cluster;
- was temporarily removed from the cluster and all Kubernetes components are stopped on it;
- is a part of the Kubernetes cluster but doesn't expose any network capabilities
managed by network-operator or sriov-network-operator;
  
No special handling required in these cases, installation should be done by following the NVIDIA Mellanox OFED installation guide.


## Install NVIDIA Mellanox OFED

If the host is a part of the Kubernetes cluster and runs PODs with a secondary network,
then additional actions are required to safely install OFED on the host.

**Required steps:**
  - [Remove all PODs which using secondary network from the host](#remove-all-pods-which-using-secondary-network-from-the-host)
  - [Remove network-operator and sriov-network-operator components from the host](#remove-network-operator-and-sriov-network-operator-components-from-the-host)
  - Install NVIDIA Mellanox OFED by following the [documentation](https://docs.mellanox.com/category/mlnxofedib)
  - Reboot the host if firmware upgrade was installed during the OFED installation
  - [Return network-operator and sriov-network-operator components to the host](#return-network-operator-and-sriov-network-operator-components-to-the-host)
  - [Return PODs to the host if required](#return-pods-to-the-host-if-required)

## Upgrade NVIDIA Mellanox OFED

If NIC FW upgrade will be applied during the upgrade it is recommended to follow
the same steps as in the previous section.

If node reboot is not required during driver upgrade it is possible to follow simplified upgrade procedure.
In case of driver upgrade you need to remove PODs with secondary network only,
there is no need to remove network-operator or sriov-network-operator components because OFED upgrade procedure
should preserve the same network devices names after driver will be reloaded to the newer version.

**Required steps:**
  - [Remove all PODs which using secondary network from the host](#remove-all-pods-which-using-secondary-network-from-the-host)
  - Install NVIDIA mellanox OFED upgrade
  - [Return PODs to the host if required](#return-pods-to-the-host-if-required)

## Remove NVIDIA Mellanox OFED

Requirements for OFED removal are mostly the same as for OFED installation except requirement for host reboot.

**Required steps:**
  - [Remove all PODs which using secondary network from the host](#remove-all-pods-which-using-secondary-network-from-the-host)
  - [Remove network-operator and sriov-network-operator components from the host](#remove-network-operator-and-sriov-network-operator-components-from-the-host)
  - Remove NVIDIA Mellanox OFED by following the [documentation](https://docs.mellanox.com/category/mlnxofedib)
  - [Return network-operator and sriov-network-operator components to the host](#return-network-operator-and-sriov-network-operator-components-to-the-host)
  - [Return PODs to the host if required](#return-pods-to-the-host-if-required)


## Examples

### Remove all PODs which using secondary network from the host

#### How to find PODs which using secondary network

Secondary Networks in Kubernetes are usually represented by NetworkAttachmentDefinition CRD.
Single NetworkAttachmentDefinition CR represents a single secondary network.
To check which secondary networks exist in the cluster usually is enough to check which NetworkAttachmentDefinition CRs
are present in the cluster.

```
kubectl get network-attachment-definitions.k8s.cni.cncf.io -A
```

PODs that use secondary networks should have `k8s.v1.cni.cncf.io/networks` annotation
It is possible to filter PODs that use secondary networks by using this snippet:

```
kubectl get po -o=jsonpath=\
'{range .items[?(@.metadata.annotations.k8s\.v1\.cni\.cncf\.io/networks!="")]}'\
'{.metadata.annotations.k8s\.v1\.cni\.cncf\.io/networks}{"\t"}'\
'{.metadata.namespace}/{.metadata.name}{"\t"}'\
'{.spec.nodeName}{"\n"}'\
'{end}'
```

These actions can provide insight into which PODs use secondary networks in the cluster.

#### How to remove PODs from the node
There is no single command which can remove selected PODs from the node in all cases.
The exact method how to remove PODs will vary depending on which Kubernetes controller manages these PODs.
This section will provide common recommendations about how to move PODs.  
For PODs that are managed by Deployment or ReplicaSet controller,
the simplest way to remove PODs from the node will be to use `kubectl drain` command.

```
kubectl drain <node_id>  --ignore-daemonsets=true
```

This command will evict PODs from the node and make this node unscheduled for new PODs.

`kubectl drain` command can't remove PODs that managed by DaemonSet controller

To remove PODs which is a part of DaemonSet you can add affinity rule which will
explicitly disallow running on a specific node.

```
apiVersion: apps/v1
kind: DaemonSet
metadata:
  ...
spec:
  ...
  template:
    metadata:
      ...
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/hostname
                operator: NotIn
                values:
                - <NODE_NAME>
```

### Remove network-operator and sriov-network-operator components from the host

#### network-operator components
`NicClusterPolicy.nodeAffinity` configuration option can be used to limit nodes on which 
network-operator components runs. To remove network-operator from the node you can set nodeAffinity 
rule which will explicitly deny network-operator components to run on the required node.

To add additional nodeAffinity rule you can edit nic-cluster-policy with `kubectl edit`
```
kubectl edit nicclusterpolicies.mellanox.com nic-cluster-policy
```

```
apiVersion: mellanox.com/v1alpha1
kind: NicClusterPolicy
...
spec:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: NotIn
          values:
          - <NODE_NAME>
...
```

#### sriov-network-operator components
To remove sriov-network-operator components from the node you need to remove
`network.nvidia.com/operator.mofed.wait` and temporary prevent it from be restored by network-operator

1. remove node-feature-discovery POD from the node, follow 
[How to remove PODs from the node](#how-to-remove-pods-from-the-node)
2. remove `feature.node.kubernetes.io/pci-15b3.present` label from the node
```
kubectl label node <NODE_NAME> feature.node.kubernetes.io/pci-15b3.present-
kubectl label node <NODE_NAME> feature.node.kubernetes.io/pci-15b3.sriov.capable-
```
3. remove `network.nvidia.com/operator.mofed.wait` label from the node
```
kubectl label node <NODE_NAME> network.nvidia.com/operator.mofed.wait-
```

After these steps sriov-network-operator components should be removed from the node

### Return network-operator and sriov-network-operator components to the host
#### network-operator components
Remove nodeAffinity rule which was added before for the node in `NicClusterPolicy`

#### sriov-network-operator components
1. restore node-feature-discovery POD on the node, it should restore required labels automatically
2. network-operator will restore `network.nvidia.com/operator.mofed.wait` label and sriov-network-operator components
will start on the node

>__NOTE__: sriov-network-operator will remove "unschedulable" taint from the node automatically
> when it restores its components on the node

### Return PODs to the host if required

1. Remove nodeAffinity which where added for DaemonSets
2. Remove "unschedulable" taint from the Node
```
kubectl uncordon <NODE_NAME>
```
