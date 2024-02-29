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

In addition, the user can specify essentially any environment variables to be exposed to the MOFED container such as
the standard `"HTTP_PROXY"`, `"HTTPS_PROXY"`, `"NO_PROXY"`

> __Note__: `CREATE_IFNAMES_UDEV` is being set automatically by Network Operator depenting of the Operating System of worker nodes
> in the cluster (cluster is assumed to be homogenous).

> __Note__: Environment Variables Marked as Experimental should not be used in production environment, they are mainly aimed at
> either providing special functionality or additional debug ability. Such variables can be added/removed with no notice.
