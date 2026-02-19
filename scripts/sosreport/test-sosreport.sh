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
REPORT_SCRIPT="$SCRIPT_DIR/generate-report.py"
REPORT_TEMPLATE="$SCRIPT_DIR/report-template.html"
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
    "generate_html_report"
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

# Test 11: HTML report generator exists and is executable
echo ""
echo "[Test 11] Checking HTML report generator..."
if [ -x "$REPORT_SCRIPT" ]; then
    echo "PASS: generate-report.py exists and is executable"
else
    echo "FAIL: generate-report.py is not executable or doesn't exist"
    exit 1
fi

# Test 11b: HTML report template exists
echo ""
echo "[Test 11b] Checking HTML report template..."
if [ -f "$REPORT_TEMPLATE" ]; then
    echo "  Template file exists"
    # Check for required placeholders
    missing_placeholders=false
    for placeholder in 'SECTION_DASHBOARD' 'SECTION_NCP_STATUS' 'SECTION_COMPONENTS' 'SECTION_DIAGNOSTICS' 'SECTION_NODES' 'SECTION_EVENTS' 'SECTION_METADATA' 'SECTION_CRDS' 'SECTION_RBAC' 'SECTION_NETWORK' 'SECTION_CONFIG' 'SECTION_ERRORS'; do
        if grep -q "\${${placeholder}}" "$REPORT_TEMPLATE"; then
            echo "  Found placeholder: \${${placeholder}}"
        else
            echo "  Missing placeholder: \${${placeholder}}"
            missing_placeholders=true
        fi
    done
    if [ "$missing_placeholders" = false ]; then
        echo "PASS: Report template exists with all required placeholders"
    else
        echo "FAIL: Report template is missing placeholders"
        exit 1
    fi
else
    echo "FAIL: report-template.html doesn't exist"
    exit 1
fi

# Test 12: HTML report generator Python syntax
echo ""
echo "[Test 12] Checking HTML report generator syntax..."
if python3 -m py_compile "$REPORT_SCRIPT"; then
    echo "PASS: generate-report.py syntax is valid"
else
    echo "FAIL: generate-report.py syntax errors detected"
    exit 1
fi

# Test 13: HTML report generator contains required functions
echo ""
echo "[Test 13] Checking report generator functions..."
report_functions=(
    "render_dashboard"
    "render_ncp_status"
    "render_components"
    "render_diagnostics"
    "render_nodes"
    "render_events"
    "render_crds"
    "render_rbac"
    "render_network"
    "render_config"
    "render_errors"
    "render_metadata"
    "render_related_operators"
)

all_report_funcs=true
for func in "${report_functions[@]}"; do
    if grep -q "^def ${func}" "$REPORT_SCRIPT"; then
        echo "  Found: $func"
    else
        echo "  Missing: $func"
        all_report_funcs=false
    fi
done

if [ "$all_report_funcs" = true ]; then
    echo "PASS: All report generator functions present"
else
    echo "FAIL: Some report generator functions are missing"
    exit 1
fi

# Test 14: HTML report generator works with synthetic data
echo ""
echo "[Test 14] Testing report generator with synthetic sosreport data..."
FIXTURE_DIR="${SCRIPT_DIR}/.test-fixture-$$"
mkdir -p "$FIXTURE_DIR"
trap 'rm -rf "$FIXTURE_DIR"' EXIT

# Create minimal sosreport directory structure
mkdir -p "$FIXTURE_DIR/metadata"
mkdir -p "$FIXTURE_DIR/crds/definitions"
mkdir -p "$FIXTURE_DIR/crds/instances/nicclusterpolicies"
mkdir -p "$FIXTURE_DIR/operator/components/network-operator/pods"
mkdir -p "$FIXTURE_DIR/operator/rbac"
mkdir -p "$FIXTURE_DIR/nodes"
mkdir -p "$FIXTURE_DIR/network"

