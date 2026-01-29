#!/usr/bin/env bash
set -euo pipefail

log() { echo "### [penelope] $*"; }
die() { echo "### [penelope] ERROR: $*" >&2; exit 1; }

require_root() {
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    die "Este script precisa rodar como root (use sudo)."
  fi
}

apt_fix_broken_pgdg() {
  # Desativa listas do PGDG (apt.postgresql.org) se existirem,
  # e remove duplicatas simples, porque em distros EOL isso costuma quebrar o apt.
  local files=()
  mapfile -t files < <(grep -Rsl "apt\.postgresql\.org" /etc/apt/sources.list /etc/apt/sources.list.d 2>/dev/null || true)

  if [[ ${#files[@]} -gt 0 ]]; then
    log "Detectei repositório PGDG (apt.postgresql.org). Vou blindar contra repo quebrado/duplicado..."
    for f in "${files[@]}"; do
      # Se é um .list, desabilita com backup timestamp
      if [[ "$f" == *.list ]]; then
        local bak="${f}.disabled.$(date +%Y%m%d%H%M%S)"
        log "Desativando: $f -> $bak"
        mv "$f" "$bak" || true
      fi
    done
  fi

  # Remove duplicação do mesmo arquivo (caso pgdg.list com linhas repetidas)
  # (Se o arquivo já foi movido, não faz nada.)
  if [[ -f /etc/apt/sources.list.d/pgdg.list ]]; then
    log "Removendo linhas duplicadas em /etc/apt/sources.list.d/pgdg.list (se houver)..."
    awk '!seen[$0]++' /etc/apt/sources.list.d/pgdg.list > /etc/apt/sources.list.d/pgdg.list.tmp
    mv /etc/apt/sources.list.d/pgdg.list.tmp /etc/apt/sources.list.d/pgdg.list
  fi
}

apt_install() {
  export DEBIAN_FRONTEND=noninteractive

  apt_fix_broken_pgdg

  # Primeiro update (se falhar por repo externo, tenta de novo após fix)
  if ! apt-get update -y; then
    log "apt-get update falhou. Tentando blindagem extra e repetindo..."
    apt_fix_broken_pgdg
    apt-get update -y
  fi

  apt-get install -y --no-install-recommends "$@"
}


detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) echo "amd64";;
    aarch64|arm64) echo "arm64";;
    *) die "Arquitetura não suportada: ${arch}";;
  esac
}

random_password() {
  head -c 32 /dev/urandom | tr -dc A-Za-z0-9 || true
}

render_tpl() {
  # render_tpl <template_file> <dest_file> key=value ...
  local tpl="$1"; shift
  local dest="$1"; shift
  local tmp
  tmp="$(mktemp)"
  cp "$tpl" "$tmp"
  local kv k v
  for kv in "$@"; do
    k="${kv%%=*}"
    v="${kv#*=}"
    # escape for sed
    v="${v//\\/\\\\}"
    v="${v//&/\\&}"
    sed -i "s|{{${k}}}|${v}|g" "$tmp"
  done
  install -m 0644 "$tmp" "$dest"
  rm -f "$tmp"
}
