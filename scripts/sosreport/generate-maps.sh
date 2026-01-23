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

# Generate CRD and Component maps for the SOS report script
# This script extracts definitions from manifests and embeds them into kubectl-netop_sosreport

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPT_FILE="$SCRIPT_DIR/kubectl-netop_sosreport"
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Extract CRD names from manifests
extract_crds() {
    local crds=()

    # Main CRDs from config/crd/bases/
    if [ -d "$REPO_ROOT/config/crd/bases" ]; then
        for crd_file in "$REPO_ROOT"/config/crd/bases/*.yaml; do
            [ -f "$crd_file" ] || continue
            local name
            name=$(grep -m1 "^  name:" "$crd_file" 2>/dev/null | awk '{print $2}')
            [ -n "$name" ] && crds+=("$name")
        done
    fi

    # Sub-component CRDs from manifests/state-*/ (all yaml/yml files)
    if [ -d "$REPO_ROOT/manifests" ]; then
        while IFS= read -r -d '' crd_file; do
            if grep -q "kind: CustomResourceDefinition" "$crd_file" 2>/dev/null; then
                local name
                name=$(grep -m1 "^  name:" "$crd_file" 2>/dev/null | awk '{print $2}')
                [ -n "$name" ] && crds+=("$name")
            fi
        done < <(find "$REPO_ROOT/manifests" \( -name "*.yaml" -o -name "*.yml" \) -type f -print0 2>/dev/null)
    fi

    # Remove duplicates and sort
    printf '%s\n' "${crds[@]}" | sort -u
}

# Extract component definitions from manifests
extract_components() {
    local -A components

    if [ -d "$REPO_ROOT/manifests" ]; then
        for state_dir in "$REPO_ROOT"/manifests/state-*/; do
            [ -d "$state_dir" ] || continue

            for manifest in "$state_dir"/*.yaml "$state_dir"/*.yml; do
                [ -f "$manifest" ] || continue

                local kind=""
                local label=""
                local comp_name=""

                if grep -q "kind: DaemonSet" "$manifest" 2>/dev/null; then
                    kind="daemonset"
                elif grep -q "kind: Deployment" "$manifest" 2>/dev/null; then
                    kind="deployment"
                else
                    continue
                fi

                # Extract component name from metadata.name in the manifest
                comp_name=$(grep -A2 "^metadata:" "$manifest" 2>/dev/null | grep "name:" | head -1 | awk '{print $2}')
                # Fallback to directory name if metadata.name not found
                [ -z "$comp_name" ] && comp_name=$(basename "$state_dir" | sed 's/state-//')

                # Skip if already found for this component
                [ -n "${components[$comp_name]}" ] && continue

                # Extract labels from matchLabels section, skip lines with Go templates
                # Try to find a stable label (one without {{ }})
                local labels
                labels=$(grep -A10 "matchLabels:" "$manifest" 2>/dev/null | grep -v "matchLabels:" | grep -v "^--$" | grep -v "{{" | head -5)

                # Get first non-template label
                label=$(echo "$labels" | head -1 | sed 's/^ *//' | sed 's/: */=/' | tr -d ' ' | sed 's/=""$/=/')

                # If the manifest has Go templates and we found nvidia.com label, prefer it
                if grep -q "{{" "$manifest" 2>/dev/null; then
                    local nvidia_label
                    nvidia_label=$(echo "$labels" | grep "nvidia.com" | head -1 | sed 's/^ *//' | sed 's/: */=/' | tr -d ' ' | sed 's/=""$/=/')
                    [ -n "$nvidia_label" ] && label="$nvidia_label"
                fi

                [ -n "$label" ] && components["$comp_name"]="$label|$kind"
            done
        done
    fi

    # Output components
    for name in "${!components[@]}"; do
        echo "$name=${components[$name]}"
    done | sort
}

