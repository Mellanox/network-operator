# maintenance-operator-chart

![Version: 0.0.1](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square)  ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)  ![AppVersion: latest](https://img.shields.io/badge/AppVersion-latest-informational?style=flat-square)

Maintenance Operator Helm Chart

## Resource sizing

Operator memory usage is dominated by the Kubernetes **Node informer cache**, so it
scales primarily with **cluster node count** (and node object size), not with
NodeMaintenance request volume.

| Approx. nodes | Suggested `operator.resources.limits.memory` |
|---------------|----------------------------------------------|
| ≤ ~150 | `256Mi` (chart default; verified stable around this size) |
| ~250–300 | raise above `256Mi` (default may be exhausted) |
| ~1000+ | plan for hundreds of MiB to ~1Gi+, depending on node density |

Raise `operator.resources.requests.memory` alongside the limit so the scheduler
and QoS class stay consistent.

### NVIDIA Network Operator installs

If this chart is deployed as a subchart of [network-operator](https://github.com/Mellanox/network-operator),
override resources under the **subchart** key:

```yaml
maintenance-operator-chart:
  operator:
    resources:
      limits:
        cpu: 500m
        memory: 256Mi
      requests:
        cpu: 10m
        memory: 192Mi
```

Do **not** confuse this with the parent chart top-level `operator.resources` —
that setting applies to the **network-operator** controller, not maintenance-operator.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| imagePullSecrets | list | `[]` | image pull secrets for the operator |
| metricsService | object | `{"ports":[{"name":"https","port":8443,"protocol":"TCP","targetPort":"https"}],"type":"ClusterIP"}` | metrics service configurations |
| operator.admissionController.certificates.certManager.enable | bool | `true` | use cert-manager for certificates |
| operator.admissionController.certificates.certManager.issuerRef | object | `{}` | reference to an existing cert-manager issuer that signs the certificate. When `name` is empty a self-signed issuer is created and used. Set this to chain the admission controller certificate to a certificate authority you own. `kind` defaults to `Issuer` and `group` to `cert-manager.io` |
| operator.admissionController.certificates.custom.enable | bool | `false` | enable custom certificates using secrets |
| operator.admissionController.certificates.secretNames.operator | string | `"operator-webhook-cert"` | secret name containing certificates for the operator admission controller |
| operator.admissionController.enable | bool | `true` | enable admission controller of the operator |
| operator.affinity | object | `{"nodeAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]},"weight":1},{"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]},"weight":1}]}}` | node affinity for the operator |
| operator.image.imagePullPolicy | string | `nil` | image pull policy for the operator image |
| operator.image.name | string | `"maintenance-operator"` | image name to use for the operator image |
| operator.image.repository | string | `"ghcr.io/mellanox"` | repository to use for the operator image |
| operator.image.tag | string | `nil` | image tag to use for the operator image |
| operator.nodeSelector | object | `{}` | node selector for the operator |
| operator.replicas | int | `1` | operator deployment number of repplicas |
| operator.resources | object | `{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"10m","memory":"192Mi"}}` | Resource requests and limits for the operator. Memory usage is dominated by the Node informer cache and therefore scales with cluster node count, not requestor workload. The default limit of 256Mi is a reasonable baseline for mid-size clusters (about 130–156Mi idle working set was measured on ~141 nodes); raise further for larger clusters (for example toward 1Gi when expecting substantial growth or GPU-dense nodes with large status payloads). When installed via NVIDIA Network Operator, override with `maintenance-operator-chart.operator.resources` — not the parent chart top-level `operator.resources`, which configures network-operator itself. |
| operator.serviceAccount.annotations | object | `{}` | set annotations for the operator service account |
| operator.tolerations | list | `[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]` | toleration for the operator |
| operatorConfig | object | `{"deploy":false,"logLevel":"info","maxNodeMaintenanceTimeSeconds":null,"maxParallelOperations":null,"maxUnavailable":null}` | operator configuration values. fields here correspond to fields in MaintenanceOperatorConfig CR |
| operatorConfig.deploy | bool | `false` | deploy operatorConfig CR with the below values |
| operatorConfig.logLevel | string | `"info"` | log level configuration |
| operatorConfig.maxNodeMaintenanceTimeSeconds | string | `nil` | max time for node maintenance |
| operatorConfig.maxParallelOperations | string | `nil` | max number of parallel operations |
| operatorConfig.maxUnavailable | string | `nil` | max number of unavailable nodes |
| webhookService | object | `{"ports":[{"port":443,"protocol":"TCP","targetPort":9443}],"type":"ClusterIP"}` | webhook service configurations |
