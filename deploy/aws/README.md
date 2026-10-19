# MongoDB data on the AWS K3s node

`mongodb-data` is a local-path PVC on the EC2 root EBS volume. The PVC manifest
has `Prune=false` so Argo CD does not remove it when a sync omits the manifest.
For an existing bound volume, run `scripts/protect-aws-mongo-volume.sh` once on
the EC2 host to change the PV reclaim policy from `Delete` to `Retain`.

These settings protect the **current** volume from PVC pruning or deletion.
They do not recover an older deleted volume, protect against loss of the EC2
root EBS volume, or replace backups. Take a verified MongoDB backup or EBS
snapshot before making storage changes. A newly provisioned empty PVC does
not contain the previous users, wallets, or conversations.
