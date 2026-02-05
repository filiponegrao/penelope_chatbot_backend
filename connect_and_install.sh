#!/usr/bin/env bash
set -euo pipefail

# connect_and_install.sh
# Rodar LOCALMENTE (na sua máquina). Ele:
#  1) envia o config.install.json para o servidor (/etc/penelope/config.json)
#  2) clona/atualiza o repo no servidor
#  3) executa installer/install.sh no servidor (gera runtime.config.json + api.env e instala tudo)

# ===== Pré-requisitos locais =====
command -v jq >/dev/null 2>&1 || { echo "Erro: jq não encontrado (instale via brew/apt)."; exit 1; }

# garante ssh-agent carregado (pra não pedir senha toda hora)
if ! ssh-add -l >/dev/null 2>&1; then
  eval "$(ssh-agent -s)" >/dev/null
  ssh-add ~/.ssh/id_rsa
fi

CONFIG="./config.install.json"
[[ -f "${CONFIG}" ]] || { echo "Erro: arquivo ${CONFIG} não encontrado."; exit 1; }

HOST="$(jq -r '.installer.domain' "${CONFIG}")"
[[ -n "${HOST}" && "${HOST}" != "null" ]] || { echo "Erro: .installer.domain está vazio no ${CONFIG}"; exit 1; }

REMOTE_CONFIG="/etc/penelope/config.json"
REMOTE_DIR="/opt/penelope/app"

echo ">> Enviando config para ${HOST}..."
scp "${CONFIG}" "root@${HOST}:/tmp/penelope-config.json"
ssh "root@${HOST}" "mkdir -p /etc/penelope && mv /tmp/penelope-config.json '${REMOTE_CONFIG}' && chmod 600 '${REMOTE_CONFIG}'"

echo ">> Clonando/atualizando repo no servidor..."
ssh "root@${HOST}" "
  set -euo pipefail
  mkdir -p /opt/penelope

  # Se o diretório já existe e não é do root, rodar Git como o dono do diretório
  # para evitar erro de "dubious ownership".
  REPO_OWNER=root
  if [ -d '${REMOTE_DIR}' ]; then
    REPO_OWNER=\$(stat -c '%U' '${REMOTE_DIR}' 2>/dev/null || echo root)
  fi

  git_as_owner() {
    if [ "\$REPO_OWNER" = "root" ]; then
      "\$@"
    else
      sudo -H -u "\$REPO_OWNER" "\$@"
    fi
  }

  if [ ! -d '${REMOTE_DIR}/.git' ]; then
    REPO_HTTPS=\$(jq -r '.installer.repo_https' '${REMOTE_CONFIG}')
    [ -n "\$REPO_HTTPS" ] && [ "\$REPO_HTTPS" != "null" ] || { echo 'Erro: .installer.repo_https vazio no JSON'; exit 1; }
    # clone como root, depois o usuário do serviço assume no install/bootstrap.
    git clone "\$REPO_HTTPS" '${REMOTE_DIR}'
  fi

  cd '${REMOTE_DIR}'
  # Tornar o repo seguro para o usuário atual (caso precise rodar como root)
  git config --global --add safe.directory '${REMOTE_DIR}' >/dev/null 2>&1 || true

  BRANCH=\$(jq -r '.installer.branch' '${REMOTE_CONFIG}')
  [ -n "\$BRANCH" ] && [ "\$BRANCH" != "null" ] || { echo 'Erro: .installer.branch vazio no JSON'; exit 1; }

  # Atualização idempotente e tolerante a force-push:
  git_as_owner git -C '${REMOTE_DIR}' fetch origin "\$BRANCH" --prune
  git_as_owner git -C '${REMOTE_DIR}' checkout "\$BRANCH" 2>/dev/null || git_as_owner git -C '${REMOTE_DIR}' checkout -b "\$BRANCH" "origin/\$BRANCH"
  git_as_owner git -C '${REMOTE_DIR}' reset --hard "origin/\$BRANCH"
  git_as_owner git -C '${REMOTE_DIR}' clean -fd
"

echo ">> Enviando installer atualizado (sem precisar dar push no GitHub)..."
# A instalação roda scripts em ${REMOTE_DIR}/installer. Como esse repo é clonado
# do GitHub (e você pode estar testando sem commitar/push), enviamos a pasta
# installer local para o servidor e sobrescrevemos apenas essa pasta.
TMP_INSTALLER_TGZ="/tmp/penelope-installer.tgz"
COPYFILE_DISABLE=1 tar --no-xattrs --exclude="._*" -czf "${TMP_INSTALLER_TGZ}" installer
scp "${TMP_INSTALLER_TGZ}" "root@${HOST}:${TMP_INSTALLER_TGZ}"
rm -f "${TMP_INSTALLER_TGZ}"

ssh "root@${HOST}" "
  set -euo pipefail
  cd '${REMOTE_DIR}'
  rm -rf installer
  tar -xzf '${TMP_INSTALLER_TGZ}' -C '${REMOTE_DIR}'
  rm -f '${TMP_INSTALLER_TGZ}'
"

echo ">> Instalando (bootstrap + deploy + certbot)..."
ssh "root@${HOST}" "bash '${REMOTE_DIR}/installer/install.sh' --config '${REMOTE_CONFIG}' --all"

echo ">> OK"
