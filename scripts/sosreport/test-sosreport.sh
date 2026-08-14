#!/bin/bash

# Behavioral tests source the collector dynamically and invoke mock functions
# through KUBECTL_BIN, which ShellCheck cannot trace statically.
# shellcheck disable=SC1090,SC2034,SC2154,SC2329

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
    "collect_container_logs"
    "collect_pods_and_logs"
    "detect_helm_release_selector"
    "resolve_pod_workload"
    "collect_helm_release_components"
    "resolve_component_selector"
    "collect_component"
    "collect_all_components"
    "run_diagnostic_command"
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
    "app=sriovdp"
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
    "role=topology-updater"
    # SR-IOV Network Operator (sub-chart)
    "app.kubernetes.io/name=sriov-network-operator"
    "name=sriov-network-operator"
    "app=sriov-network-config-daemon"
    "app=network-resources-injector"
    "app=operator-webhook"
    "app=sriov-device-plugin"
    "app=sriov-network-metrics-exporter"
    "app=sriov-dra-driver"
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

# Test 6c: NameSuffix selectors cover NicClusterPolicy and NicNodePolicy workloads
echo ""
echo "[Test 6c] Testing suffix-aware component selectors..."
(
    source "$SOSREPORT_SCRIPT"

    fake_policy_kubectl() {
        case "$*" in
            "get daemonsets -n test-operator -l app -o jsonpath="*|\
            "get pods -n test-operator -l app -o jsonpath="*)
                printf 'rdma-shared-dp\nrdma-shared-dp-blue-policy\nrdma-shared-dp-red-policy\nunrelated\n'
                ;;
            *nicnodepolicies*)
                echo "FAIL: Selector discovery must not require NicNodePolicy access"
                exit 1
                ;;
        esac
    }

    KUBECTL_BIN=fake_policy_kubectl
    OPERATOR_NAMESPACE=test-operator
    resolve_component_selector "app=rdma-shared-dp__NAME_SUFFIX__" "$OPERATOR_NAMESPACE" "daemonset"
    resolved_selector="$RESOLVED_COMPONENT_SELECTOR"
    expected_selector="app in (rdma-shared-dp,rdma-shared-dp-blue-policy,rdma-shared-dp-red-policy)"

    if [ "$resolved_selector" != "$expected_selector" ]; then
        echo "FAIL: Unexpected suffix-aware selector: $resolved_selector"
        exit 1
    fi

    if ! grep -Fq '["rdma-shared-dp-ds"]="app=rdma-shared-dp__NAME_SUFFIX__|daemonset"' "$SOSREPORT_SCRIPT"; then
        echo "FAIL: Generated component map does not preserve NameSuffix markers"
        exit 1
    fi
    if ! grep -Fq '["network-operator-sriov-device-plugin"]="app=sriovdp|daemonset"' "$SOSREPORT_SCRIPT"; then
        echo "FAIL: SR-IOV device plugin selector does not cover NCP and NNP workloads"
        exit 1
    fi
)
echo "PASS: Component selectors include NicNodePolicy workloads"

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
    "nicnodepolicies.mellanox.com"
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

# Test 9b: Every collected CRD definition also has instance collection
echo ""
echo "[Test 9b] Checking CRD definition and instance collection parity..."
definition_crds=$(mktemp)
instance_crds=$(mktemp)
sed -n '/local all_crds=(/,/# \[GENERATED-CRDS-END\]/p' "$SOSREPORT_SCRIPT" \
    | sed -n 's/.*"\([^"]*\)".*/\1/p' \
    | sort -u > "$definition_crds"
sed -n '/# Collect all CR types/,/STATS\[crds_instances\]/p' "$SOSREPORT_SCRIPT" \
    | sed -n 's/.*collect_cr_instances "\([^"]*\)".*/\1/p' \
    | sort -u > "$instance_crds"

missing_instance_collectors=$(comm -23 "$definition_crds" "$instance_crds")
unexpected_instance_collectors=$(comm -13 "$definition_crds" "$instance_crds")
rm -f "$definition_crds" "$instance_crds"

if [ -n "$missing_instance_collectors" ]; then
    echo "FAIL: CRDs missing instance collection:"
    echo "$missing_instance_collectors"
    exit 1
fi
if [ -n "$unexpected_instance_collectors" ]; then
    echo "FAIL: Instance collectors missing from the CRD definition inventory:"
    echo "$unexpected_instance_collectors"
    exit 1
fi
echo "PASS: Every CRD definition has matching instance collection"

