#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
require_root

REPO_SSH="${REPO_SSH:-git@github.com:filiponegrao/penelope_chatbot_backend.git}"
REPO_HTTPS="${REPO_HTTPS:-https://github.com/filiponegrao/penelope_chatbot_backend.git}"
BRANCH="${BRANCH:-main}"

APP_USER="${APP_USER:-penelope}"
APP_HOME="${APP_HOME:-/opt/penelope}"
API_DIR="${API_DIR:-${APP_HOME}/api}"
SRC_DIR="${API_DIR}/src"
BIN_DIR="${API_DIR}/bin"
API_BIN="${BIN_DIR}/penelope-api"

GITHUB_TOKEN="${GITHUB_TOKEN:-}"  # opcional (para clone via HTTPS sem prompt)
GIT_SSH_KEY_B64="${GIT_SSH_KEY_B64:-}"  # opcional (base64 de uma key privada com acesso ao repo)

mkdir -p "${SRC_DIR}" "${BIN_DIR}"
chown -R "${APP_USER}:${APP_USER}" "${API_DIR}"

log "Preparando método de clone (ssh key b64 / token https / ssh normal)..."
GIT_CMD=(git)
if [[ -n "${GIT_SSH_KEY_B64}" ]]; then
  log "Configurando deploy key temporária para git (via GIT_SSH_KEY_B64)..."
  SSH_DIR="${APP_HOME}/.ssh"
  mkdir -p "${SSH_DIR}"
  echo "${GIT_SSH_KEY_B64}" | base64 -d > "${SSH_DIR}/id_deploy"
  chmod 600 "${SSH_DIR}/id_deploy"
  chown -R "${APP_USER}:${APP_USER}" "${SSH_DIR}"
  # shellcheck disable=SC2016
  export GIT_SSH_COMMAND="ssh -i ${SSH_DIR}/id_deploy -o StrictHostKeyChecking=accept-new"
elif [[ -n "${GITHUB_TOKEN}" ]]; then
  log "Usando GITHUB_TOKEN para clone via HTTPS (sem prompt)..."
  REPO_URL="https://${GITHUB_TOKEN}@github.com/filiponegrao/penelope_chatbot_backend.git"
else
  REPO_URL="${REPO_SSH}"
fi

log "Clonando/atualizando repo em ${SRC_DIR}..."
if [[ ! -d "${SRC_DIR}/.git" ]]; then
  sudo -u "${APP_USER}" bash -lc "git clone '${REPO_URL}' '${SRC_DIR}'"
fi

pushd "${SRC_DIR}" >/dev/null
sudo -u "${APP_USER}" bash -lc "cd '${SRC_DIR}' && git fetch --prune"
sudo -u "${APP_USER}" bash -lc "cd '${SRC_DIR}' && git checkout '${BRANCH}'"
sudo -u "${APP_USER}" bash -lc "cd '${SRC_DIR}' && git pull --ff-only origin '${BRANCH}'"

log "Buildando binário..."
sudo -u "${APP_USER}" bash -lc "cd '${SRC_DIR}' && /usr/local/go/bin/go mod tidy"
sudo -u "${APP_USER}" bash -lc "cd '${SRC_DIR}' && /usr/local/go/bin/go build -o '${API_BIN}' ."

log "Atualizando ExecStart do systemd (para rodar binário em vez de go run)..."
# Ajusta o service para ExecStart apontar para o binário.
# (O template instala por padrão 'go run', mas produção é melhor binário.)
SERVICE_FILE="/etc/systemd/system/penelope-api.service"
if grep -q "ExecStart=.*go run" "${SERVICE_FILE}"; then
  sed -i "s|ExecStart=.*|ExecStart=${API_BIN}|g" "${SERVICE_FILE}"
fi

systemctl daemon-reload
systemctl restart penelope-api.service
systemctl --no-pager --full status penelope-api.service || true

log "Admin (placeholder) em ${APP_HOME}/admin."
systemctl restart penelope-admin.service
systemctl --no-pager --full status penelope-admin.service || true

log "Deploy OK."
