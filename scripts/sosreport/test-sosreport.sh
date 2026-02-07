#!/bin/bash

#  2026 NVIDIA CORPORATION & AFFILIATES
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

# Test script for kubectl-netop_sosreport
# This script validates the SOS-report script without requiring a live cluster

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOSREPORT_SCRIPT="$SCRIPT_DIR/kubectl-netop_sosreport"
LEGACY_SYMLINK="$SCRIPT_DIR/network-operator-sosreport.sh"

echo "========================================"
echo "NVIDIA Network Operator SOS-Report Test Suite"
echo "========================================"
echo ""

# Test 1: Script exists and is executable
echo "[Test 1] Checking if script exists and is executable..."
if [ -x "$SOSREPORT_SCRIPT" ]; then
    echo "PASS: Script exists and is executable"
else
    echo "FAIL: Script is not executable or doesn't exist"
    exit 1
fi

# Test 1b: Backward compatibility symlink
echo ""
echo "[Test 1b] Checking backward compatibility symlink..."
if [ -L "$LEGACY_SYMLINK" ] && [ -x "$LEGACY_SYMLINK" ]; then
    echo "PASS: Legacy symlink exists and works"
else
    echo "FAIL: Legacy symlink missing or not executable"
    exit 1
fi

# Test 2: Bash syntax check
echo ""
echo "[Test 2] Checking bash syntax..."
if bash -n "$SOSREPORT_SCRIPT"; then
    echo "PASS: Bash syntax is valid"
else
    echo "FAIL: Bash syntax errors detected"
    exit 1
fi

# Test 4: Help flag works
echo ""
echo "[Test 4] Testing --help flag..."
if "$SOSREPORT_SCRIPT" --help &> /dev/null; then
    echo "PASS: Help flag works"
else
    echo "FAIL: Help flag failed"
    exit 1
fi

# Test 5: Script contains all required functions
echo ""
echo "[Test 5] Checking for required functions..."
required_functions=(
    "check_prerequisites"
    "detect_operator_namespace"
    "collect_crd_definitions"
    "collect_crd_instances"
    "collect_operator_resources"
    "collect_component"
    "collect_all_components"
    "collect_diagnostic_commands"
    "collect_node_info"
    "collect_network_info"
    "collect_related_operators"
    "cleanup_empty_artifacts"
    "generate_summary"
    "create_archive"
)

all_found=true
for func in "${required_functions[@]}"; do
    if grep -q "^${func}()" "$SOSREPORT_SCRIPT"; then
        echo "  Found: $func"
    else
        echo "  Missing: $func"
        all_found=false
    fi
done

if [ "$all_found" = true ]; then
    echo "PASS: All required functions present"
else
    echo "FAIL: Some functions are missing"
    exit 1
fi

# Test 6: collect_resource function exists (renamed from safe_kubectl)
echo ""
echo "[Test 6] Checking for collect_resource function..."
if grep -q "^collect_resource()" "$SOSREPORT_SCRIPT"; then
    echo "PASS: collect_resource function found"
else
    echo "FAIL: collect_resource function not found"
    exit 1
fi

# Test 6b: Ensure old safe_kubectl function is NOT present
echo ""
echo "[Test 6b] Verifying old safe_kubectl function is removed..."
if ! grep -q "safe_kubectl()" "$SOSREPORT_SCRIPT"; then
    echo "PASS: Old safe_kubectl function properly removed"
else
    echo "FAIL: Old safe_kubectl function still present"
    exit 1
fi

# Test 5: Documentation files exist
echo ""
echo "[Test 5] Checking documentation files..."
doc_files=(
    "$SCRIPT_DIR/README.md"
)

all_docs_found=true
for doc in "${doc_files[@]}"; do
    if [ -f "$doc" ]; then
        echo "  Found: $(basename "$doc")"
    else
        echo "  Missing: $(basename "$doc")"
        all_docs_found=false
    fi
done

if [ "$all_docs_found" = true ]; then
    echo "PASS: All documentation files present"
else
    echo "FAIL: Some documentation files are missing"
    exit 1
fi

# Test 6: Script contains all component labels
echo ""
echo "[Test 6] Checking component label definitions..."
# Labels are extracted from manifests at build time - test that key labels are present
required_labels=(
    # Main Network Operator components (from manifests/state-*)
    "nvidia.com/ofed-driver"
    "name=nic-feature-discovery"
    "name=network-operator-sriov-device-plugin"
    "app=rdma-shared-dp"
    "name=multus"
    "name=cni-plugins"
    "name=ipoib-cni"
    "name=ib-kubernetes"
    "name=nv-ipam-controller"
    "name=nv-ipam-node"
    "app.kubernetes.io/component=nic-configuration-operator"
    "app.kubernetes.io/name=nic-configuration-daemon"
    "app.kubernetes.io/name=doca-telemetry"
    "control-plane=spectrum-x-flowcontroller"
    # Node Feature Discovery (sub-chart)
    "app.kubernetes.io/name=node-feature-discovery"
    # SR-IOV Network Operator (sub-chart)
    "app=sriov-network-operator"
    "app=sriov-network-config-daemon"
    # Maintenance Operator (sub-chart)
    "app.kubernetes.io/name=maintenance-operator"
)