# Generate new CRD section
generate_crds_section() {
    echo "    # [GENERATED-CRDS-START] - Auto-generated, do not edit manually"
    echo "    # Generated at: $TIMESTAMP"
    echo "    # All CRDs we want to collect"
    echo "    local all_crds=("

    echo "        # Main Network Operator CRDs"
    for crd in $(extract_crds | grep -E "\.mellanox\.com$"); do
        echo "        \"$crd\""
    done

    echo "        # NV-IPAM CRDs"
    for crd in $(extract_crds | grep -E "\.nv-ipam\.nvidia\.com$"); do
        echo "        \"$crd\""
    done

    echo "        # NIC Configuration Operator CRDs"
    for crd in $(extract_crds | grep -E "\.configuration\.net\.nvidia\.com$"); do
        echo "        \"$crd\""
    done

    echo "        # Spectrum-X CRDs"
    for crd in $(extract_crds | grep -E "\.spectrumx\.nvidia\.com$"); do
        echo "        \"$crd\""
    done

    echo "        # Multus CRDs"
    echo "        \"network-attachment-definitions.k8s.cni.cncf.io\""

    echo "        # SR-IOV Operator CRDs (optional sub-chart)"
    echo "        \"sriovnetworknodepolicies.sriovnetwork.openshift.io\""
    echo "        \"sriovnetworknodestates.sriovnetwork.openshift.io\""
    echo "        \"sriovnetworks.sriovnetwork.openshift.io\""
    echo "        \"sriovibnetworks.sriovnetwork.openshift.io\""
    echo "        \"ovsnetworks.sriovnetwork.openshift.io\""
    echo "        \"sriovoperatorconfigs.sriovnetwork.openshift.io\""
    echo "        \"sriovnetworkpoolconfigs.sriovnetwork.openshift.io\""

    echo "        # Maintenance Operator CRDs (optional sub-chart)"
    echo "        \"nodemaintenances.maintenance.nvidia.com\""
    echo "        \"maintenanceoperatorconfigs.maintenance.nvidia.com\""

    echo "        # Node Feature Discovery CRDs (optional sub-chart)"
    echo "        \"nodefeatures.nfd.k8s-sigs.io\""
    echo "        \"nodefeaturerules.nfd.k8s-sigs.io\""

    echo "    )"
    echo "    # [GENERATED-CRDS-END]"
}

# Generate new components section
generate_components_section() {
    echo "    # [GENERATED-COMPONENTS-START] - Auto-generated, do not edit manually"
    echo "    # Generated at: $TIMESTAMP"
    echo "    # Component definitions: name, label, type"
    echo "    declare -A components=("

    echo "        # Main Network Operator components"
    for line in $(extract_components); do
        local name="${line%%=*}"
        local value="${line#*=}"
        echo "        [\"$name\"]=\"$value\""
    done

    # Add sub-chart components (not in manifests/)
    echo "        # Node Feature Discovery (sub-chart)"
    echo "        [\"nfd-master\"]=\"app.kubernetes.io/name=node-feature-discovery,app.kubernetes.io/component=master|deployment\""
    echo "        [\"nfd-worker\"]=\"app.kubernetes.io/name=node-feature-discovery,app.kubernetes.io/component=worker|daemonset\""
    echo "        [\"nfd-gc\"]=\"app.kubernetes.io/name=node-feature-discovery,app.kubernetes.io/component=gc|deployment\""

    echo "        # SR-IOV Network Operator (sub-chart)"
    echo "        [\"sriov-network-operator\"]=\"app=sriov-network-operator|deployment\""
    echo "        [\"sriov-network-config-daemon\"]=\"app=sriov-network-config-daemon|daemonset\""
    echo "        [\"sriov-network-resources-injector\"]=\"app=network-resources-injector|deployment\""

    echo "        # Maintenance Operator (sub-chart)"
    echo "        [\"maintenance-operator\"]=\"app.kubernetes.io/name=maintenance-operator|deployment\""

    echo "    )"
    echo "    # [GENERATED-COMPONENTS-END]"
}

