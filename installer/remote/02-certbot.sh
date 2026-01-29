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