all_labels_found=true
for label in "${required_labels[@]}"; do
    if grep -q "$label" "$SOSREPORT_SCRIPT"; then
        echo "  Found label: $label"
    else
        echo "  Missing label: $label"
        all_labels_found=false
    fi
done

if [ "$all_labels_found" = true ]; then
    echo "PASS: All component labels present"
else
    echo "FAIL: Some component labels are missing"
    exit 1
fi

# Test 7: Script has proper error handling
echo ""
echo "[Test 7] Checking error handling mechanisms..."
if grep -q "set -o pipefail" "$SOSREPORT_SCRIPT" && \
   grep -q "ERROR_LOG" "$SOSREPORT_SCRIPT" && \
   grep -q "log_error" "$SOSREPORT_SCRIPT" && \
   grep -q "log_warn" "$SOSREPORT_SCRIPT"; then
    echo "PASS: Error handling mechanisms present"
else
    echo "FAIL: Missing error handling mechanisms"
    exit 1
fi

# Test 8: Script version is defined
echo ""
echo "[Test 8] Checking script version..."
if grep -q "SCRIPT_VERSION=" "$SOSREPORT_SCRIPT"; then
    version=$(grep "SCRIPT_VERSION=" "$SOSREPORT_SCRIPT" | head -1 | cut -d'"' -f2)
    echo "  Script version: $version"
    echo "PASS: Script version defined"
else
    echo "FAIL: Script version not defined"
    exit 1
fi

# Test 9: Check CRD collection coverage
echo ""
echo "[Test 9] Checking CRD collection coverage..."
required_crds=(
    # Main Network Operator CRDs
    "nicclusterpolicies.mellanox.com"
    "macvlannetworks.mellanox.com"
    "hostdevicenetworks.mellanox.com"
    "ipoibnetworks.mellanox.com"
    # NV-IPAM CRDs
    "ippools.nv-ipam.nvidia.com"
    "cidrpools.nv-ipam.nvidia.com"
    # NIC Configuration Operator CRDs
    "nicdevices.configuration.net.nvidia.com"
    # Multus CRDs
    "network-attachment-definitions.k8s.cni.cncf.io"
    # SR-IOV Operator CRDs
    "sriovnetworknodepolicies.sriovnetwork.openshift.io"
    "sriovnetworknodestates.sriovnetwork.openshift.io"
    # Maintenance Operator CRDs
    "nodemaintenances.maintenance.nvidia.com"
    # Node Feature Discovery CRDs
    "nodefeatures.nfd.k8s-sigs.io"
)

all_crds_found=true
for crd in "${required_crds[@]}"; do
    if grep -q "$crd" "$SOSREPORT_SCRIPT"; then
        echo "  Found CRD: $crd"
    else
        echo "  Missing CRD: $crd"
        all_crds_found=false
    fi
done

if [ "$all_crds_found" = true ]; then
    echo "PASS: All major CRDs covered"
else
    echo "FAIL: Some CRDs are missing"
    exit 1
fi

# Test 10: Check diagnostic commands
echo ""
echo "[Test 10] Checking diagnostic commands..."
diagnostic_commands=(
    "lsmod"
    "ibstat"
    "ibv_devinfo"
    "mst status"
    "dmesg"
    "ip link"
    "ip addr"
)

all_commands_found=true
for cmd in "${diagnostic_commands[@]}"; do
    if grep -q "$cmd" "$SOSREPORT_SCRIPT"; then
        echo "  Found command: $cmd"
    else
        echo "  Missing command: $cmd"
        all_commands_found=false
    fi
done

if [ "$all_commands_found" = true ]; then
    echo "PASS: All diagnostic commands present"
else
    echo "FAIL: Some diagnostic commands are missing"
    exit 1
fi

# Summary
echo ""
echo "========================================"
echo "Test Summary: ALL TESTS PASSED ✓"
echo "========================================"
echo ""
echo "The SOS-report script is ready for use!"
echo ""
echo "Next steps:"
echo "1. Test on a live cluster: ./network-operator-sosreport.sh --help"
echo "2. Run a test collection: ./network-operator-sosreport.sh --skip-diagnostics"
echo "3. Review the output structure"
echo ""
echo "For live cluster testing, ensure:"
echo "  • kubectl is installed and in PATH"
echo "  • kubeconfig is configured"
echo "  • You have cluster-admin permissions"
echo "  • Network Operator is deployed"
echo ""

exit 0
