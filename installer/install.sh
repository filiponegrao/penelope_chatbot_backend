#!/usr/bin/env bash
set -euo pipefail

# installer/install.sh
# Rodado NO SERVIDOR, dentro do repo clonado.
#
# Fonte única: /etc/penelope/config.json (copiado pelo run.sh LOCAL).
# A partir dele, este script:
#   1) valida o JSON
#   2) gera:
#        - /etc/penelope/runtime.config.json   (config no formato esperado pelo backend Go)
#        - /etc/penelope/api.env               (EnvironmentFile do systemd; o usuário não edita)
#   3) executa:
#        - installer/remote/00-bootstrap.sh
#        - installer/remote/01-deploy-api.sh
#        - installer/remote/02-certbot.sh
#
# Uso:
#   sudo bash installer/install.sh --config /etc/penelope/config.json --all
#
# Flags:
#   --bootstrap | --deploy | --certbot | --all
#   --skip-certbot
#   --config <path>

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_DIR="${SCRIPT_DIR}/remote"
# shellcheck source=remote/lib.sh
source "${REMOTE_DIR}/lib.sh"

require_root

CONFIG_PATH="/etc/penelope/config.json"
DO_BOOTSTRAP=0
DO_DEPLOY=0
DO_CERTBOT=0
SKIP_CERTBOT=0

usage() { sed -n '1,200p' "$0"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) CONFIG_PATH="${2:-}"; shift 2;;
    --bootstrap) DO_BOOTSTRAP=1; shift;;
    --deploy) DO_DEPLOY=1; shift;;
    --certbot) DO_CERTBOT=1; shift;;
    --all) DO_BOOTSTRAP=1; DO_DEPLOY=1; DO_CERTBOT=1; shift;;
    --skip-certbot) SKIP_CERTBOT=1; shift;;
    -h|--help) usage; exit 0;;
    *) die "Argumento desconhecido: $1";;
  esac
done

if [[ $DO_BOOTSTRAP -eq 0 && $DO_DEPLOY -eq 0 && $DO_CERTBOT -eq 0 ]]; then
  DO_BOOTSTRAP=1; DO_DEPLOY=1; DO_CERTBOT=1
fi

[[ -f "${CONFIG_PATH}" ]] || die "Config JSON não encontrado em ${CONFIG_PATH}. Copie pelo run.sh local."

if ! command -v jq >/dev/null 2>&1; then
  # Máquina virgem: precisamos do jq para ler o JSON antes do bootstrap.
  apt-get update -y >/dev/null
  apt-get install -y jq >/dev/null
fi

# jq helpers
jget() { jq -r "$1 // empty" "${CONFIG_PATH}" 2>/dev/null || true; }
jget_num() { jq -r "$1 // empty" "${CONFIG_PATH}" 2>/dev/null | tr -d '[:space:]' || true; }

# escape for systemd EnvironmentFile: KEY="value"
env_escape() {
  local s="${1:-}"
  s="${s//\\/\\\\}"      # backslash
  s="${s//\"/\\\"}"      # double quote
  s="${s//$'\r'/}"       # CR
  s="${s//$'\n'/\\n}"    # NL -> \n
  printf '%s' "${s}"
}

# =========================
# Read JSON (installer)
# =========================
DOMAIN="$(jget '.installer.domain')"
ADMIN_EMAIL="$(jget '.installer.admin_email')"
BRANCH="$(jget '.installer.branch')"
GO_VERSION="$(jget '.installer.go_version')"
REPO_SSH="$(jget '.installer.repo_ssh')"
REPO_HTTPS="$(jget '.installer.repo_https')"

# =========================
# DB (for Postgres + runtime.config.json)
# =========================
DATABASE="$(jget '.db.database')"   # "postgres" | "sqlite3"
DB_HOST="$(jget '.db.host')"
DB_PORT="$(jget '.db.port')"
DB_NAME="$(jget '.db.name')"
DB_USER="$(jget '.db.user')"
DB_PASS="$(jget '.db.pass')"

# =========================
# Runtime (env)
# =========================
PORT="$(jget '.runtime.port')"
AUTOMIGRATE="$(jget '.runtime.automigrate')"

