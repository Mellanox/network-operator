#!/bin/bash

echo "Deleting Secondary Network with Whareabouts IPAM"
echo "################################################"
kubectl delete -f deploy/crds/networking/rdma-net-cr-whereabouts-ipam.yml
kubectl delete -f deploy/networking/whareabouts-daemonset-install.yaml
kubectl delete -f deploy/networking/whereabouts.cni.cncf.io_ippools.yaml
kubectl delete -f deploy/networking/multus-daemonset.yml
