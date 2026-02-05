#!/usr/bin/env bash
set -euo pipefail

# penelope-up.sh
# Orquestrador local: copia scripts para o servidor via scp e executa via ssh.
#
# Exemplo:
#   ./penelope-up.sh --host 1.2.3.4 --domain vendittoapp.com --email admin@vendittoapp.com
#
# Se o repo pedir credenciais:
#   - Opção 1 (recomendada): export GITHUB_TOKEN=xxxx (PAT com acesso ao repo)
#   - Opção 2: export GIT_SSH_KEY_B64="$(base64 -w0 ~/.ssh/id_ed25519)"  (deploy key)
#
# Flags:
#   --host <ip/dns>        (obrigatório)
#   --user <ssh user>      (default: root)
#   --domain <domínio>     (default: vendittoapp.com)
#   --email <email>        (default: admin@vendittoapp.com)
#   --branch <branch>      (default: main)
#   --go-version <ver>     (default: 1.22.5)
#   --skip-certbot         (não roda certbot)
#
# Requer:
#   - ssh/scp local
#   - acesso ssh ao servidor

log() { echo "### [penelope-up] $*"; }
die() { echo "### [penelope-up] ERROR: $*" >&2; exit 1; }

HOST=""
USER="root"
DOMAIN="vendittoapp.com"
EMAIL="admin@vendittoapp.com"
BRANCH="main"
GO_VERSION="1.22.5"
SKIP_CERTBOT="0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) HOST="${2:-}"; shift 2;;
    --user) USER="${2:-}"; shift 2;;
    --domain) DOMAIN="${2:-}"; shift 2;;
    --email) EMAIL="${2:-}"; shift 2;;
    --branch) BRANCH="${2:-}"; shift 2;;
    --go-version) GO_VERSION="${2:-}"; shift 2;;
    --skip-certbot) SKIP_CERTBOT="1"; shift 1;;
    -h|--help) sed -n '1,120p' "$0"; exit 0;;
    *) die "Argumento desconhecido: $1";;
  esac
done

[[ -n "${HOST}" ]] || die "Faltando --host"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_DIR="/tmp/penelope-installer"

SSH_OPTS="-o StrictHostKeyChecking=accept-new"

log "Copiando scripts para ${USER}@${HOST}:${REMOTE_DIR} ..."
ssh ${SSH_OPTS} "${USER}@${HOST}" "mkdir -p '${REMOTE_DIR}'"
scp ${SSH_OPTS} -r "${ROOT_DIR}/remote" "${USER}@${HOST}:${REMOTE_DIR}/"
scp ${SSH_OPTS} -r "${ROOT_DIR}/templates" "${USER}@${HOST}:${REMOTE_DIR}/"

log "Bootstrap (instala dependências + postgres + apache + systemd templates)..."
ssh ${SSH_OPTS} "${USER}@${HOST}" \
  "sudo DOMAIN='${DOMAIN}' ADMIN_EMAIL='${EMAIL}' GO_VERSION='${GO_VERSION}' bash '${REMOTE_DIR}/remote/00-bootstrap.sh'"

log "Deploy da API (clone/build/restart)..."
# Passa credenciais opcionais via env (se você exportou localmente)
# shellcheck disable=SC2029
ssh ${SSH_OPTS} "${USER}@${HOST}" \
  "sudo DOMAIN='${DOMAIN}' BRANCH='${BRANCH}' GITHUB_TOKEN='${GITHUB_TOKEN:-}' GIT_SSH_KEY_B64='${GIT_SSH_KEY_B64:-}' bash '${REMOTE_DIR}/remote/01-deploy-api.sh'"

if [[ "${SKIP_CERTBOT}" != "1" ]]; then
  log "Certbot (Apache) - requer DNS já apontando..."
  ssh ${SSH_OPTS} "${USER}@${HOST}" \
    "sudo DOMAIN='${DOMAIN}' ADMIN_EMAIL='${EMAIL}' bash '${REMOTE_DIR}/remote/02-certbot.sh'"
else
  log "Pulando certbot (--skip-certbot)."
fi

log "OK. Teste:"
log "  https://${DOMAIN}/api  (deve responder 404/JSON dependendo do router)"
log "  https://${DOMAIN}/admin (placeholder, por enquanto)"
