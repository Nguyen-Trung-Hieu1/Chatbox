# Monitoring GitOps

Argo CD manages the monitoring stack in the `monitoring` namespace.

| Component | Chart | Pinned version | Values |
| --- | --- | --- | --- |
| Prometheus and Grafana | `kube-prometheus-stack` | `91.4.0` | `prometheus-values.yaml` |
| Loki | `loki` | `18.13.1` | `loki-values.yaml` |
| Alloy | `alloy` | `1.12.1` | `alloy-values.yaml` |

The root application is `argocd/monitoring-stack.yaml`. Apply that file once to
bootstrap the stack. Afterward, Argo CD reads all changes from Git and reconciles
them automatically.

Grafana continues to use the existing `monitoring-grafana` Kubernetes Secret for
its administrator credentials. The secret is intentionally not stored in Git.