# Campos novos no JSON (como você definiu)
CHAT_HISTORY_WINDOW_MIN="$(jget '.runtime.CHAT_HISTORY_WINDOW_MIN')"
CHAT_HISTORY_MAX_EVENTS="$(jget '.runtime.CHAT_HISTORY_MAX_EVENTS')"

WHATSAPP_VERIFY_TOKEN="$(jget '.runtime.whatsapp.verify_token')"
WHATSAPP_PHONE_NUMBER_ID="$(jget '.runtime.whatsapp.phone_number_id')"
WHATSAPP_ACCESS_TOKEN="$(jget '.runtime.whatsapp.access_token')"
WHATSAPP_APP_SECRET="$(jget '.runtime.whatsapp.whatsapp_app_secret')"
WEBHOOKS_ROOT_CA_PEM="$(jget '.runtime.whatsapp.webhooks_root_ca_pem')"

OPENAI_API_KEY="$(jget '.runtime.openai.api_key')"
OPENAI_MODEL="$(jget '.runtime.openai.model')"
OPENAI_SYSTEM_PROMPT="$(jget '.runtime.openai.system_prompt')"

JWT_SECRET="$(jget '.runtime.security.jwt_secret')"
ACTIVATION_CODE_LEN="$(jget_num '.runtime.security.activation_code_len')"
REFRESH_CODE_LEN="$(jget_num '.runtime.security.refresh_code_len')"
REFRESH_MAX_DAYS="$(jget_num '.runtime.security.refresh_code_max_valid_days')"

# Defaults (non-sensitive)
DOMAIN="${DOMAIN:-vendittoapp.com}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@vendittoapp.com}"
BRANCH="${BRANCH:-main}"
GO_VERSION="${GO_VERSION:-1.23.0}"
REPO_SSH="${REPO_SSH:-git@github.com:filiponegrao/penelope_chatbot_backend.git}"
REPO_HTTPS="${REPO_HTTPS:-https://github.com/filiponegrao/penelope_chatbot_backend.git}"

DATABASE="${DATABASE:-postgres}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-penelope}"
DB_USER="${DB_USER:-penelope}"

PORT="${PORT:-5000}"
AUTOMIGRATE="${AUTOMIGRATE:-1}"
OPENAI_MODEL="${OPENAI_MODEL:-gpt-4.1-mini}"
ACTIVATION_CODE_LEN="${ACTIVATION_CODE_LEN:-6}"
REFRESH_CODE_LEN="${REFRESH_CODE_LEN:-32}"
REFRESH_MAX_DAYS="${REFRESH_MAX_DAYS:-30}"

# Required (sensitive)
[[ -n "${DB_PASS}" ]] || die "Campo obrigatório ausente: .db.pass (no JSON)"
[[ -n "${JWT_SECRET}" ]] || die "Campo obrigatório ausente: .runtime.security.jwt_secret (no JSON)"
[[ -n "${WHATSAPP_VERIFY_TOKEN}" ]] || die "Campo obrigatório ausente: .runtime.whatsapp.verify_token (no JSON)"
[[ -n "${WHATSAPP_PHONE_NUMBER_ID}" ]] || die "Campo obrigatório ausente: .runtime.whatsapp.phone_number_id (no JSON)"
[[ -n "${WHATSAPP_ACCESS_TOKEN}" ]] || die "Campo obrigatório ausente: .runtime.whatsapp.access_token (no JSON)"
[[ -n "${OPENAI_API_KEY}" ]] || die "Campo obrigatório ausente: .runtime.openai.api_key (no JSON)"

# Required (chat history) - sem fallback, do jeito que você quer
[[ -n "${CHAT_HISTORY_WINDOW_MIN}" ]] || die "Campo obrigatório ausente: .runtime.CHAT_HISTORY_WINDOW_MIN (no JSON)"
[[ "${CHAT_HISTORY_WINDOW_MIN}" =~ ^[0-9]+$ ]] || die "Campo inválido: .runtime.CHAT_HISTORY_WINDOW_MIN deve ser número (minutos)"
[[ -n "${CHAT_HISTORY_MAX_EVENTS}" ]] || die "Campo obrigatório ausente: .runtime.CHAT_HISTORY_MAX_EVENTS (no JSON)"
[[ "${CHAT_HISTORY_MAX_EVENTS}" =~ ^[0-9]+$ ]] || die "Campo inválido: .runtime.CHAT_HISTORY_MAX_EVENTS deve ser número (quantidade)"