# Verify mode - check if maps are current
verify_maps() {
    log_info "Verifying CRD and component maps are current..."

    local temp_crds
    temp_crds=$(mktemp)
    local temp_components
    temp_components=$(mktemp)

    generate_crds_section > "$temp_crds"
    generate_components_section > "$temp_components"

    local current_crds
    current_crds=$(sed -n '/# \[GENERATED-CRDS-START\]/,/# \[GENERATED-CRDS-END\]/p' "$SCRIPT_FILE")

    local current_components
    current_components=$(sed -n '/# \[GENERATED-COMPONENTS-START\]/,/# \[GENERATED-COMPONENTS-END\]/p' "$SCRIPT_FILE")

    local crds_match=true
    local components_match=true

    # Compare (ignoring timestamps)
    if ! diff -q <(echo "$current_crds" | grep -v "Generated at:") <(cat "$temp_crds" | grep -v "Generated at:") &>/dev/null; then
        crds_match=false
    fi

    if ! diff -q <(echo "$current_components" | grep -v "Generated at:") <(cat "$temp_components" | grep -v "Generated at:") &>/dev/null; then
        components_match=false
    fi

    rm -f "$temp_crds" "$temp_components"

    if [ "$crds_match" = true ] && [ "$components_match" = true ]; then
        log_info "Maps are current"
        return 0
    else
        log_error "Maps are out of date. Run 'make generate-sosreport-maps' to update."
        [ "$crds_match" = false ] && log_warn "  - CRDs need updating"
        [ "$components_match" = false ] && log_warn "  - Components need updating"
        return 1
    fi
}

# Update maps in the script
update_maps() {
    log_info "Generating CRD and component maps..."

    # Check if script file exists
    if [ ! -f "$SCRIPT_FILE" ]; then
        log_error "Script file not found: $SCRIPT_FILE"
        exit 1
    fi

    # Check for markers
    if ! grep -q "\[GENERATED-CRDS-START\]" "$SCRIPT_FILE"; then
        log_error "CRD markers not found in script. Please add markers manually."
        exit 1
    fi

    if ! grep -q "\[GENERATED-COMPONENTS-START\]" "$SCRIPT_FILE"; then
        log_error "Component markers not found in script. Please add markers manually."
        exit 1
    fi

    # Create temp file
    local temp_file
    temp_file=$(mktemp)

    # Generate new CRDs section
    local crds_section
    crds_section=$(generate_crds_section)

    # Generate new components section
    local components_section
    components_section=$(generate_components_section)

    # Replace CRDs section
    awk -v replacement="$crds_section" '
        /# \[GENERATED-CRDS-START\]/ { print replacement; skip=1; next }
        /# \[GENERATED-CRDS-END\]/ { skip=0; next }
        !skip { print }
    ' "$SCRIPT_FILE" > "$temp_file"

    # Replace components section
    awk -v replacement="$components_section" '
        /# \[GENERATED-COMPONENTS-START\]/ { print replacement; skip=1; next }
        /# \[GENERATED-COMPONENTS-END\]/ { skip=0; next }
        !skip { print }
    ' "$temp_file" > "${temp_file}.2"

    mv "${temp_file}.2" "$SCRIPT_FILE"
    rm -f "$temp_file"

    local crd_count
    crd_count=$(extract_crds | wc -l)
    local component_count
    component_count=$(extract_components | wc -l)

    log_info "Updated: $SCRIPT_FILE"
    log_info "  - CRDs: $crd_count (from manifests) + 12 (sub-charts)"
    log_info "  - Components: $component_count (from manifests) + 7 (sub-charts)"
}

# Main
main() {
    case "${1:-}" in
        --verify)
            verify_maps
            ;;
        --help|-h)
            echo "Usage: $0 [--verify]"
            echo ""
            echo "Generate or verify CRD and component maps for sosreport script."
            echo ""
            echo "Options:"
            echo "  --verify    Check if maps are current (exit 1 if outdated)"
            echo "  --help      Show this help message"
            echo ""
            echo "Without options, updates the maps in kubectl-netop_sosreport."
            ;;
        "")
            update_maps
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
}

main "$@"
