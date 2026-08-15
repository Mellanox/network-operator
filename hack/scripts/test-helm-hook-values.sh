#!/bin/bash

#  2026 NVIDIA CORPORATION & AFFILIATES
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

set -o nounset
set -o pipefail
set -o errexit

HELM_BIN="${1:?helm binary path is required}"
CHART_PATH="${2:?chart path is required}"

KEEP_NCP_RESOURCES=(
    "network-operator-hooks-keep-ncp-sa"
    "network-operator-hooks-keep-ncp-role"
    "network-operator-hooks-scale-binding"
    "network-operator-keep-ncp"
)

UPGRADE_CRD_RESOURCES=(
    "network-operator-hooks-sa"
    "network-operator-hooks-role"
    "network-operator-hooks-binding"
    "network-operator-upgrade-crd"
)

assert_resources() {
    local manifest="$1"
    local expected="$2"
    shift 2

    local resource
    for resource in "$@"; do
        if [[ "$expected" == "true" ]]; then
            if ! grep -Fqx "  name: $resource" <<< "$manifest"; then
                echo "expected rendered resource $resource" >&2
                exit 1
            fi
        elif grep -Fqx "  name: $resource" <<< "$manifest"; then
            echo "did not expect rendered resource $resource" >&2
            exit 1
        fi
    done
}

run_case() {
    local description="$1"
    local expect_keep_ncp="$2"
    local expect_upgrade_crd="$3"
    shift 3

    local manifest
    manifest=$("$HELM_BIN" template network-operator "$CHART_PATH" \
        --kube-version 1.32.0 \
        --namespace network-operator "$@")

    assert_resources "$manifest" "$expect_keep_ncp" "${KEEP_NCP_RESOURCES[@]}"
    assert_resources "$manifest" "$expect_upgrade_crd" "${UPGRADE_CRD_RESOURCES[@]}"
    echo "PASS: $description"
}

run_case "default hook values" true true
run_case "both hook groups enabled" true true \
    --set keepNCP=true \
    --set upgradeCRDs=true
run_case "keep NCP hooks disabled" false true \
    --set keepNCP=false \
    --set upgradeCRDs=true
run_case "CRD upgrade hooks disabled" true false \
    --set keepNCP=true \
    --set upgradeCRDs=false
run_case "all hook groups disabled" false false \
    --set keepNCP=false \
    --set upgradeCRDs=false