# Test 9c: NicNodePolicy instances are archived as first-class artifacts
echo ""
echo "[Test 9c] Testing NicNodePolicy instance collection..."
(
    source "$SOSREPORT_SCRIPT"

    TEST_DIR=$(mktemp -d)
    trap 'rm -rf "$TEST_DIR"' EXIT

    fake_nnp_kubectl() {
        case "$*" in
            "get nicnodepolicies.mellanox.com -A --no-headers")
                echo "blue-policy"
                ;;
            "get nicnodepolicies.mellanox.com -A -o yaml")
                cat <<NNP_EOF
apiVersion: v1
kind: List
items:
  - apiVersion: mellanox.com/v1alpha1
    kind: NicNodePolicy
    metadata:
      name: blue-policy
NNP_EOF
                ;;
            "get nicnodepolicies.mellanox.com -A")
                return 0
                ;;
            *)
                return 1
                ;;
        esac
    }

    KUBECTL_BIN=fake_nnp_kubectl
    OUTPUT_DIR="$TEST_DIR/report"
    ERROR_LOG="$OUTPUT_DIR/collection-errors.log"
    mkdir -p "$OUTPUT_DIR"
    : > "$ERROR_LOG"
    STATS[crds_instances]=0

    if ! collect_crd_instances; then
        echo "FAIL: CR instance collection returned an error"
        exit 1
    fi

    NNP_FILE="$OUTPUT_DIR/crds/instances/nicnodepolicies/all.yaml"
    if [ ! -s "$NNP_FILE" ] || ! grep -q "name: blue-policy" "$NNP_FILE"; then
        echo "FAIL: NicNodePolicy instance YAML was not collected"
        exit 1
    fi
    if [ "${STATS[crds_instances]}" -ne 1 ]; then
        echo "FAIL: NicNodePolicy collection was not included in CR instance statistics"
        exit 1
    fi
)
echo "PASS: NicNodePolicy instance YAML is collected"

# Test 10: Check diagnostic commands
echo ""
echo "[Test 10] Checking diagnostic commands..."
diagnostic_commands=(
    "lsmod"
    "ibstat"
    "ibv_devinfo"
    "mst status"
    "rdma dev show"
    "rdma link show"
    "devlink dev show"
    "devlink dev info"
    "devlink port show"
    "/sys/class/infiniband"
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

# Test 10b: Missing optional diagnostics are skipped without collection errors
echo ""
echo "[Test 10b] Testing capability-aware OFED diagnostics..."
(
    source "$SOSREPORT_SCRIPT"

    TEST_DIR=$(mktemp -d)
    trap 'rm -rf "$TEST_DIR"' EXIT
    DIAGNOSTIC_CALLS="$TEST_DIR/kubectl-calls.txt"

    fake_diagnostics_kubectl() {
        printf '%s\n' "$*" >> "$DIAGNOSTIC_CALLS"
        case "$*" in
            "get pods -n test-operator -l nvidia.com/ofed-driver= -o name")
                echo "pod/mofed-node-1"
                ;;
            "get -n test-operator pod/mofed-node-1 -o jsonpath={.spec.nodeName}")
                echo "node-1"
                ;;
            exec*)
                if [[ "$*" == *"command -v ibstat"* ]] ||
                   [[ "$*" == *"command -v ibv_devinfo"* ]] ||
                   [[ "$*" == *"command -v mst"* ]]; then
                    echo "unavailable"
                elif [[ "$*" == *"command -v"* ]]; then
                    echo "available"
                else
                    echo "synthetic diagnostic output"
                fi
                ;;
        esac
    }

    KUBECTL_BIN=fake_diagnostics_kubectl
    OPERATOR_NAMESPACE=test-operator
    OUTPUT_DIR="$TEST_DIR/report"
    ERROR_LOG="$OUTPUT_DIR/collection-errors.log"
    NODE_SELECTOR=""
    SKIP_DIAGNOSTICS=false
    mkdir -p "$OUTPUT_DIR"
    : > "$ERROR_LOG"
    STATS[diagnostic_commands]=0
    STATS[diagnostic_skipped]=0
    STATS[diagnostic_failed]=0

    collect_diagnostic_commands

    STATUS_FILE="$OUTPUT_DIR/operator/components/ofed-driver/diagnostics/node-1-diagnostic_status.txt"
    for command_name in ibstat ibv_devinfo mst_status; do
        if ! grep -Eq "^${command_name}[[:space:]]+SKIPPED" "$STATUS_FILE"; then
            echo "FAIL: $command_name was not recorded as skipped"
            exit 1
        fi
    done
    if [ "${STATS[diagnostic_skipped]}" -ne 3 ] || [ "${STATS[diagnostic_failed]}" -ne 0 ]; then
        echo "FAIL: Unexpected diagnostic skip/failure counts"
        exit 1
    fi
    if grep -q "command not found" "$ERROR_LOG"; then
        echo "FAIL: Missing optional tools polluted collection-errors.log"
        exit 1
    fi
    for output_name in rdma_dev rdma_link devlink_dev devlink_info devlink_port infiniband_sysfs; do
        if [ ! -s "$OUTPUT_DIR/operator/components/ofed-driver/diagnostics/node-1-${output_name}.txt" ]; then
            echo "FAIL: Portable diagnostic output is missing: $output_name"
            exit 1
        fi
    done
    if grep '^exec ' "$DIAGNOSTIC_CALLS" | grep -vq -- '-c mofed-container -- sh -c'; then
        echo "FAIL: A diagnostic command did not target mofed-container explicitly"
        exit 1
    fi
)
echo "PASS: Optional diagnostics are skipped and portable diagnostics are collected"

