#!/usr/bin/env bash
# Запускает claude с токеном для http-хуков myagentflow (T2).
# Токен берётся из backend/.env (API_TOKEN), в файлы не копируется.
set -euo pipefail
ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env"
set -a; . "$ENV_FILE"; set +a
export MYAGENTFLOW_TOKEN="$API_TOKEN"
exec claude "$@"
