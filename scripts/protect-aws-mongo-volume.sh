#!/usr/bin/env bash
set -euo pipefail

# Run on the AWS EC2 host. This protects the currently bound MongoDB volume;
# it does not recover data from a previously deleted volume.
claim=mongodb-data
namespace=chatbox1
pv=$(sudo k3s kubectl -n "$namespace" get pvc "$claim" -o jsonpath='{.spec.volumeName}')
if [[ -z "$pv" ]]; then
  echo "MongoDB PVC is not bound; no changes made." >&2
  exit 1
fi

sudo k3s kubectl patch pv "$pv" --type merge -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'
sudo k3s kubectl -n "$namespace" annotate pvc "$claim" 'argocd.argoproj.io/sync-options=Prune=false' --overwrite
sudo k3s kubectl get pv "$pv" -o custom-columns=NAME:.metadata.name,RECLAIM:.spec.persistentVolumeReclaimPolicy,STATUS:.status.phase
