#!/bin/bash -e
# Copyright 2022 NVIDIA CORPORATION & AFFILIATES.
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

set -o pipefail
set +e
this=`basename $0`

usage () {
	cat << EOF
	Usage: $this [-h] [--mofed] [--nvpeer] [--shared-dp] [--sriov-dp] [--cni-plugins] [--multus] [--whereabouts] <version>
Options:
  -h         show this help and exit
  --mofed          update MOFED image version
  --nvpeer         update nv-peer-mem image version
  --shared-dp      update RDMA Shared Device Plugin image version
  --sriov-dp       update SR-IOV Device Plugin image version
  --cni-plugins    update CNI Plugins image version
  --multus         update Multus CNI image version
  --whereabouts    update Whereabouts IPAM Plugin image version
EOF
}


updateVersion() {
	old=`grep $1 deployment/network-operator/values.yaml  -A 4 | grep version | awk '{ print $2 }'`
	new=$2
	grep -l -r $old ./deployment ./example | xargs sed -i 's/'"$old"'/'"$new"'/g'
}

updateMofed() {
	updateVersion "ofedDriver" $1
}

updateNvpeer() {
	updateVersion "nvPeerDriver" $1
}

updateShared() {
	updateVersion "rdmaSharedDevicePlugin" $1
}

updateSriov() {
	updateVersion "sriovDevicePlugin" $1
}

updateCni() {
	updateVersion "cniPlugins" $1
}

updateMultus() {
	updateVersion "multus" $1
}

whereabouts() {
	updateVersion "ipamPlugin" $1
}

ARGUMENT_LIST=(
  "mofed"
  "nvpeer"
  "shared-dp"
  "sriov-dp"
  "cni-plugins"
  "multus"
  "whereabouts"
)

opts=$(getopt \
  --longoptions "$(printf "%s:," "${ARGUMENT_LIST[@]}")" \
  --name "$(basename "$0")" \
  --options "" \
  -- "$@"
)

eval set --$opts

new_version=$2
case "$1" in
    --mofed)
  updateMofed $new_version
  exit 0
        ;;
--nvpeer)
  updateNvpeer $new_version
  exit 0
        ;;
--shared-dp)
  updateShared $new_version
  exit 0
        ;;
--sriov-dp)
  updateSriov $new_version
  exit 0
        ;;
--cni-plugins)
  updateCni $new_version
  exit 0
        ;;
--multus)
  updateMultus $new_version
  exit 0
        ;;
--whereabouts)
  updateWhereabouts $new_version
  exit 0
        ;;
    h)  usage
        exit 0
        ;;
    *)  usage
        exit 1
        ;;
esac

