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
# Se o arquivo de CA foi instalado, habilitamos a conf que exige client cert no /api/webhook.
if [[ -f "/etc/ssl/certs/meta-webhooks-root-ca.pem" ]]; then
  log "Habilitando mTLS do webhook (Meta) via Apache conf..."
  cp -f "${SCRIPT_DIR}/../templates/apache-webhook-mtls.conf.tpl" /etc/apache2/conf-available/penelope-webhook-mtls.conf
  a2enmod ssl >/dev/null 2>&1 || true
  a2enconf penelope-webhook-mtls >/dev/null 2>&1 || true
  apache2ctl configtest
  systemctl reload apache2
  log "mTLS do webhook habilitado: /api/webhook"
else
  log "Meta Root CA não encontrada (/etc/ssl/certs/meta-webhooks-root-ca.pem). Pulando mTLS do webhook."
fi