# Test 10c: SR-IOV workloads and standalone operator logs are collected
echo ""
echo "[Test 10c] Testing SR-IOV Operator log collection..."
(
    source "$SOSREPORT_SCRIPT"

    TEST_DIR=$(mktemp -d)
    trap 'rm -rf "$TEST_DIR"' EXIT

    emit_resource_yaml() {
        cat <<RESOURCE_EOF
apiVersion: v1
kind: List
metadata:
  name: synthetic
spec:
  replicas: 1
status:
  readyReplicas: 1
RESOURCE_EOF
    }

    fake_sriov_kubectl() {
        case "$*" in
            "get deployments -n test-operator -l app.kubernetes.io/name=sriov-network-operator --no-headers")
                echo "sriov-network-operator 1/1 1 1 1m"
                ;;
            "get deployments -n test-operator -l app.kubernetes.io/name=sriov-network-operator -o yaml"|\
            "get -n test-operator pod/sriov-controller -o yaml"|\
            "get namespace standalone-sriov -o yaml"|\
            "get deployments -n standalone-sriov -o yaml"|\
            "get daemonsets -n standalone-sriov -o yaml"|\
            "get configmaps -n standalone-sriov -o yaml"|\
            "get events -n standalone-sriov --sort-by=.lastTimestamp -o yaml"|\
            "get -n standalone-sriov pod/standalone-controller -o yaml")
                emit_resource_yaml
                ;;
            "get pods -n test-operator -l name=sriov-network-operator -o name")
                echo "pod/sriov-controller"
                ;;
            "get -n test-operator pod/sriov-controller -o jsonpath={.spec.containers[*].name}")
                echo "controller sidecar"
                ;;
            "logs -n test-operator pod/sriov-controller -c "*" --previous --tail="*)
                echo "bundled previous log"
                ;;
            "logs -n test-operator pod/sriov-controller -c "*" --tail="*)
                echo "bundled current log"
                ;;
            "get sriovoperatorconfigs.sriovnetwork.openshift.io -A -o jsonpath="*)
                echo "standalone-sriov"
                ;;
            "get deployments -A -l app.kubernetes.io/name=sriov-network-operator -o jsonpath="*|\
            "get pods -A -l name=sriov-network-operator -o jsonpath="*)
                ;;
            "get namespace openshift-sriov-network-operator")
                return 1
                ;;
            "get pods -n standalone-sriov -o name")
                echo "pod/standalone-controller"
                ;;
            "get -n standalone-sriov pod/standalone-controller -o jsonpath={.spec.containers[*].name}")
                echo "controller"
                ;;
            "logs -n standalone-sriov pod/standalone-controller -c controller --previous --tail="*)
                echo "standalone previous log"
                ;;
            "logs -n standalone-sriov pod/standalone-controller -c controller --tail="*)
                echo "standalone current log"
                ;;
        esac
    }

    KUBECTL_BIN=fake_sriov_kubectl
    OPERATOR_NAMESPACE=test-operator
    OUTPUT_DIR="$TEST_DIR/report"
    ERROR_LOG="$OUTPUT_DIR/collection-errors.log"
    NODE_SELECTOR=""
    LOG_LINES=100
    mkdir -p "$OUTPUT_DIR"
    : > "$ERROR_LOG"
    STATS[components_found]=0
    STATS[components_skipped]=0
    STATS[component_pods]=0

    if ! collect_component "sriov-network-operator" \
        "app.kubernetes.io/name=sriov-network-operator" \
        "deployment" "name=sriov-network-operator"; then
        echo "FAIL: Bundled SR-IOV component collection failed"
        exit 1
    fi
    collect_related_operators

    BUNDLED_PODS="$OUTPUT_DIR/operator/components/sriov-network-operator/pods"
    STANDALONE_PODS="$OUTPUT_DIR/related-operators/sriov-network-operator/standalone-sriov/pods"
    for expected_file in \
        "$BUNDLED_PODS/sriov-controller-controller.log" \
        "$BUNDLED_PODS/sriov-controller-sidecar.log" \
        "$BUNDLED_PODS/sriov-controller-controller-previous.log" \
        "$STANDALONE_PODS/standalone-controller-controller.log" \
        "$STANDALONE_PODS/standalone-controller-controller-previous.log"; do
        if [ ! -s "$expected_file" ]; then
            echo "FAIL: Expected SR-IOV log is missing: $expected_file"
            exit 1
        fi
    done
)
echo "PASS: Bundled and standalone SR-IOV Operator logs are collected"

