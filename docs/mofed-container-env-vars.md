# MOFED Driver Container Environment Variables

When creating _nicclustercpolicy_ instance for Network Operator, under `ofedDriver` section
it is possible to specify a list of environment variables to be exposed to mofed driver container.

These variables affect the operation of the container.

__Example__

```yaml
ofedDriver:
  env:
  - name: CREATE_IFNAMES_UDEV
    value: "true"
```

The use of these Environment variables is intended for advanced cases.
In most deployments its expected to not require setting those.

| Name | Experimental | Default | Description |
| ---- | ------------ | ------- | ----------- |
| CREATE_IFNAMES_UDEV |N|`"false"`| create udev rule to preserve "old-style" path based netdev names e.g `enp3s0f0`|
| UNLOAD_STORAGE_MODULES |N|`"false"`| unload host storage modules prior to loading mofed modules  |
| ENABLE_NFSRDMA |N|`"false"`| enable loading of nfs relates storage modules from mofed container|
| RESTORE_DRIVER_ON_POD_TERMINATION |N|`"true"`| restore host drivers when container is gracefully stopped |
| NVIDIA_NIC_DRIVERS_INVENTORY_PATH | N | `"/mnt/drivers-inventory"` | enable use of a persistent directory to store drivers' build artifacts to avoid recompilation between runs. Keep the default value or set to "" to disable. |
| UNLOAD_THIRD_PARTY_RDMA_MODULES | N | `"false"` | When `true`, all known third-party RDMA kernel modules (from rdma-core: qedr, efa, siw, etc.) are blacklisted and unloaded before OFED driver reload. The init container will skip these modules in its dependency check when this flag is set. |
| STORAGE_MODULES | N | `"ib_iser ib_isert ib_srp ib_srpt nvme_rdma nvmet_rdma rpcrdma xprtrdma"` | Space-separated list of storage-over-RDMA kernel modules to unload when `UNLOAD_STORAGE_MODULES="true"`. Override this only when the default list does not match your host's storage stack. |
| THIRD_PARTY_RDMA_MODULES | N | `"qedr efa siw bnxt_re"` (extensible) | Space-separated list of third-party NIC vendor RDMA provider modules to unload when `UNLOAD_THIRD_PARTY_RDMA_MODULES="true"`. The default covers the common rdma-core consumers (`qedr`, `efa`, `siw`, `bnxt_re`). Override this only to add or remove vendor modules specific to your deployment. |
| SKIP_PREFLIGHT_CHECKS | N | `"true"` | When `true` (default — applied by the init container binary when the env var is unset), the init container skips the module dependency check and succeeds immediately; pre-flight runs only on explicit opt-in. When `false`, the check runs and any finding causes the init container to exit non-zero, blocking pod init until the host is remediated. Opt in by adding `{name: SKIP_PREFLIGHT_CHECKS, value: "false"}` to `spec.ofedDriver.env`. |

In addition, the user can specify essentially any environment variables to be exposed to the MOFED container such as
the standard `"HTTP_PROXY"`, `"HTTPS_PROXY"`, `"NO_PROXY"`

> __Note__: `CREATE_IFNAMES_UDEV` is being set automatically by Network Operator depenting of the Operating System of worker nodes
> in the cluster (cluster is assumed to be homogenous).

> __Note__: Environment Variables Marked as Experimental should not be used in production environment, they are mainly aimed at
> either providing special functionality or additional debug ability. Such variables can be added/removed with no notice.

## Init Container Module Dependency Detection

When the MOFED driver init container is enabled, it runs a module dependency check before the main driver container starts. This check detects third-party kernel modules that depend (directly or transitively) on NVIDIA MOFED RDMA/MLX5 modules.

### How it works

The init container reads `/proc/modules` and `/sys/module/*/holders/` on the host to build a full reverse dependency tree starting from the configured MOFED modules (e.g., `mlx5_core`, `ib_core`). It traverses the tree to find all non-MOFED modules that depend on any MOFED module, including transitive dependencies (e.g., `lustre → ko2iblnd → ib_core`).

### Behavior on failure

By default (`SKIP_PREFLIGHT_CHECKS="true"`) the init container does not run the module dependency check at all — init succeeds immediately and the main driver container proceeds with MOFED load. The Network Operator injects this default onto the init container in the rendered DaemonSet; users who want the check must explicitly opt in.

When `SKIP_PREFLIGHT_CHECKS="false"`, the init container runs the check and exits with a non-zero exit code on any blocking dependency, logging each offending module along with what it depends on. Because this is a Kubernetes init container, the main driver container will not start until the init container succeeds. This prevents the OFED driver reload from being attempted when it would fail and leave the node in a broken state, at the cost of requiring the host to be clean before the pod can progress.

### Resolution

The init container classifies blocking dependencies into three categories:

1. **Third-party RDMA modules** (e.g., `qedr`, `efa`, `siw`, `bnxt_re`): These are known rdma-core consumer modules. Set `UNLOAD_THIRD_PARTY_RDMA_MODULES="true"` in the NicClusterPolicy `ofedDriver` env vars to automatically blacklist and unload them before driver reload. Verify that no running workloads depend on these modules before enabling.
2. **Unknown kernel modules**: Modules not recognized as third-party RDMA modules require manual intervention. Unload or blacklist them on the host before the driver container starts (e.g., `modprobe -r <module>` or via a DaemonSet).
3. **Userspace processes**: Processes holding MOFED device files open (e.g., `/dev/infiniband/*`). Identify them with `lsof /dev/infiniband/*` or `fuser -v /dev/infiniband/*` and stop them before deploying the driver. Common culprits: `opensm`, `ibacm`, `rdma-ndd`.
