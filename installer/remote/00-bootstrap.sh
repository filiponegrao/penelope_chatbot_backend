#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_root

DOMAIN="${DOMAIN:-vendittoapp.com}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@vendittoapp.com}"

APP_USER="${APP_USER:-penelope}"
APP_HOME="${APP_HOME:-/opt/penelope}"
API_DIR="${API_DIR:-${APP_HOME}/api}"
ADMIN_DIR="${ADMIN_DIR:-${APP_HOME}/admin}"
ENV_DIR="/etc/penelope"
API_ENV="${ENV_DIR}/api.env"
API_CONFIG="${ENV_DIR}/config.json"

API_PORT="${API_PORT:-5000}"
ADMIN_PORT="${ADMIN_PORT:-8888}"

DB_NAME="${DB_NAME:-penelope}"
DB_USER="${DB_USER:-penelope}"
DB_PASS="${DB_PASS:-}"

# Pacotes
log "Instalando dependências (Apache, Postgres, Certbot, build tools)..."
apt_install ca-certificates curl gnupg lsb-release git unzip jq \
  apache2 \
  postgresql postgresql-contrib \
  certbot python3-certbot-apache \
  build-essential

log "Instalando Go (fixo) a partir do tarball oficial..."
GO_VERSION="${GO_VERSION:-1.22.5}"
ARCH="$(detect_arch)"
GO_TGZ="go${GO_VERSION}.linux-${ARCH}.tar.gz"
GO_URL="https://go.dev/dl/${GO_TGZ}"

if ! command -v go >/dev/null 2>&1 || [[ "$(go version 2>/dev/null || true)" != *"go${GO_VERSION}"* ]]; then
  rm -rf /usr/local/go
  curl -fsSL "${GO_URL}" -o "/tmp/${GO_TGZ}"
  tar -C /usr/local -xzf "/tmp/${GO_TGZ}"
  rm -f "/tmp/${GO_TGZ}"
fi

cat > /etc/profile.d/go.sh <<'EOF'
export PATH="/usr/local/go/bin:$PATH"
EOF

export PATH="/usr/local/go/bin:$PATH"
go version

log "Criando usuário e pastas..."
id -u "${APP_USER}" >/dev/null 2>&1 || useradd --system --home "${APP_HOME}" --shell /usr/sbin/nologin "${APP_USER}"
mkdir -p "${APP_HOME}" "${API_DIR}" "${ADMIN_DIR}" /var/log/penelope "${ENV_DIR}"
chown -R "${APP_USER}:${APP_USER}" "${APP_HOME}" /var/log/penelope
chmod 750 /var/log/penelope
chmod 750 "${ENV_DIR}"

log "Configurando PostgreSQL (db/user)..."

systemctl restart postgresql
pg_isready || die "Postgres não está pronto."

if [[ -z "${DB_PASS}" ]]; then
  die "DB_PASS obrigatório (modo all-in). Ex: export DB_PASS='...' ou passar via env no penelope-up.sh"
fi

export PSQLRC=/dev/null
export PAGER=cat

log "Criando role (se não existir)..."
sudo -u postgres psql -X -v ON_ERROR_STOP=1 <<EOF
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${DB_USER}') THEN
    CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASS}';
  END IF;
END
\$\$;
EOF

log "Checando database..."
DB_EXISTS="$(sudo -u postgres psql -X -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" || true | tr -d '[:space:]')"
if [[ "${DB_EXISTS}" != "1" ]]; then
  log "Database não existe. Criando..."
  sudo -u postgres createdb -O "${DB_USER}" "${DB_NAME}"
else
  log "Database já existe. OK."
fi

log "Garantindo privilégios..."
sudo -u postgres psql -X -v ON_ERROR_STOP=1 -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"

log "Postgres OK."

log "Criando database se não existir..."
if ! sudo -u postgres psql -X -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
  sudo -u postgres createdb -O "${DB_USER}" "${DB_NAME}"
fi

log "Garantindo privilégios..."
sudo -u postgres psql -X -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"

log "Criando env/config em /etc/penelope (não sobrescreve se já existir)..."
if [[ ! -f "${API_ENV}" ]]; then
  cat > "${API_ENV}" <<EOF
# Penelope API runtime env
PORT=${API_PORT}
CONFIG_PATH=${API_CONFIG}
AUTOMIGRATE=1

# WhatsApp
WHATSAPP_VERIFY_TOKEN=CHANGEME
WHATSAPP_PHONE_NUMBER_ID=CHANGEME
WHATSAPP_ACCESS_TOKEN=CHANGEME

# OpenAI
OPENAI_API_KEY=CHANGEME
OPENAI_MODEL=gpt-4.1-mini

# Segurança
JWT_SECRET=CHANGEME
EOF
  chown root:"${APP_USER}" "${API_ENV}"
  chmod 640 "${API_ENV}"
fi

if [[ ! -f "${API_CONFIG}" ]]; then
  cat > "${API_CONFIG}" <<EOF
{
  "api_port": "${API_PORT}",
  "log_path": "/var/log/penelope/api.log",
  "database": "postgres",
  "db_host": "127.0.0.1",
  "db_port": "5432",
  "db_user": "${DB_USER}",
  "db_name": "${DB_NAME}",
  "db_pass": "${DB_PASS}",
  "security": {
    "jwt_secret": "CHANGE_ME",
    "activation_code_len": 6,
    "refresh_code_len": 32,
    "refresh_code_max_valid_days": 30
  }
}
EOF
  chown root:"${APP_USER}" "${API_CONFIG}"
  chmod 640 "${API_CONFIG}"
fi

log "Apache: habilitando módulos necessários (proxy, headers, rewrite, ssl)..."
a2enmod proxy proxy_http headers rewrite ssl
a2dissite 000-default.conf >/dev/null 2>&1 || true

log "Criando VirtualHost para ${DOMAIN}..."
TPL_DIR="${SCRIPT_DIR}/../templates"
render_tpl "${TPL_DIR}/apache-vhost.conf.tpl" "/etc/apache2/sites-available/${DOMAIN}.conf" \
  DOMAIN="${DOMAIN}" \
  API_PORT="${API_PORT}" \
  ADMIN_PORT="${ADMIN_PORT}"

a2ensite "${DOMAIN}.conf"
apache2ctl configtest
systemctl reload apache2

log "Criando systemd services (api + admin placeholder)..."
render_tpl "${TPL_DIR}/systemd-penelope-api.service.tpl" "/etc/systemd/system/penelope-api.service" \
  APP_USER="${APP_USER}" \
  API_DIR="${API_DIR}" \
  API_ENV="${API_ENV}" \
  GO_BIN="/usr/local/go/bin/go"

render_tpl "${TPL_DIR}/systemd-penelope-admin.service.tpl" "/etc/systemd/system/penelope-admin.service" \
  APP_USER="${APP_USER}" \
  ADMIN_DIR="${ADMIN_DIR}" \
  ADMIN_PORT="${ADMIN_PORT}"

systemctl daemon-reload
systemctl enable penelope-api.service penelope-admin.service

log "Bootstrap concluído."
log "Próximos passos:"
log "  1) Deploy da API: rode remote/01-deploy-api.sh (ou pelo script local penelope-up.sh)"
log "  2) Certbot: rode remote/02-certbot.sh quando o DNS já estiver apontando"
