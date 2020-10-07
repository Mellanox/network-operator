#!/bin/bash
# Copyright 2020 NVIDIA
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

echo "Deploying Network Operator:"
echo "###########################"
kubectl apply -f deploy/operator-ns.yaml
kubectl apply -f deploy/operator-resources-ns.yaml
kubectl apply -f deploy/crds/mellanox.com_nicclusterpolicies_crd.yaml
kubectl apply -f deploy/crds/mellanox.com_macvlannetworks_crd.yaml
kubectl apply -f deploy/role.yaml
kubectl apply -f deploy/service_account.yaml
kubectl apply -f deploy/role_binding.yaml
kubectl apply -f deploy/operator.yaml
