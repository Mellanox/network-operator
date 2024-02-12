#!/bin/bash

#  2024 NVIDIA CORPORATION & AFFILIATES
#
#  Licensed under the Apache License, Version 2.0 (the License);
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an AS IS BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

set -o nounset
set -o pipefail
set -o errexit

if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

CLUSTER_NAME="${CLUSTER_NAME:-"net-op-dev"}"
MINIKUBE_BIN="${MINIKUBE_BIN:-"unknown"}"
MINIKUBE_DRIVER="${MINIKUBE_DRIVER:-"unknown"}"
USE_MINIKUBE_DOCKER="${USE_MINIKUBE_DOCKER:-"true"}"
NUM_NODES="${NUM_NODES:-"1"}"
NODE_MEMORY="${NODE_MEMORY:-"4g"}"
NODE_CPUS="${NODE_CPUS:-"2"}"
NODE_DISK="${NODE_DISK:-"20g"}"

## Detect the OS.
OS="unknown"
if [[ "${OSTYPE}" == "linux"* ]]; then
  OS="linux"
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  OS="darwin"
fi

# Exit if the OS is not supported.
if [[ "$OS" == "unknown" ]]; then
  echo "os '$OSTYPE' not supported. Aborting." >&2
  exit 1
fi

## Set the driver used for minikube machines. By default this script will select the preferred VM driver for each OS.
## Users can override this using the MINIKUBE_DRIVER env variable.
if [[ "$MINIKUBE_DRIVER" == "unknown" ]]; then
  if [[ "${OSTYPE}" == "linux"* ]]; then
    MINIKUBE_DRIVER="kvm2"
  elif [[ "${OSTYPE}" == "darwin"* ]]; then
    MINIKUBE_DRIVER="qemu"
  fi
fi

MINIKUBE_ARGS="${MINIKUBE_ARGS:-"\
  --driver $MINIKUBE_DRIVER \
  --cpus=$NODE_CPUS \
  --memory=$NODE_MEMORY \
  --disk-size=$NODE_DISK \
  --nodes=$NUM_NODES \
  --preload=true \
  --cni calico \
  --container-runtime=docker\
  --install-addons \
  --addons registry"}"

MINIKUBE_EXTRA_ARGS="${MINIKUBE_EXTRA_ARGS:-""}"

echo "Setting up Minikube cluster..."

## Exit early if the cluster already exists.
if [[ $($MINIKUBE_BIN status -p $CLUSTER_NAME  -f '{{.Name}}') == $CLUSTER_NAME ]]; then
  echo "Minikube cluster '$CLUSTER_NAME' found. Skipping cluster set-up"
  ## Set environment variables to use the Minikube VM docker build for
  if [[ "$USE_MINIKUBE_DOCKER" == "true" ]]; then
    echo "Setting environment variables to use $CLUSTER_NAME as docker build environment."
    eval $($MINIKUBE_BIN -p $CLUSTER_NAME docker-env)
  fi
  exit 0
fi

$MINIKUBE_BIN start --profile $CLUSTER_NAME $MINIKUBE_ARGS $MINIKUBE_EXTRA_ARGS

## Set environment variables to use the Minikube VM docker build for
if [[ "$USE_MINIKUBE_DOCKER" == "true" ]]; then
  echo "Setting environment variables to use $CLUSTER_NAME as docker build environment."
  eval $($MINIKUBE_BIN -p $CLUSTER_NAME docker-env)
fi
