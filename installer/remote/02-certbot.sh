#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
require_root

DOMAIN="${DOMAIN:-vendittoapp.com}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@vendittoapp.com}"

log "Rodando certbot (Apache) para ${DOMAIN}..."
# Deve funcionar depois que o DNS A record apontar para o IP deste servidor.
certbot --apache -d "${DOMAIN}" -m "${ADMIN_EMAIL}" --agree-tos --non-interactive || true

log "Certbot finalizado. Verifique:"
log "  sudo apache2ctl -S"
log "  sudo systemctl status apache2"


# === Webhook mTLS (Meta) ===
# If the CA file is installed and valid, enable the conf that requires client cert on /api/webhook.
META_CA="/etc/ssl/certs/meta-webhooks-root-ca.pem"
if [[ -f "${META_CA}" ]] && openssl x509 -in "${META_CA}" -noout >/dev/null 2>&1; then
  log "Habilitando mTLS do webhook (Meta) via Apache conf..."
  cp -f "${SCRIPT_DIR}/../templates/apache-webhook-mtls.conf.tpl" /etc/apache2/conf-available/penelope-webhook-mtls.conf
  a2enmod ssl >/dev/null 2>&1 || true
  a2enconf penelope-webhook-mtls >/dev/null 2>&1 || true
  apache2ctl configtest
  systemctl reload apache2
  log "mTLS do webhook habilitado: /api/webhook"
else
  if [[ -f "${META_CA}" ]]; then
    log "Meta Root CA encontrada, mas inválida (não é X509 PEM). Pulando mTLS do webhook para não derrubar o Apache."
  else
    log "Meta Root CA não encontrada (${META_CA}). Pulando mTLS do webhook."
  fi
fi
