#!/bin/bash

echo "Deleting Secondary Network"
echo "##########################"
kubectl delete -f deploy/crds/networking/rdma-net-cr.yml
kubectl delete -f deploy/networking/multus-daemonset.yml