# =========================
# Generate files in /etc/penelope
# =========================
ENV_DIR="/etc/penelope"
API_ENV="${ENV_DIR}/api.env"
RUNTIME_CONFIG="${ENV_DIR}/runtime.config.json"

APP_USER="${APP_USER:-penelope}"
APP_GROUP="${APP_GROUP:-root}"   # sem grupo "penelope"

# Garante usuário do serviço sem criar grupo novo (usa grupo root)
if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  useradd --system \
    --create-home --home-dir "/home/${APP_USER}" \
    --shell /usr/sbin/nologin \
    --gid "${APP_GROUP}" \
    "${APP_USER}"
fi

umask 077
mkdir -p "${ENV_DIR}"

# Diretório precisa ser acessível pelo usuário do serviço para ler o runtime.config.json
chown "${APP_USER}:${APP_GROUP}" "${ENV_DIR}"
chmod 750 "${ENV_DIR}"

# 1) runtime.config.json (use jq -n for proper JSON escaping)
jq -n \
  --arg api_port "${PORT}" \
  --arg log_path "/var/log/penelope/api.log" \
  --arg database "${DATABASE}" \
  --arg db_host "${DB_HOST}" \
  --arg db_port "${DB_PORT}" \
  --arg db_user "${DB_USER}" \
  --arg db_name "${DB_NAME}" \
  --arg db_pass "${DB_PASS}" \
  --arg jwt_secret "${JWT_SECRET}" \
  --argjson activation_code_len "${ACTIVATION_CODE_LEN}" \
  --argjson refresh_code_len "${REFRESH_CODE_LEN}" \
  --argjson refresh_code_max_valid_days "${REFRESH_MAX_DAYS}" \
  '{
    api_port: $api_port,
    log_path: $log_path,
    database: $database,
    db_host: $db_host,
    db_port: $db_port,
    db_user: $db_user,
    db_name: $db_name,
    db_pass: $db_pass,
    security: {
      jwt_secret: $jwt_secret,
      activation_code_len: $activation_code_len,
      refresh_code_len: $refresh_code_len,
      refresh_code_max_valid_days: $refresh_code_max_valid_days
    }
  }' > "${RUNTIME_CONFIG}"

chown "${APP_USER}:${APP_GROUP}" "${RUNTIME_CONFIG}"
chmod 640 "${RUNTIME_CONFIG}"

# 2) api.env for systemd EnvironmentFile (quote values)
{
  echo "# Gerado automaticamente por installer/install.sh"
  echo "PORT=\"$(env_escape "${PORT}")\""
  echo "CONFIG_PATH=\"$(env_escape "${RUNTIME_CONFIG}")\""
  echo "AUTOMIGRATE=\"$(env_escape "${AUTOMIGRATE}")\""
  echo "CHAT_HISTORY_WINDOW_MIN=\"$(env_escape "${CHAT_HISTORY_WINDOW_MIN}")\""
  echo "CHAT_HISTORY_MAX_EVENTS=\"$(env_escape "${CHAT_HISTORY_MAX_EVENTS}")\""
  echo ""
  echo "# WhatsApp Cloud API"
  echo "WEBHOOK_VERIFY_TOKEN=\"$(env_escape "${WHATSAPP_VERIFY_TOKEN}")\""
  echo "WHATSAPP_VERIFY_TOKEN=\"$(env_escape "${WHATSAPP_VERIFY_TOKEN}")\""
  echo "WHATSAPP_PHONE_NUMBER_ID=\"$(env_escape "${WHATSAPP_PHONE_NUMBER_ID}")\""
  echo "WHATSAPP_ACCESS_TOKEN=\"$(env_escape "${WHATSAPP_ACCESS_TOKEN}")\""
  echo "WHATSAPP_APP_SECRET=\"$(env_escape "${WHATSAPP_APP_SECRET}")\""
  echo "WEBHOOK_APP_SECRET=\"$(env_escape "${WHATSAPP_APP_SECRET}")\""
  echo ""
  echo "# OpenAI"
  echo "OPENAI_API_KEY=\"$(env_escape "${OPENAI_API_KEY}")\""
  echo "OPENAI_MODEL=\"$(env_escape "${OPENAI_MODEL}")\""
  echo "OPENAI_SYSTEM_PROMPT=\"$(env_escape "${OPENAI_SYSTEM_PROMPT}")\""
  echo ""
  echo "# Segurança"
  echo "JWT_SECRET=\"$(env_escape "${JWT_SECRET}")\""
} > "${API_ENV}"

