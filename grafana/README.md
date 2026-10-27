# Network Operator Grafana dashboard

Import [network-operator-dashboard.json](network-operator-dashboard.json) into
Grafana to inspect component readiness, container restarts, controller activity,
and optional host RDMA port traffic and errors. The restart chart shows a rolling
five-minute count at each point, including events just before the displayed range.
The dashboard uses built-in Grafana time-series panels and a Prometheus data
source; it does not install exporters, configure scraping, or create alerts.

## Prerequisites and import

1. Use a Prometheus data source scoped to **one Kubernetes cluster**. For a
   multi-cluster backend, first configure a cluster-scoped data source or add
   your cluster label to every query and variable; namespace and job names alone
   do not distinguish clusters.
2. Scrape kube-state-metrics with its Deployment, DaemonSet and Pod collectors.
   Preserve the `job`, `namespace`, `daemonset`, `deployment`, `pod` and
   `container` labels. No Kubernetes label allowlist is needed.
3. In Grafana, select **Dashboards → New → Import**, upload the JSON, and import
   it. Choose your **Prometheus** data source and **kube-state-metrics job**.
4. Select **Component namespaces** containing Network Operator and its managed
   components (typically `nvidia-network-operator`). Include any separate
   namespaces used by sub-operators. All workloads in these namespaces are
   shown, including unrelated workloads if namespaces are shared. Names are
   discovered from metrics, so Helm release names and NicNodePolicy-generated
   DaemonSet names need no hardcoded list.

A desired replica/node count of zero is not proof of successful deployment.
Compare ready/available counts with desired counts and inspect Kubernetes events
when they differ. Readiness is not a throughput or connectivity test, and these
panels do not expose NicClusterPolicy/NicNodePolicy status fields directly.

## Optional controller panels

Scrape the Network Operator manager's `/metrics` endpoint, then set **Operator
scrape job** to that target's Prometheus `job` label (a regular expression is
accepted). The default `network-operator` is a placeholder; the dashboard does
not assume a particular ServiceMonitor naming convention. Choose a job that
contains only Network Operator targets in this cluster.

The manager defaults to HTTP on port 8080 (`--metrics-bind-address`). The Helm
chart does not automatically create a metrics Service or ServiceMonitor. Use
your monitoring stack's discovery configuration to reach that port and restrict
access according to your cluster policy. Kustomize installations may override
the address to loopback and use a metrics proxy; inspect the deployed arguments
and scrape the configured proxy endpoint instead. Controller panels use
`controller_runtime_reconcile_total` and
`controller_runtime_reconcile_errors_total`. Their scope is the selected job,
independent of **Component namespaces**.

## Optional RDMA panels

Scrape Prometheus node exporter on the relevant nodes with its `infiniband`
collector enabled and access to the host's `/sys/class/infiniband` tree. Set
**Node exporter scrape job** to the appropriate `job` label, then choose **RDMA
exporter instances**. These selectors are independent of component namespaces:
the counters describe host ports, not Kubernetes pods or policy ownership.

The panels use the following node-exporter counters with `instance`, `device`
and `port` labels:

- `node_infiniband_port_data_received_bytes_total`
- `node_infiniband_port_data_transmitted_bytes_total`
- `node_infiniband_port_errors_received_total`
- `node_infiniband_port_discards_transmitted_total`

Traffic is shown in bytes/second; the exporter already converts hardware data
units to bytes. Counter availability depends on the exporter, NIC and driver.
This dashboard does not use DOCA Telemetry Service's version-specific metric
schema and does not establish an expected throughput baseline.

## Missing data

“No data” is not a healthy/zero result. Check the selected data source, jobs and
namespaces, Prometheus target health, and whether the metric exists in Prometheus.
Rate queries need multiple samples; configure Grafana's Prometheus scrape interval
to match the actual scrape interval. Disabled or absent optional exporters leave
the corresponding panels empty. No missing-data fallback or alert thresholds are
supplied.

## Metric contracts

- [kube-state-metrics DaemonSet metrics](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/daemonset-metrics.md)
- [kube-state-metrics Deployment metrics](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/deployment-metrics.md)
- [kube-state-metrics Pod metrics](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/pod-metrics.md)
- [node exporter InfiniBand collector](https://github.com/prometheus/node_exporter/blob/master/collector/infiniband_linux.go)
