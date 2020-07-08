#!/bin/bash

echo "Deploying Secondary Network: \"rdma-net\" with RDMA resource : \"rdma/hca_shared_devices_a\""
echo "############################################################################################"
kubectl apply -f networking/multus-daemonset.yml
kubectl apply -f networking/rdma-net-cr.yml
