#!/usr/bin/env bash
# Запускает claude с токеном для http-хуков myagentflow (T2).
# Из backend/.env берётся только API_TOKEN, в файлы он не копируется.
set -euo pipefail
ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env"
token="$(sed -n 's/^API_TOKEN=//p' "$ENV_FILE" | head -n1)"
token="${token%\"}"; token="${token#\"}"; token="${token%\'}"; token="${token#\'}"
[ -n "$token" ] || { echo "API_TOKEN не найден в $ENV_FILE" >&2; exit 1; }
export MYAGENTFLOW_TOKEN="$token"
exec claude "$@"
