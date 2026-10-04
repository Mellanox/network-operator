#!/usr/bin/env bash

#  2026 NVIDIA CORPORATION & AFFILIATES
#
#  Licensed under the Apache License, Version 2.0 (the License);
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

# Flush Go integration coverage from a network-operator pod on OpenShift.
#
# The operator image is distroless, so coverage cannot be flushed via oc exec.
# Send SIGUSR1 to the manager process on the node instead, then read covdata
# from the kubelet emptyDir path.
#
# Usage:
#   ./scripts/e2e-coverage/flush-openshift.sh [namespace] [pod-name] [output-dir]
#
# When pod-name is omitted, the first running network-operator controller pod
# in the namespace is used.

set -euo pipefail

NS="${1:-nvidia-network-operator}"
POD="${2:-}"
OUT_DIR="${3:-build/ocp-coverage-flush}"

if ! command -v oc >/dev/null 2>&1; then
  echo "error: oc is required" >&2
  exit 1
fi

if [[ -z "${POD}" ]]; then
  POD="$(oc -n "${NS}" get pod -l control-plane=nvidia-network-operator-controller \
    -o jsonpath='{.items[0].metadata.name}')"
fi

NODE="$(oc -n "${NS}" get pod "${POD}" -o jsonpath='{.spec.nodeName}')"
POD_UID="$(oc -n "${NS}" get pod "${POD}" -o jsonpath='{.metadata.uid}')"
CID="$(oc -n "${NS}" get pod "${POD}" -o jsonpath='{.status.containerStatuses[0].containerID}' \
  | sed 's|.*://||')"

VOL="/var/lib/kubelet/pods/${POD_UID}/volumes/kubernetes.io~empty-dir/go-coverage"
mkdir -p "${OUT_DIR}"
# Drop leftover covdata so a reused output dir cannot merge stale executions.
rm -f "${OUT_DIR}"/covmeta.* "${OUT_DIR}"/covcounters.*

echo "namespace=${NS} pod=${POD} node=${NODE}"
echo "containerID=${CID}"
echo "volume=${VOL}"

oc debug "node/${NODE}" -- chroot /host bash -c \
  "PID=\$(crictl inspect --output go-template --template '{{.info.pid}}' ${CID} 2>/dev/null || crictl inspect ${CID} | awk -F: '/\"pid\"/ {gsub(/[^0-9]/,\"\",\$2); print \$2; exit}'); echo manager pid=\${PID}; kill -USR1 \${PID}; sleep 3; ls -la ${VOL}; ls ${VOL}/covcounters.*"

echo "Fetching coverage data into ${OUT_DIR}"
oc debug "node/${NODE}" -- chroot /host bash -c "tar -C ${VOL} -cf - ." > "${OUT_DIR}/covdata.tar"
tar -xf "${OUT_DIR}/covdata.tar" -C "${OUT_DIR}"
rm -f "${OUT_DIR}/covdata.tar"

echo "Done. Coverage files:"
ls -la "${OUT_DIR}"

echo
echo "Convert with:"
echo "  go tool covdata percent -i=${OUT_DIR}"
echo "  go tool covdata textfmt -i=${OUT_DIR} -o=${OUT_DIR}/coverage.out"
echo "  go tool cover -func=${OUT_DIR}/coverage.out"
