# Deploy Keycloak on the existing K3s node through SSM. The database and
# bootstrap admin passwords come from Kubernetes Secrets, not Terraform state.
resource "aws_ssm_document" "keycloak_server" {
  name          = "${var.name}-keycloak-server"
  document_type = "Command"

  content = jsonencode({
    schemaVersion = "2.2"
    description   = "Deploy Keycloak on K3s"
    mainSteps = [{
      action = "aws:runShellScript"
      name   = "deployKeycloak"
      inputs = {
        runCommand = [<<-SCRIPT
          set -eu
          k3s kubectl get namespace keycloak >/dev/null
          k3s kubectl -n keycloak get secret keycloak-db >/dev/null
          k3s kubectl -n keycloak get secret keycloak-admin >/dev/null
          k3s kubectl -n keycloak get service keycloak-postgres >/dev/null
          k3s kubectl apply -f - <<'YAML'
          apiVersion: v1
          kind: Service
          metadata:
            name: keycloak
            namespace: keycloak
          spec:
            type: NodePort
            selector:
              app: keycloak
            ports:
              - name: http
                port: 8080
                targetPort: 8080
                nodePort: 30081
          ---
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: keycloak
            namespace: keycloak
          spec:
            replicas: 1
            strategy:
              type: Recreate
            selector:
              matchLabels:
                app: keycloak
            template:
              metadata:
                labels:
                  app: keycloak
              spec:
                containers:
                  - name: keycloak
                    image: quay.io/keycloak/keycloak:26.8.1
                    imagePullPolicy: IfNotPresent
                    args: ["start"]
                    env:
                      - name: KC_DB
                        value: postgres
                      - name: KC_DB_URL
                        value: jdbc:postgresql://keycloak-postgres.keycloak.svc.cluster.local:5432/keycloak
                      - name: KC_DB_USERNAME
                        value: keycloak
                      - name: KC_DB_PASSWORD
                        valueFrom:
                          secretKeyRef:
                            name: keycloak-db
                            key: password
                      - name: KC_BOOTSTRAP_ADMIN_USERNAME
                        valueFrom:
                          secretKeyRef:
                            name: keycloak-admin
                            key: username
                      - name: KC_BOOTSTRAP_ADMIN_PASSWORD
                        valueFrom:
                          secretKeyRef:
                            name: keycloak-admin
                            key: password
                      - name: KC_HOSTNAME
                        value: https://auth.chatbox.cyou
                      - name: KC_HTTP_ENABLED
                        value: "true"
                      - name: KC_PROXY_HEADERS
                        value: xforwarded
                      - name: KC_HEALTH_ENABLED
                        value: "true"
                    ports:
                      - name: http
                        containerPort: 8080
                      - name: management
                        containerPort: 9000
                    startupProbe:
                      httpGet:
                        path: /health/started
                        port: management
                      periodSeconds: 10
                      failureThreshold: 30
                    readinessProbe:
                      httpGet:
                        path: /health/ready
                        port: management
                      periodSeconds: 10
                    livenessProbe:
                      httpGet:
                        path: /health/live
                        port: management
                      periodSeconds: 20
                    resources:
                      requests:
                        cpu: 200m
                        memory: 1Gi
                      limits:
                        cpu: "2"
                        memory: 2Gi
          YAML
        SCRIPT
        ]
      }
    }]
  })
}

resource "aws_ssm_association" "keycloak_server" {
  name = aws_ssm_document.keycloak_server.name

  targets {
    key    = "InstanceIds"
    values = [aws_instance.chatbox.id]
  }

  depends_on = [aws_ssm_association.keycloak_postgres]
}