cat > "$FIXTURE_DIR/metadata/collection-info.txt" <<FIXTURE_EOF
Collection Time: 2026-02-18 14:30:00 UTC
Script Version: v26.1.0
Operator Namespace: nvidia-network-operator
Platform: Kubernetes
FIXTURE_EOF

cat > "$FIXTURE_DIR/crds/instances/nicclusterpolicies/all.yaml" <<FIXTURE_EOF
apiVersion: mellanox.com/v1alpha1
kind: NicClusterPolicy
metadata:
  name: nic-cluster-policy
spec:
  ofedDriver:
    image: mofed
status:
  state: ready
  appliedStates:
  - name: state-OFED
    state: ready
  - name: state-Multus
    state: ready
FIXTURE_EOF

cat > "$FIXTURE_DIR/operator/components/network-operator/deployment.yaml" <<FIXTURE_EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: network-operator
spec:
  replicas: 1
status:
  replicas: 1
  readyReplicas: 1
FIXTURE_EOF

cat > "$FIXTURE_DIR/operator/components/network-operator/pods/network-operator-abc123.yaml" <<FIXTURE_EOF
apiVersion: v1
kind: Pod
metadata:
  name: network-operator-abc123
spec:
  nodeName: node-1
status:
  phase: Running
  containerStatuses:
  - name: network-operator
    restartCount: 0
    ready: true
FIXTURE_EOF

echo "test log line" > "$FIXTURE_DIR/operator/components/network-operator/pods/network-operator-abc123.log"

cat > "$FIXTURE_DIR/nodes/nodes-summary.txt" <<FIXTURE_EOF
NAME     STATUS   ROLES    AGE   VERSION
node-1   Ready    master   10d   v1.28.0
FIXTURE_EOF

cat > "$FIXTURE_DIR/diagnostic-summary.txt" <<FIXTURE_EOF
NVIDIA Network Operator Diagnostic Summary
====================================

Nodes: 1

Collection Statistics:
---------------------
CRD Definitions: 2
CRD Instances: 1
Component Pods: 1
Components Found: 1
Components Skipped: 0
Warnings: 0
Errors: 0
FIXTURE_EOF

touch "$FIXTURE_DIR/collection-errors.log"

# Run the report generator
REPORT_OUTPUT="$FIXTURE_DIR/report.html"
if python3 "$REPORT_SCRIPT" "$FIXTURE_DIR" --output "$REPORT_OUTPUT" --template "$REPORT_TEMPLATE" > /dev/null 2>&1; then
    echo "  Report generated successfully"
else
    echo "FAIL: Report generation failed"
    exit 1
fi

# Verify the output file exists and contains expected content
if [ -f "$REPORT_OUTPUT" ] && [ -s "$REPORT_OUTPUT" ]; then
    echo "  Report file exists and is non-empty"
else
    echo "FAIL: Report file missing or empty"
    exit 1
fi

# Check for key HTML elements
missing_elements=false
for element in "<!DOCTYPE html>" "NicClusterPolicy" "Component Health" "OFED Diagnostics" "Node Overview" "Events" "RBAC" "sidebar"; do
    if grep -q "$element" "$REPORT_OUTPUT"; then
        echo "  Found element: $element"
    else
        echo "  Missing element: $element"
        missing_elements=true
    fi
done

if [ "$missing_elements" = false ]; then
    echo "PASS: HTML report generated correctly with all sections"
else
    echo "FAIL: HTML report is missing expected content"
    exit 1
fi

# Test 15: Collection script references report generator
echo ""
echo "[Test 15] Checking collection script references report generator..."
if grep -q "generate_html_report" "$SOSREPORT_SCRIPT" && \
   grep -q "generate-report.py" "$SOSREPORT_SCRIPT" && \
   grep -q "report-template.html" "$SOSREPORT_SCRIPT" && \
   grep -q "skip-report" "$SOSREPORT_SCRIPT"; then
    echo "PASS: Collection script properly references report generator"
else
    echo "FAIL: Collection script missing report generator integration"
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
