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

## Pod event notifications (paused)

Alloy reads Kubernetes Events in `chatbox1` directly through
`loki.source.kubernetes_events` in `alloy-values.yaml`. It sends them to Loki as
JSON log lines with `job="kubernetes-events"`. The Loki ruler is currently
disabled in `loki-values.yaml` after the application returned HTTP 502; do not
assume this Event-based Telegram alert is active. When enabled, the ruler loads the LogQL rule
from `resources/pod-event-rules.yaml` and forwards matching Pod Events to the
existing Prometheus Alertmanager, which sends Telegram notifications through
the existing `alertmanager-telegram` Secret.

The rule matches `Killing`, `BackOff`, `Unhealthy`, `FailedKillPod`, and `Evicted`
Events. Normal deployments can emit `Killing`, so these notifications do not
necessarily mean an outage. Kubernetes does not guarantee that deleting a Pod
creates a Kubernetes Event; this path cannot guarantee an alert for every Pod
deletion or identify who requested it. The Loki rule evaluates every 10 seconds,
so delivery is not instantaneous. Existing Prometheus rules remain responsible
for prolonged missing replicas and container restarts.
