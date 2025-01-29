# maintenance-operator-chart

![Version: 0.0.1](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: latest](https://img.shields.io/badge/AppVersion-latest-informational?style=flat-square)

Maintenance Operator Helm Chart

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| imagePullSecrets | list | `[]` | image pull secrets for the operator |
| metricsService | object | `{"ports":[{"name":"https","port":8443,"protocol":"TCP","targetPort":"https"}],"type":"ClusterIP"}` | metrics service configurations |
| operator.admissionController.certificates.certManager.enable | bool | `true` | use cert-manager for certificates |
| operator.admissionController.certificates.certManager.generateSelfSigned | bool | `true` | generate self-signed certificiates with cert-manager |
| operator.admissionController.certificates.custom.enable | bool | `false` | enable custom certificates using secrets |
| operator.admissionController.certificates.secretNames.operator | string | `"operator-webhook-cert"` | secret name containing certificates for the operator admission controller |
| operator.admissionController.enable | bool | `true` | enable admission controller of the operator |
| operator.affinity | object | `{"nodeAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]},"weight":1},{"preference":{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]},"weight":1}]}}` | node affinity for the operator |
| operator.image.imagePullPolicy | string | `nil` | image pull policy for the operator image |
| operator.image.repository | string | `"ghcr.io/mellanox"` | repository to use for the operator image |
| operator.image.name | string | `"maintenance-operator"` | image name to use for the operator image |
| operator.image.tag | string | `nil` | image tag to use for the operator image |
| operator.nodeSelector | object | `{}` | node selector for the operator |
| operator.replicas | int | `1` | operator deployment number of repplicas |
| operator.resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | specify resource requests and limits for the operator |
| operator.serviceAccount.annotations | object | `{}` | set annotations for the operator service account |
| operator.tolerations | list | `[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]` | toleration for the operator |
| operatorConfig | object | `{"logLevel":"info","maxNodeMaintenanceTimeSeconds":null,"maxParallelOperations":null,"maxUnavailable":null}` | operator configuration values. fields here correspond to fields in MaintenanceOperatorConfig CR |
| operatorConfig.logLevel | string | `"info"` | log level configuration |
| operatorConfig.maxNodeMaintenanceTimeSeconds | string | `nil` | max time for node maintenance |
| operatorConfig.maxParallelOperations | string | `nil` | max number of parallel operations |
| operatorConfig.maxUnavailable | string | `nil` | max number of unavailable nodes |
| webhookService | object | `{"ports":[{"port":443,"protocol":"TCP","targetPort":9443}],"type":"ClusterIP"}` | webhook service configurations |

