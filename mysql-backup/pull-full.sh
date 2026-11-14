#!/usr/bin/env bash
# Jala la foto cifrada. No tiene límite de tiempo: termina cuando el archivo
# está completo. Si la conexión deja de recibir datos, se corta y se sigue
# desde el último byte.
set -u

ENV_FILE=${MYSQL_BACKUP_ENV:-/home/ventago/homelab/mysql-backup.env}
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${VPS_HOST:?}"
: "${VPS_USER:?}"
: "${VPS_SSH_KEY:?}"
: "${BACKUP_DIR:?}"

log() { echo "$(date -u +%FT%TZ) mysql-backup-full $*"; }

# 15 s × 2: unos 30 s sin bytes y SSH se cae. Una descarga que avanza sigue.
ssh_rsh="ssh -i ${VPS_SSH_KEY} -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=2"

mkdir -p "$BACKUP_DIR/full"

while true; do
  if rsync -a --partial --append-verify \
    -e "$ssh_rsh" \
    "${VPS_USER}@${VPS_HOST}:full/" \
    "$BACKUP_DIR/full/"
  then
    break
  fi
  log "full_stalled status=$? retry"
  sleep 15
done

find "$BACKUP_DIR/full" -type f -name '*.partial' -delete
log "full_ok"
