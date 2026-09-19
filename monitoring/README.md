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

## Immediate Pod notifications

`monitoring/resources/pod-watcher.yaml` runs the small Go watcher built into the
backend image. Its service account can list and watch Pods only in `chatbox1`.
It reads the existing `alertmanager-telegram` Secret in `monitoring` and sends
plain Telegram messages when a Pod is deleted, becomes NotReady after being
Ready, enters Failed/Unknown, or gets stuck in CrashLoopBackOff/ImagePullBackOff.
The watcher does not identify who deleted a Pod. Normal deployments also delete
old Pods and therefore generate notifications. Kubernetes watch delivery is
usually quick but is not a guaranteed real-time audit trail.

Prometheus and Alertmanager remain installed for metrics-based alerts, including
Deployment replicas unavailable for over one minute. The watcher is independent
of Prometheus and does not require application stdout logs or K3s audit logging.

To test without interrupting Chatbox, create a temporary Pod after the watcher
Deployment is Ready, then delete it:

```bash
sudo k3s kubectl -n chatbox1 run pod-alert-test --image=busybox:1.36 --restart=Never --command -- sleep 3600
sudo k3s kubectl -n chatbox1 delete pod pod-alert-test
```
