#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_root

DOMAIN="${DOMAIN:-vendittoapp.com}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@vendittoapp.com}"

APP_USER="${APP_USER:-penelope}"
APP_GROUP="${APP_GROUP:-root}"   # grupo root, sem grupo "penelope"
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


ensure_swap() {
  local swaps mem_mb free_mb desired_mb size_mb swap_file

  swaps="$(swapon --show --noheadings 2>/dev/null | wc -l | tr -d '[:space:]')"
  if [[ "${swaps}" != "0" ]]; then
    log "Swap: já existe swap ativa. OK."
    return 0
  fi

  mem_mb="$(awk '/MemTotal/ {printf("%d", $2/1024)}' /proc/meminfo 2>/dev/null || echo 0)"
  if [[ "${mem_mb}" -gt 2048 ]]; then
    log "Swap: RAM=${mem_mb}MB (>2048MB). Pulando criação de swap."
    return 0
  fi

  free_mb="$(df -Pm / | awk 'NR==2 {print $4}' 2>/dev/null || echo 0)"

  desired_mb=$(( mem_mb * 2 ))
  if [[ "${desired_mb}" -lt 2048 ]]; then desired_mb=2048; fi
  if [[ "${desired_mb}" -gt 4096 ]]; then desired_mb=4096; fi

  size_mb="${desired_mb}"

  if [[ "${free_mb}" -le 2048 ]]; then
    log "Swap: pouco espaço em disco (free=${free_mb}MB). Pulando criação de swap."
    return 0
  fi

  local max_by_disk=$(( free_mb - 1024 ))
  if [[ "${size_mb}" -gt "${max_by_disk}" ]]; then
    size_mb="${max_by_disk}"
  fi

  if [[ "${size_mb}" -lt 512 ]]; then
    log "Swap: size calculado muito pequeno (${size_mb}MB). Pulando."
    return 0
  fi

  swap_file="/swapfile"

  if [[ -f "${swap_file}" ]]; then
    log "Swap: /swapfile já existe. Tentando ativar..."
    chmod 600 "${swap_file}" || true
    mkswap "${swap_file}" >/dev/null 2>&1 || true
    swapon "${swap_file}" >/dev/null 2>&1 || true
    return 0
  fi

  log "Criando swapfile (${size_mb}MB) para suportar build em máquina pequena..."
  if command -v fallocate >/dev/null 2>&1; then
    fallocate -l "${size_mb}M" "${swap_file}" 2>/dev/null || true
  fi
  if [[ ! -s "${swap_file}" ]]; then
    dd if=/dev/zero of="${swap_file}" bs=1M count="${size_mb}" status=none
  fi

  chmod 600 "${swap_file}"
  mkswap "${swap_file}" >/dev/null
  swapon "${swap_file}"

  grep -q "^${swap_file} " /etc/fstab || echo "${swap_file} none swap sw 0 0" >> /etc/fstab
  sysctl -w vm.swappiness=10 >/dev/null || true

  log "Swap criada e ativada. Estado atual:"
  free -h || true
}

# 🔴 chamada obrigatória
ensure_swap

log "Instalando Go (fixo) a partir do tarball oficial..."
GO_VERSION="${GO_VERSION:-1.23.0}"
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
if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  useradd --system \
    --create-home --home-dir "${APP_HOME}" \
    --shell /usr/sbin/nologin \
    --gid "${APP_GROUP}" \
    "${APP_USER}"
fi

mkdir -p "${APP_HOME}" "${API_DIR}" "${ADMIN_DIR}" /var/log/penelope "${ENV_DIR}"
chown -R "${APP_USER}:${APP_GROUP}" "${APP_HOME}" /var/log/penelope

chmod 750 /var/log/penelope
chmod 750 "${ENV_DIR}"
chown "${APP_USER}:${APP_GROUP}" "${ENV_DIR}"

log "Configurando PostgreSQL (db/user)..."

systemctl restart postgresql
pg_isready || die "Postgres não está pronto."

if [[ -z "${DB_PASS}" ]]; then
  die "DB_PASS obrigatório."
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
  sudo -u postgres createdb -O "${DB_USER}" "${DB_NAME}"
fi

sudo -u postgres psql -X -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"

log "Apache: preparando docroot vazio (pra / não cair no Apache default)..."
mkdir -p /var/www/penelope/empty
# Se você quiser que / retorne 404, remova a linha abaixo (não crie index.html)
: > /var/www/penelope/empty/index.html

# Páginas estáticas (Política de Privacidade e Termos de Serviço)
TPL_DIR="${SCRIPT_DIR}/../templates"
if [[ -f "${TPL_DIR}/policy.html" ]]; then
  cp -f "${TPL_DIR}/policy.html" /var/www/penelope/empty/policy.html
fi
if [[ -f "${TPL_DIR}/terms.html" ]]; then
  cp -f "${TPL_DIR}/terms.html" /var/www/penelope/empty/terms.html
fi

chown -R www-data:www-data /var/www/penelope

log "Apache: habilitando módulos necessários (proxy, headers, rewrite, ssl)..."
a2enmod proxy proxy_http headers rewrite ssl >/dev/null 2>&1 || true

log "Apache: desabilitando sites default (HTTP + SSL)..."
a2dissite 000-default.conf >/dev/null 2>&1 || true
a2dissite default-ssl.conf  >/dev/null 2>&1 || true

systemctl reload apache2 || true

log "Criando VirtualHost para ${DOMAIN}..."
TPL_DIR="${SCRIPT_DIR}/../templates"
render_tpl "${TPL_DIR}/apache-vhost.conf.tpl" "/etc/apache2/sites-available/${DOMAIN}.conf" \
  DOMAIN="${DOMAIN}" \
  API_PORT="${API_PORT}" \
  ADMIN_PORT="${ADMIN_PORT}"

a2ensite "${DOMAIN}.conf" >/dev/null
apache2ctl configtest
systemctl reload apache2

log "Criando systemd services (api + admin placeholder)..."
render_tpl "${TPL_DIR}/systemd-penelope-api.service.tpl" "/etc/systemd/system/penelope-api.service" \
  APP_USER="${APP_USER}" \
  APP_GROUP="${APP_GROUP}" \
  API_DIR="${API_DIR}" \
  API_ENV="${API_ENV}" \
  GO_BIN="/usr/local/go/bin/go"

render_tpl "${TPL_DIR}/systemd-penelope-admin.service.tpl" "/etc/systemd/system/penelope-admin.service" \
  APP_USER="${APP_USER}" \
  APP_GROUP="${APP_GROUP}" \
  ADMIN_DIR="${ADMIN_DIR}" \
  ADMIN_PORT="${ADMIN_PORT}"

systemctl daemon-reload
systemctl enable penelope-api.service penelope-admin.service

log "Bootstrap concluído."
log "Próximos passos:"
log "  1) Deploy da API: rode remote/01-deploy-api.sh"
log "  2) Certbot: rode remote/02-certbot.sh"
