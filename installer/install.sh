#!/usr/bin/env bash
set -euo pipefail

# installer/install.sh
# Orquestrador NO SERVIDOR, rodado dentro do repo clonado.
# Lê um JSON de config (opcional) e executa:
#   - remote/00-bootstrap.sh
#   - remote/01-deploy-api.sh
#   - remote/02-certbot.sh
#
# Uso:
#   sudo bash installer/install.sh --config /etc/penelope/installer.json
#   sudo bash installer/install.sh --all
#   sudo bash installer/install.sh --bootstrap
#   sudo bash installer/install.sh --deploy
#   sudo bash installer/install.sh --certbot
#
# Vars podem vir do JSON ou do ambiente.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REMOTE_DIR="${SCRIPT_DIR}/remote"

# shellcheck source=remote/lib.sh
source "${REMOTE_DIR}/lib.sh"

require_root

CONFIG_PATH=""
DO_BOOTSTRAP=0
DO_DEPLOY=0
DO_CERTBOT=0
SKIP_CERTBOT=0

usage() {
  sed -n '1,120p' "$0"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) CONFIG_PATH="${2:-}"; shift 2;;
    --all) DO_BOOTSTRAP=1; DO_DEPLOY=1; DO_CERTBOT=1; shift;;
    --bootstrap) DO_BOOTSTRAP=1; shift;;
    --deploy) DO_DEPLOY=1; shift;;
    --certbot) DO_CERTBOT=1; shift;;
    --skip-certbot) SKIP_CERTBOT=1; shift;;
    -h|--help) usage; exit 0;;
    *) die "Argumento desconhecido: $1";;
  esac
done

# Default: tudo
if [[ $DO_BOOTSTRAP -eq 0 && $DO_DEPLOY -eq 0 && $DO_CERTBOT -eq 0 ]]; then
  DO_BOOTSTRAP=1
  DO_DEPLOY=1
  DO_CERTBOT=1
fi

# Helpers JSON
json_get() {
  local jq_expr="$1"
  if [[ -n "${CONFIG_PATH}" && -f "${CONFIG_PATH}" ]]; then
    jq -r "${jq_expr} // empty" "${CONFIG_PATH}" 2>/dev/null || true
  else
    echo ""
  fi
}

# =========================
# Carrega config (JSON ou env)
# =========================
DOMAIN="${DOMAIN:-$(json_get '.installer.domain')}"
ADMIN_EMAIL="${ADMIN_EMAIL:-$(json_get '.installer.admin_email')}"
BRANCH="${BRANCH:-$(json_get '.installer.branch')}"
GO_VERSION="${GO_VERSION:-$(json_get '.installer.go_version')}"

DB_NAME="${DB_NAME:-$(json_get '.db.name')}"
DB_USER="${DB_USER:-$(json_get '.db.user')}"
DB_PASS="${DB_PASS:-$(json_get '.db.pass')}"

# credenciais de clone (opcionais)
GITHUB_TOKEN="${GITHUB_TOKEN:-$(json_get '.git.github_token')}"
GIT_SSH_KEY_B64="${GIT_SSH_KEY_B64:-$(json_get '.git.ssh_key_b64')}"

# Defaults seguros
DOMAIN="${DOMAIN:-vendittoapp.com}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@vendittoapp.com}"
BRANCH="${BRANCH:-main}"
GO_VERSION="${GO_VERSION:-1.22.5}"
DB_NAME="${DB_NAME:-penelope}"
DB_USER="${DB_USER:-penelope}"

# Se DB_PASS não veio, tenta reaproveitar /etc/penelope/db.pass, senão gera
DB_PASS_FILE="/etc/penelope/db.pass"
if [[ -z "${DB_PASS}" ]]; then
  if [[ -f "${DB_PASS_FILE}" ]]; then
    DB_PASS="$(cat "${DB_PASS_FILE}")"
  else
    DB_PASS="$(random_password)"
    umask 077
    mkdir -p /etc/penelope
    echo -n "${DB_PASS}" > "${DB_PASS_FILE}"
    chmod 600 "${DB_PASS_FILE}"
  fi
fi

log "Repo: ${REPO_DIR}"
log "Config: ${CONFIG_PATH:-<sem JSON>}"
log "Domain: ${DOMAIN}"
log "Branch: ${BRANCH}"
log "Go: ${GO_VERSION}"
log "DB: ${DB_NAME} / ${DB_USER} (pass: ${DB_PASS_FILE})"

# =========================
# Executa etapas
# =========================
if [[ $DO_BOOTSTRAP -eq 1 ]]; then
  log ">> Rodando bootstrap..."
  DOMAIN="${DOMAIN}" ADMIN_EMAIL="${ADMIN_EMAIL}" GO_VERSION="${GO_VERSION}" \
  DB_NAME="${DB_NAME}" DB_USER="${DB_USER}" DB_PASS="${DB_PASS}" \
  bash "${REMOTE_DIR}/00-bootstrap.sh"
fi

if [[ $DO_DEPLOY -eq 1 ]]; then
  log ">> Rodando deploy..."
  DOMAIN="${DOMAIN}" BRANCH="${BRANCH}" \
  GITHUB_TOKEN="${GITHUB_TOKEN}" GIT_SSH_KEY_B64="${GIT_SSH_KEY_B64}" \
  bash "${REMOTE_DIR}/01-deploy-api.sh"
fi

if [[ $DO_CERTBOT -eq 1 ]]; then
  if [[ $SKIP_CERTBOT -eq 1 ]]; then
    log ">> Pulando certbot (--skip-certbot)."
  else
    log ">> Rodando certbot..."
    DOMAIN="${DOMAIN}" ADMIN_EMAIL="${ADMIN_EMAIL}" \
    bash "${REMOTE_DIR}/02-certbot.sh"
  fi
fi

log "OK."
