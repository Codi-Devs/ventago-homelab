#!/usr/bin/env bash
# Local deploy to the Tecodigi home lab. Not GitHub Actions. Not main→live.
# SSH is only .agents/scripts/homelab_ssh.sh from the knowledge clone.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -n "${VENTAGO_KNOWLEDGE:-}" ]]; then
  BRAIN_ROOT="$(cd "$VENTAGO_KNOWLEDGE" && pwd)"
else
  BRAIN_ROOT="$(cd "$REPO_ROOT/.." && pwd)"
fi
SSH="$BRAIN_ROOT/.agents/scripts/homelab_ssh.sh"
SENSORS="$BRAIN_ROOT/.agents/scripts/homelab_sensors.sh"
REMOTE_DIR="orchestrator"

if [[ ! -x "$SSH" ]]; then
  echo "ERROR: missing $SSH" >&2
  echo "Clone the Tecodigi knowledge repo as the parent of this project, or set VENTAGO_KNOWLEDGE." >&2
  exit 2
fi

stage="$(mktemp -d "${TMPDIR:-/tmp}/ventago-homelab-XXXXXX")"
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT

rsync -a \
  --exclude '.git/' \
  --exclude '.env' \
  --exclude 'bin/' \
  --exclude 'configs/config.local.json' \
  "$REPO_ROOT/" "$stage/"

echo "==> homelab check"
"$SSH" check

echo "==> bootstrap remote_root"
"$SSH" exec -- 'mkdir -p /home/ventago/homelab/orchestrator && ls -ld /home/ventago/homelab /home/ventago/homelab/orchestrator'

echo "==> sync"
"$SSH" sync "$stage/" "$REMOTE_DIR/"

echo "==> compose up"
"$SSH" exec -- 'cd /home/ventago/homelab/orchestrator && docker compose up -d --build'

echo "==> status"
"$SSH" exec -- 'docker ps --filter name=ventago-homelab --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"'
"$SSH" exec -- 'cd /home/ventago/homelab/orchestrator && docker compose logs --tail 40'

echo "==> health"
ok=0
for i in 1 2 3 4 5 6 7 8 9 10; do
  if "$SSH" exec -- 'curl -fsS http://127.0.0.1:8080/health'; then
    ok=1
    break
  fi
  sleep 2
done
if [[ "$ok" -ne 1 ]]; then
  echo "ERROR: health check failed" >&2
  exit 1
fi

if [[ -x "$SENSORS" ]]; then
  echo "==> sensors (no inference in this bootstrap)"
  "$SENSORS" || true
fi

echo "OK: orchestrator deployed to /home/ventago/homelab/orchestrator"
echo "Rollback: $SSH exec -- 'cd /home/ventago/homelab/orchestrator && docker compose down'"
