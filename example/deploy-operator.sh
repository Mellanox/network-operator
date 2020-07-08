#!/bin/bash

echo "Deploying Network Operator:"
echo "###########################"
kubectl apply -f deploy/operator-ns.yaml
kubectl apply -f deploy/crds/mellanox.com_nicclusterpolicies_crd.yaml
kubectl apply -f deploy/role.yaml
kubectl apply -f deploy/service_account.yaml
kubectl apply -f deploy/role_binding.yaml
kubectl apply -f deploy/operator.yaml
