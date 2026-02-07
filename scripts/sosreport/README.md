# SOS-Report Collection Script

### Overview

The Network Operator SOS-Report script collects comprehensive diagnostic data from a Kubernetes cluster running the NVIDIA Network Operator. This tool is designed to help troubleshoot issues by gathering all relevant configuration, logs, and status information in a single archive.

**File**: `kubectl-netop_sosreport` (also available as `network-operator-sosreport.sh` for backward compatibility)

### Installation

The script can be used in two ways:

#### 1. As a kubectl Plugin (Recommended)

```bash
# Install system-wide
sudo cp kubectl-netop_sosreport /usr/local/bin/

# Or user-only
mkdir -p ~/.local/bin
cp kubectl-netop_sosreport ~/.local/bin/
export PATH="$HOME/.local/bin:$PATH"

# Use as kubectl command
kubectl netop-sosreport --help
```

#### 2. As a Standalone Script

```bash
# Run directly from scripts directory
./kubectl-netop_sosreport --help

# Or use the backward-compatible symlink
./network-operator-sosreport.sh --help
```

### Features

- **Automatic Detection**: Auto-detects the Network Operator namespace
- **Comprehensive Collection**: Gathers CRDs, workloads, logs, and diagnostic data
- **Graceful Error Handling**: Continues collection even if some resources fail
- **Configurable**: Multiple options to customize the collection behavior
- **Platform Aware**: Detects OpenShift vs vanilla Kubernetes

### What's Collected

#### Custom Resources
- NicClusterPolicy, MacVlanNetwork, HostDeviceNetwork, IPoIBNetwork
- IPPools, CIDRPools (NV-IPAM)
- NICDevice, NICConfigurationTemplate, NICFirmwareResource, etc.
- NetworkAttachmentDefinitions (Multus)
- SR-IOV resources (if present)
- Node maintenance objects (if present)

#### Operator Resources
- Deployment, Pods, ConfigMaps, Secrets (metadata only)
- RBAC resources (ServiceAccounts, Roles, RoleBindings)
- Events in operator namespace
- Webhook configurations

#### Component Workloads (15 components)
- OFED Driver
- NIC Feature Discovery
- SR-IOV Device Plugin
- RDMA Shared Device Plugin
- Multus CNI
- Container Networking Plugins
- IPoIB CNI
- IB Kubernetes
- NV-IPAM (Node and Controller)
- NIC Configuration Operator (Daemon and Controller)
- DOCA Telemetry Service
- Spectrum-X Operator

For each component:
- DaemonSet/Deployment specifications
- All pod details and status
- Current and previous logs (if restarted)
- Related ConfigMaps and Services

