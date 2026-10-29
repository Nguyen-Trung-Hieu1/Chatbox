# Run the Kubernetes storage bootstrap on the existing EC2 instance through SSM.
# The database password stays in the pre-existing keycloak-db Kubernetes Secret.
resource "aws_ssm_document" "keycloak_postgres_storage" {
  name          = "${var.name}-keycloak-postgres-storage"
  document_type = "Command"

  content = jsonencode({
    schemaVersion = "2.2"
    description   = "Create the persistent volume claim for Keycloak PostgreSQL"
    mainSteps = [{
      action = "aws:runShellScript"
      name   = "createPostgresVolumeClaim"
      inputs = {
        runCommand = [<<-SCRIPT
          set -eu
          k3s kubectl get namespace keycloak >/dev/null
          k3s kubectl -n keycloak get secret keycloak-db >/dev/null
          k3s kubectl apply -f - <<'YAML'
          apiVersion: v1
          kind: PersistentVolumeClaim
          metadata:
            name: keycloak-postgres-data
            namespace: keycloak
          spec:
            accessModes:
              - ReadWriteOnce
            storageClassName: local-path
            resources:
              requests:
                storage: 5Gi
          YAML
        SCRIPT
        ]
      }
    }]
  })
}

resource "aws_ssm_association" "keycloak_postgres_storage" {
  name = aws_ssm_document.keycloak_postgres_storage.name

  targets {
    key    = "InstanceIds"
    values = [aws_instance.chatbox.id]
  }
}
