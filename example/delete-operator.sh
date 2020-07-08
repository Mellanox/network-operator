#!/bin/bash

echo "Deleting Network Operator:"
echo "##########################"
kubectl delete -f deploy/operator.yaml
kubectl delete -f deploy/role_binding.yaml
kubectl delete -f deploy/service_account.yaml
kubectl delete -f deploy/role.yaml
kubectl delete -f deploy/crds/mellanox.com_nicclusterpolicies_crd.yaml
kubectl delete -f deploy/operator-ns.yaml
