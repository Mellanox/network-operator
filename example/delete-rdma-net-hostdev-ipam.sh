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

echo "Deleting Secondary Network with Whereabouts IPAM"
echo "################################################"
kubectl delete -f networking/rdma-net-hostdev-cr-whereabouts-ipam.yml
kubectl delete -f networking/whareabouts-daemonset-install.yaml
kubectl delete -f networking/whereabouts.cni.cncf.io_ippools.yaml
kubectl delete -f networking/multus-daemonset.yml