# Test 10d: Helm-only workloads and every pod container type are collected
echo ""
echo "[Test 10d] Testing complete Helm component and init container collection..."
(
    source "$SOSREPORT_SCRIPT"

    TEST_DIR=$(mktemp -d)
    trap 'rm -rf "$TEST_DIR"' EXIT

    emit_resource_yaml() {
        cat <<RESOURCE_EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: chart-addon
spec:
  replicas: 1
status:
  readyReplicas: 1
RESOURCE_EOF
    }

    fake_helm_kubectl() {
        case "$*" in
            "get deployments -n test-operator -l app.kubernetes.io/name=network-operator -o jsonpath="*)
                echo "netop-release"
                ;;
            "get deployments -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml"|\
            "get deployment -n test-operator chart-addon -o yaml"|\
            "get -n test-operator pod/chart-addon-abc -o yaml")
                emit_resource_yaml
                ;;
            "get daemonsets -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml"|\
            "get statefulsets -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml"|\
            "get replicasets -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml"|\
            "get jobs -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml"|\
            "get cronjobs -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml"|\
            "get poddisruptionbudgets -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml"|\
            "get networkpolicies -n test-operator -l app.kubernetes.io/instance=netop-release -o yaml")
                cat <<EMPTY_EOF
