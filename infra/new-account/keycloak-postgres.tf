# Bootstrap PostgreSQL on the existing K3s node through SSM. The database
# password is read from the existing Kubernetes Secret, never from Terraform.
resource "aws_ssm_document" "keycloak_postgres" {
  name          = "${var.name}-keycloak-postgres"
  document_type = "Command"

  content = jsonencode({
    schemaVersion = "2.2"
    description   = "Deploy PostgreSQL for Keycloak on K3s"
    mainSteps = [{
      action = "aws:runShellScript"
      name   = "deployPostgres"
      inputs = {
        runCommand = [<<-SCRIPT
          set -eu
          k3s kubectl get namespace keycloak >/dev/null
          k3s kubectl -n keycloak get secret keycloak-db >/dev/null
          k3s kubectl -n keycloak get pvc keycloak-postgres-data >/dev/null
          k3s kubectl apply -f - <<'YAML'
          apiVersion: v1
          kind: Service
          metadata:
            name: keycloak-postgres
            namespace: keycloak
          spec:
            type: ClusterIP
            selector:
              app: keycloak-postgres
            ports:
              - name: postgres
                port: 5432
                targetPort: 5432
          ---
          apiVersion: apps/v1
          kind: StatefulSet
          metadata:
            name: keycloak-postgres
            namespace: keycloak
          spec:
            serviceName: keycloak-postgres
            replicas: 1
            selector:
              matchLabels:
                app: keycloak-postgres
            template:
              metadata:
                labels:
                  app: keycloak-postgres
              spec:
                containers:
                  - name: postgres
                    image: postgres:17
                    imagePullPolicy: IfNotPresent
                    env:
                      - name: POSTGRES_DB
                        value: keycloak
                      - name: POSTGRES_USER
                        value: keycloak
                      - name: POSTGRES_PASSWORD
                        valueFrom:
                          secretKeyRef:
                            name: keycloak-db
                            key: password
                      - name: PGDATA
                        value: /var/lib/postgresql/data/pgdata
                    ports:
                      - containerPort: 5432
                        name: postgres
                    readinessProbe:
                      exec:
                        command: ["pg_isready", "-U", "keycloak", "-d", "keycloak"]
                      initialDelaySeconds: 10
                      periodSeconds: 10
                    resources:
                      requests:
                        cpu: 100m
                        memory: 128Mi
                      limits:
                        cpu: 500m
                        memory: 512Mi
                    volumeMounts:
                      - name: data
                        mountPath: /var/lib/postgresql/data
                volumes:
                  - name: data
                    persistentVolumeClaim:
                      claimName: keycloak-postgres-data
          YAML
        SCRIPT
        ]
      }
    }]
  })
}

resource "aws_ssm_association" "keycloak_postgres" {
  name = aws_ssm_document.keycloak_postgres.name

  targets {
    key    = "InstanceIds"
    values = [aws_instance.chatbox.id]
  }

  depends_on = [aws_ssm_association.keycloak_postgres_storage]
}
