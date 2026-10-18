# New AWS account infrastructure

This directory creates only the base infrastructure for the new account
`421344955349` in Sydney: one EC2 server, one encrypted 40 GiB EBS root
volume, an Elastic IP, HTTPS/HTTP firewall rules, and an SSM role.

It deliberately has an AWS account guard, so it cannot apply to the old
account `172155959241`. SSH is not exposed; server maintenance uses AWS
Systems Manager Session Manager.

The separate bootstrap step installs K3s, Argo CD, Grafana and Chatbox. It
will restore the private 9router data locally on the server, never in Git or
Terraform state. Chatbox's MongoDB starts clean, as requested.

## Apply

Run from this directory after configuring the `chatbox-new` AWS CLI profile:

```powershell
terraform init
terraform plan -out new-account.tfplan
terraform apply new-account.tfplan
```

Terraform output returns the instance ID and the new Elastic IP. Update the
`A` records for `chatbox.cyou` and `grafana.chatbox.cyou` at Spaceship after
the application health checks pass.