apiVersion: v1
items: []
kind: List
metadata: {}
EMPTY_EOF
                ;;
            "get pods -n test-operator -l app.kubernetes.io/instance=netop-release -o name")
                echo "pod/chart-addon-abc"
                ;;
            "get -n test-operator pod/chart-addon-abc -o jsonpath={.metadata.ownerReferences[0].kind}")
                echo "ReplicaSet"
                ;;
            "get -n test-operator pod/chart-addon-abc -o jsonpath={.metadata.ownerReferences[0].name}")
                echo "chart-addon-rs"
                ;;
            "get replicaset -n test-operator chart-addon-rs -o jsonpath={.metadata.ownerReferences[0].kind}")
                echo "Deployment"
                ;;
            "get replicaset -n test-operator chart-addon-rs -o jsonpath={.metadata.ownerReferences[0].name}")
                echo "chart-addon"
                ;;
            "get -n test-operator pod/chart-addon-abc -o jsonpath={.spec.containers[*].name}")
                echo "controller sidecar"
                ;;
            "get -n test-operator pod/chart-addon-abc -o jsonpath={.spec.initContainers[*].name}")
                echo "setup"
                ;;
            "get -n test-operator pod/chart-addon-abc -o jsonpath={.spec.ephemeralContainers[*].name}")
                echo "debugger"
                ;;
            "logs -n test-operator pod/chart-addon-abc -c "*" --previous --tail="*)
                echo "synthetic previous log"
                ;;
            "logs -n test-operator pod/chart-addon-abc -c "*" --tail="*)
                echo "synthetic current log"
                ;;
        esac
    }

    KUBECTL_BIN=fake_helm_kubectl
    OPERATOR_NAMESPACE=test-operator
    OUTPUT_DIR="$TEST_DIR/report"
    ERROR_LOG="$OUTPUT_DIR/collection-errors.log"
    NODE_SELECTOR=""
    LOG_LINES=100
    HELM_RELEASE_SELECTOR=""
    HELM_RELEASE_DISCOVERED=false
    declare -A COLLECTED_PODS=()
    declare -A DISCOVERED_COMPONENTS=()
    mkdir -p "$OUTPUT_DIR"
    : > "$ERROR_LOG"
    STATS[components_found]=0
    STATS[component_pods]=0

    collect_helm_release_components
    collect_helm_release_components

    COMPONENT_DIR="$OUTPUT_DIR/operator/components/chart-addon"
    for expected_file in \
        "$OUTPUT_DIR/operator/helm-release/workloads/deployments.yaml" \
        "$COMPONENT_DIR/deployment.yaml" \
        "$COMPONENT_DIR/pods/chart-addon-abc.yaml" \
        "$COMPONENT_DIR/pods/chart-addon-abc-controller.log" \
        "$COMPONENT_DIR/pods/chart-addon-abc-controller-previous.log" \
        "$COMPONENT_DIR/pods/chart-addon-abc-sidecar.log" \
        "$COMPONENT_DIR/pods/chart-addon-abc-init-setup.log" \
        "$COMPONENT_DIR/pods/chart-addon-abc-init-setup-previous.log" \
        "$COMPONENT_DIR/pods/chart-addon-abc-ephemeral-debugger.log"; do
        if [ ! -s "$expected_file" ]; then
            echo "FAIL: Expected Helm component artifact is missing: $expected_file"
            exit 1
        fi
    done

    if [ "${STATS[components_found]}" -ne 1 ] || [ "${STATS[component_pods]}" -ne 1 ]; then
        echo "FAIL: Helm component or pod was counted more than once"
        exit 1
    fi
)
echo "PASS: Helm-only workloads and all container types are collected once"

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
    for placeholder in 'SECTION_DASHBOARD' 'SECTION_NCP_STATUS' 'SECTION_COMPONENTS' 'SECTION_DIAGNOSTICS' 'SECTION_NODES' 'SECTION_EVENTS' 'SECTION_METADATA' 'SECTION_CRDS' 'SECTION_RBAC' 'SECTION_NETWORK' 'SECTION_CONFIG' 'SECTION_RELATED' 'SECTION_ERRORS'; do
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
trap 'rm -rf "$FIXTURE_DIR" "$SCRIPT_DIR/__pycache__"' EXIT

# Create minimal sosreport directory structure
mkdir -p "$FIXTURE_DIR/metadata"
mkdir -p "$FIXTURE_DIR/crds/definitions"
mkdir -p "$FIXTURE_DIR/crds/instances/nicclusterpolicies"
mkdir -p "$FIXTURE_DIR/operator/components/network-operator/pods"
mkdir -p "$FIXTURE_DIR/operator/rbac"
mkdir -p "$FIXTURE_DIR/operator/helm-release/workloads"
mkdir -p "$FIXTURE_DIR/operator/components/ofed-driver/diagnostics"
mkdir -p "$FIXTURE_DIR/related-operators/sriov-network-operator/standalone-sriov/pods"
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
echo "sidecar container log" > "$FIXTURE_DIR/operator/components/network-operator/pods/network-operator-abc123-sidecar.log"
echo "init setup log" > "$FIXTURE_DIR/operator/components/network-operator/pods/network-operator-abc123-init-setup.log"

cat > "$FIXTURE_DIR/operator/helm-release/workloads/jobs.yaml" <<FIXTURE_EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: network-operator-upgrade-hook
FIXTURE_EOF

cat > "$FIXTURE_DIR/operator/components/ofed-driver/diagnostics/node-1-diagnostic_status.txt" <<FIXTURE_EOF
OFED diagnostic command status
ibstat                       SKIPPED executable ibstat is not installed in mofed-container
rdma_dev                     SUCCESS
FIXTURE_EOF
echo "link mlx5_0/1 state ACTIVE" > "$FIXTURE_DIR/operator/components/ofed-driver/diagnostics/node-1-rdma_dev.txt"

echo "standalone sriov namespace log" > "$FIXTURE_DIR/related-operators/sriov-network-operator/standalone-sriov/pods/sriov-controller-controller.log"

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
for element in "<!DOCTYPE html>" "NicClusterPolicy" "Component Health" "OFED Diagnostics" "Node Overview" "Events" "RBAC" "sidebar" "sidecar container log" "Init Container Logs (setup)" "Helm Release Artifacts" "network-operator-upgrade-hook" "Diagnostic command status" "rdma dev show" "standalone sriov namespace log"; do
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
