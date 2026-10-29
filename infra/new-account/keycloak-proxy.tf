# Add the Keycloak HTTPS virtual host to the Caddy installation already
# running on the EC2 instance. The NodePort is not opened in the EC2 firewall.
resource "aws_ssm_document" "keycloak_proxy" {
  name          = "${var.name}-keycloak-proxy"
  document_type = "Command"

  content = jsonencode({
    schemaVersion = "2.2"
    description   = "Configure Caddy HTTPS proxy for Keycloak"
    mainSteps = [{
      action = "aws:runShellScript"
      name   = "configureKeycloakProxy"
      inputs = {
        runCommand = [<<-SCRIPT
          set -eu
          config=/etc/caddy/Caddyfile
          backup=/etc/caddy/Caddyfile.before-keycloak
          if grep -Eq '^auth[.]chatbox[.]cyou[[:space:]]*\{' "$config"; then
            caddy validate --config "$config" --adapter caddyfile
            exit 0
          fi
          temporary=$(mktemp)
          cp "$config" "$temporary"
          cat >> "$temporary" <<'CADDY'

          auth.chatbox.cyou {
            reverse_proxy 127.0.0.1:30081
          }
          CADDY
          caddy validate --config "$temporary" --adapter caddyfile
          cp "$config" "$backup"
          cp "$temporary" "$config"
          if ! systemctl reload caddy; then
            cp "$backup" "$config"
            systemctl reload caddy || true
            rm -f "$temporary"
            exit 1
          fi
          rm -f "$temporary"
        SCRIPT
        ]
      }
    }]
  })
}

resource "aws_ssm_association" "keycloak_proxy" {
  name = aws_ssm_document.keycloak_proxy.name

  targets {
    key    = "InstanceIds"
    values = [aws_instance.chatbox.id]
  }

  depends_on = [aws_ssm_association.keycloak_server]
}