#### Node Information
- All node details with labels and annotations
- Node conditions and status
- Allocatable resources (RDMA, SR-IOV, GPU)
- Node-specific labels (feature.node.kubernetes.io/*)

#### Diagnostic Commands (from OFED pods)
- `lsmod | grep mlx` - Loaded Mellanox modules
- `ibstat` - InfiniBand device status
- `ibv_devinfo` - RDMA device information
- `mst status` - Mellanox Software Tools status
- Kernel version
- Recent dmesg output
- Network interface information

#### Cluster-Wide Resources
- Kubernetes version
- API resources list
- NetworkAttachmentDefinitions
- Related operators (SR-IOV, Maintenance)

### Usage

#### Basic Usage

```bash
# Run with auto-detection (recommended)
./network-operator-sosreport.sh

# Specify kubeconfig explicitly
./network-operator-sosreport.sh --kubeconfig /path/to/kubeconfig

# Specify operator namespace
./network-operator-sosreport.sh --namespace nvidia-network-operator
```

#### Advanced Options

```bash
# Collect more log lines per pod
./network-operator-sosreport.sh --log-lines 10000

# Skip diagnostic commands for faster collection
# This skips executing commands inside OFED pods:
#   - lsmod | grep mlx (loaded Mellanox kernel modules)
#   - ibstat (InfiniBand device status)
#   - ibv_devinfo (RDMA device information)
#   - mst status (Mellanox Software Tools status)
#   - dmesg (kernel messages)
#   - ip link/addr (network interface information)
./network-operator-sosreport.sh --skip-diagnostics

# Don't compress, leave as directory
./network-operator-sosreport.sh --no-compress

# Custom output directory
./network-operator-sosreport.sh --output-dir /tmp/my-sosreport

# Verbose output during collection
./network-operator-sosreport.sh --verbose
```

#### Full Command-Line Options

```
Options:
  --kubeconfig PATH           Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)
  --namespace NAMESPACE       Network operator namespace (default: auto-detect)
  --output-dir PATH          Output directory (default: ./network-operator-sosreport-<timestamp>)
  --no-compress              Don't create tarball, leave as directory
  --log-lines N              Number of log lines to collect per pod (default: 5000)
  --skip-diagnostics         Skip running diagnostic commands in OFED pods
                             (lsmod, ibstat, ibv_devinfo, mst, dmesg, ip commands)
                             Use this for faster collection when driver-level diagnostics are not needed
  --kubectl-path PATH        Path to kubectl binary (default: kubectl from PATH)
  --verbose                  Verbose output during collection
  --help                     Show help message
```

### Output Structure

The script creates a timestamped archive with the following structure:

```
network-operator-sosreport-<timestamp>/
├── metadata/
│   ├── collection-info.txt           # Script version, collection time, cluster info
│   ├── cluster-version.yaml          # Kubernetes/OpenShift version
│   ├── namespaces.txt                # List of all namespaces
│   └── api-resources.txt             # Available API resources
├── crds/
│   ├── definitions/                  # CRD definitions (schemas)
│   │   ├── nicclusterpolicies.mellanox.com.yaml
│   │   ├── ippools.nv-ipam.nvidia.com.yaml
│   │   └── ...
│   └── instances/                    # CR instances (actual resources)
│       ├── nicclusterpolicies/
│       ├── ippools/
│       └── ...
├── operator/
│   ├── namespace.yaml                # Operator namespace details
│   ├── configmaps.yaml               # ConfigMaps in operator namespace
│   ├── secrets-metadata.txt          # Secret names only (no data)
│   ├── rbac/                         # Roles, RoleBindings, etc.
│   ├── events.yaml                   # Namespace events
│   ├── validatingwebhookconfigurations.yaml
│   ├── mutatingwebhookconfigurations.yaml
│   └── components/
│       ├── network-operator/         # Operator controller
│       │   ├── deployment.yaml
│       │   └── pods/
│       │       ├── <pod-name>.yaml
│       │       ├── <pod-name>.log
│       │       └── <pod-name>-previous.log
│       ├── ofed-driver/
│       │   ├── daemonset.yaml
│       │   ├── pods/
│       │   └── diagnostics/          # Host commands output
│       ├── rdma-shared-dp-ds/
│       ├── sriov-device-plugin/
│       ├── nv-ipam-node/
│       ├── nv-ipam-controller/
│       └── ...                       # All components
├── nodes/
│   ├── all-nodes.yaml                # All nodes with full details
│   ├── nodes-summary.txt             # Node summary table
│   ├── node-labels.txt               # Extracted labels
│   └── node-resources.txt            # Allocatable resources summary
├── network/
│   └── services.yaml
├── related-operators/
│   └── sriov-network-operator/       # If present
├── diagnostic-summary.txt            # Quick summary and issues
└── collection-errors.log             # Any errors during collection
```

### Output Archive

By default, the script creates a compressed tarball:
- `network-operator-sosreport-<timestamp>.tar.gz` - Compressed archive
- `network-operator-sosreport-<timestamp>.tar.gz.sha256` - Checksum file

### Requirements

- `kubectl` binary installed and in PATH
- Valid kubeconfig with cluster access
- Permissions to read resources (cluster-admin recommended)
- Bash 4.0 or later
- Standard Unix utilities (tar, gzip, sha256sum)

### Exit Codes

- `0` - Success, all data collected
- `1` - Critical error (no kubectl, no cluster access)
- `2` - Partial success (some resources failed to collect)

### Troubleshooting

#### Script can't find kubectl
```bash
./network-operator-sosreport.sh --kubectl-path /usr/local/bin/kubectl
```

#### Permission denied errors
The script requires read access to all resources. Run with a user that has cluster-admin privileges, or expect some resources to be skipped.

#### Collection takes too long
- Use `--skip-diagnostics` to skip diagnostic commands in OFED pods (lsmod, ibstat, ibv_devinfo, etc.)
- Use `--log-lines 1000` to collect fewer log lines

#### Can't detect operator namespace
```bash
./network-operator-sosreport.sh --namespace <your-namespace>
```

### Security Considerations

- **Secrets**: Only metadata (names, types) are collected, not secret data
- **Logs**: May contain IP addresses and hostnames
- **Review**: Always review the collected data before sharing externally

### Best Practices

1. **Run immediately when issues occur** - Logs and events are time-sensitive
2. **Include recent changes** - Note any configuration changes before running
3. **Compress for sharing** - Use default compression for easier sharing
4. **Check the summary** - Review `diagnostic-summary.txt` first
5. **Check errors** - Review `collection-errors.log` for missing data

### Example Workflows

#### Troubleshooting Pod Failures
```bash
# Collect full diagnostics
./network-operator-sosreport.sh --verbose

# Check the summary
tar -xzf network-operator-sosreport-*.tar.gz
cat network-operator-sosreport-*/diagnostic-summary.txt

# Look at specific component logs
cat network-operator-sosreport-*/operator/components/ofed-driver/pods/*.log
```

#### Quick Health Check
```bash
# Fast collection without diagnostics
./network-operator-sosreport.sh --skip-diagnostics --log-lines 1000

# Extract and check summary
tar -xzf network-operator-sosreport-*.tar.gz
less network-operator-sosreport-*/diagnostic-summary.txt
```

#### Preparing for Support Case
```bash
# Comprehensive collection with verbose output
./network-operator-sosreport.sh --verbose --log-lines 10000

# Verify the archive
sha256sum -c network-operator-sosreport-*.tar.gz.sha256

# Upload to support case
# (archive is ready to share)
```
