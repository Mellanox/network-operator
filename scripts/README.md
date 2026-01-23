# Network Operator Scripts

This directory contains utility scripts for the NVIDIA Network Operator.

## Subdirectories

### sosreport/

Comprehensive diagnostic collection tool for troubleshooting Network Operator issues.

**Main script**: `sosreport/kubectl-netop_sosreport`

**Quick start**:
```bash
# As kubectl plugin (install first)
sudo cp sosreport/kubectl-netop_sosreport /usr/local/bin/
kubectl netop-sosreport

# Or run directly
./sosreport/kubectl-netop_sosreport

# Get help
./sosreport/kubectl-netop_sosreport --help
```

**Documentation**:
- [sosreport/README.md](sosreport/README.md) - Complete usage guide

See [sosreport/README.md](sosreport/README.md) for detailed documentation.

### releases/

Contains release-related scripts and utilities.
