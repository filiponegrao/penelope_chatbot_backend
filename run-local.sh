#!/usr/bin/env bash
set -euo pipefail

# Carrega variáveis locais (não versionadas)
if [ -f .env ]; then
  set -a
  source .env
  set +a
fi

# Defaults úteis
export PORT="${PORT:-8080}"
export OPENAI_MODEL="${OPENAI_MODEL:-gpt-4.1-mini}"
export POC_NO_WHATSAPP="${POC_NO_WHATSAPP:-true}"

go run .
