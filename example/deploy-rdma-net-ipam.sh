#!/bin/bash

echo "Deploying Secondary Network with Whareabouts IPAM: \"rdma-net-ipam\" with RDMA resource : \"rdma/hca_shared_devices_a\""
echo "#######################################################################################################################"
kubectl apply -f networking/multus-daemonset.yml
kubectl apply -f networking/whereabouts.cni.cncf.io_ippools.yaml
kubectl apply -f networking/whareabouts-daemonset-install.yaml
kubectl apply -f networking/rdma-net-cr-whereabouts-ipam.yml