# api.env deve ser root-only porque contém secrets
chown root:root "${API_ENV}"
chmod 600 "${API_ENV}"

# 3) Meta Webhooks mTLS Root CA (optional but recommended)
# Installed so Apache can verify Meta's client cert chain for /api/webhook.
# Prefer the bundled asset (fixed across customers), but allow an override via JSON.
META_CA_PATH="/etc/ssl/certs/meta-webhooks-root-ca.pem"
BUNDLED_META_CA="${SCRIPT_DIR}/assets/meta-webhooks-root-ca.pem"

install_meta_ca() {
  umask 077
  mkdir -p "$(dirname "${META_CA_PATH}")"
  if [[ -f "${BUNDLED_META_CA}" ]]; then
    # Bundled PEM (recommended): stable for all tenants; update only if Meta rotates the CA.
    install -m 0644 "${BUNDLED_META_CA}" "${META_CA_PATH}"
    log "Instalado Meta Webhooks Root CA (bundled) em: ${META_CA_PATH}"
    return 0
  fi

  if [[ -n "${WEBHOOKS_ROOT_CA_PEM}" ]]; then
    # JSON may store \n; convert them to real newlines.
    printf "%b" "${WEBHOOKS_ROOT_CA_PEM}" > "${META_CA_PATH}"
    chown root:root "${META_CA_PATH}"
    chmod 644 "${META_CA_PATH}"
    log "Instalado Meta Webhooks Root CA (from config) em: ${META_CA_PATH}"
    return 0
  fi

  return 1
}

if install_meta_ca; then
  # Validate PEM to avoid crashing Apache/mod_ssl.
  if openssl x509 -in "${META_CA_PATH}" -noout >/dev/null 2>&1; then
    : # OK
  else
    log "ERRO: Meta Root CA em ${META_CA_PATH} não é um X509 PEM válido. Removendo e desabilitando mTLS."
    rm -f "${META_CA_PATH}" || true
  fi
else
  log "Meta Root CA não encontrada (bundled/config vazio); mTLS do webhook não será habilitado automaticamente."
fi

log "Config carregada de: ${CONFIG_PATH}"
log "Gerados:"
log "  - ${RUNTIME_CONFIG} (lido pelo serviço: ${APP_USER})"
log "  - ${API_ENV} (root-only)"
log "Domain: ${DOMAIN} | Branch: ${BRANCH} | Go: ${GO_VERSION} | DB: ${DB_NAME}/${DB_USER}"
log "Obs: secrets não são logados."

# =========================
# Execute steps
# =========================
if [[ $DO_BOOTSTRAP -eq 1 ]]; then
  log ">> Bootstrap..."
  DOMAIN="${DOMAIN}" ADMIN_EMAIL="${ADMIN_EMAIL}" GO_VERSION="${GO_VERSION}" \
  DB_NAME="${DB_NAME}" DB_USER="${DB_USER}" DB_PASS="${DB_PASS}" \
  API_PORT="${PORT}" \
  APP_USER="${APP_USER}" APP_GROUP="${APP_GROUP}" \
  bash "${REMOTE_DIR}/00-bootstrap.sh"
fi

if [[ $DO_DEPLOY -eq 1 ]]; then
  log ">> Deploy..."
  DOMAIN="${DOMAIN}" BRANCH="${BRANCH}" REPO_SSH="${REPO_SSH}" REPO_HTTPS="${REPO_HTTPS}" \
  APP_USER="${APP_USER}" APP_GROUP="${APP_GROUP}" \
  bash "${REMOTE_DIR}/01-deploy-api.sh"
fi

if [[ $DO_CERTBOT -eq 1 ]]; then
  if [[ $SKIP_CERTBOT -eq 1 ]]; then
    log ">> Pulando certbot (--skip-certbot)."
  else
    log ">> Certbot..."
    DOMAIN="${DOMAIN}" ADMIN_EMAIL="${ADMIN_EMAIL}" \
    APP_USER="${APP_USER}" APP_GROUP="${APP_GROUP}" \
    bash "${REMOTE_DIR}/02-certbot.sh"
  fi
fi

log "OK."
