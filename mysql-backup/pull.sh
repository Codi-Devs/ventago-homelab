#!/usr/bin/env bash
# Jala las copias cifradas del spool del VPS. No borra lo ya guardado aquí.
set -u

ENV_FILE=${MYSQL_BACKUP_ENV:-/home/ventago/homelab/mysql-backup.env}
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${VPS_HOST:?}"
: "${VPS_USER:?}"
: "${VPS_SSH_KEY:?}"
: "${BACKUP_DIR:?}"
: "${HC_PULL_URL:?}"

log() { echo "$(date -u +%FT%TZ) mysql-backup-pull $*"; }

mkdir -p "$BACKUP_DIR/full" "$BACKUP_DIR/binlog" "$BACKUP_DIR/binlog-open" "$BACKUP_DIR/monthly"

# rrsync already confines this login to the spool out/ directory.
if ! rsync -a --partial \
  -e "ssh -i ${VPS_SSH_KEY} -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes" \
  "${VPS_USER}@${VPS_HOST}:" \
  "${BACKUP_DIR}/"
then
  log "rsync_failed"
  exit 1
fi

keep_days=${KEEP_DAYS:-35}
monthly_days=${MONTHLY_DAYS:-366}

find "$BACKUP_DIR/full" -mindepth 1 -maxdepth 1 -type d -mtime +"$keep_days" -exec rm -rf {} +
find "$BACKUP_DIR/binlog" -type f -name '*.zst.age' -mtime +"$keep_days" -delete
find "$BACKUP_DIR/monthly" -mindepth 1 -maxdepth 1 -type d -mtime +"$monthly_days" -exec rm -rf {} +
while IFS= read -r open; do
  [[ -n "$open" ]] || continue
  if [[ -f "$BACKUP_DIR/binlog/$(basename "$open")" ]]; then
    rm -f "$open"
  fi
done < <(find "$BACKUP_DIR/binlog-open" -type f -name '*.zst.age')
find "$BACKUP_DIR/binlog-open" -type f -name '*.zst.age' -mtime +"$keep_days" -delete

# El día 1 de Panamá se conserva un año, fuera de la poda de 35 días.
while IFS= read -r dir; do
  [[ -n "$dir" ]] || continue
  base=$(basename "$dir")
  day=${base:8:2}
  if [[ "$day" == "01" && -f "$dir/manifest.json" ]]; then
    month=${base:0:7}
    mkdir -p "$BACKUP_DIR/monthly/$month"
    rsync -a "$dir/" "$BACKUP_DIR/monthly/$month/"
  fi
done < <(find "$BACKUP_DIR/full" -mindepth 1 -maxdepth 1 -type d | sort)

fresh=$(find "$BACKUP_DIR/binlog" "$BACKUP_DIR/binlog-open" -type f -name '*.zst.age' -printf '%T@ %p\n' 2>/dev/null | sort -n | tail -1 | cut -d' ' -f1)
if [[ -z "$fresh" ]]; then
  log "binlog_missing"
else
  age_sec=$(( $(date +%s) - ${fresh%.*} ))
  if (( age_sec < 600 )); then
    curl -fsS -m 10 --retry 2 "$HC_PULL_URL" >/dev/null || log "healthcheck_pull_failed"
  else
    log "binlog_stale seconds=$age_sec"
  fi
fi

newest=$(find "$BACKUP_DIR/full" -mindepth 2 -maxdepth 2 -name manifest.json -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2-)
if [[ -n "$newest" ]]; then
  age_min=$(( ($(date +%s) - $(stat -c %Y "$newest")) / 60 ))
  if [[ -n "${HC_DUMP_URL:-}" && "$age_min" -lt 1560 ]]; then
    curl -fsS -m 10 --retry 2 "$HC_DUMP_URL" >/dev/null || log "healthcheck_dump_failed"
  fi
fi

log "pull_ok"
