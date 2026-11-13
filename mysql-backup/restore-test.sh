#!/usr/bin/env bash
# Restaura la última foto y el diario posterior en un MySQL temporal.
# Borra el temporal al terminar. No publica puertos y no toca las copias.
set -u -o pipefail

ENV_FILE=${MYSQL_BACKUP_ENV:-/home/ventago/homelab/mysql-backup.env}
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${BACKUP_DIR:?}"
: "${AGE_IDENTITY:?}"
: "${HC_RESTORE_URL:?}"

NAME=mysql-backup-restore-test
WORKDIR=$(mktemp -d /tmp/mysql-backup-restore.XXXXXX)
ROOTPW=$(openssl rand -hex 16)
started=$(date +%s)

log() { echo "$(date -u +%FT%TZ) mysql-backup-restore $*"; }

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

fail() {
  log "restore_failed reason=$1"
  exit 1
}

mysql_exec() {
  docker exec -e MYSQL_PWD="$ROOTPW" "$NAME" mysql -uroot "$@"
}

latest=$(find "$BACKUP_DIR/full" -mindepth 2 -maxdepth 2 -name manifest.json | sort | tail -1)
[[ -n "$latest" ]] || fail "no_manifest"
dump_dir=$(dirname "$latest")
[[ -f "$dump_dir/all.sql.zst.age" ]] || fail "no_dump"

docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" \
  -e MYSQL_ROOT_PASSWORD="$ROOTPW" \
  mysql:8.0.43 \
  --innodb-buffer-pool-size=268435456 >/dev/null || fail "docker_run"

ready=0
for _ in $(seq 1 90); do
  if docker exec -e MYSQL_PWD="$ROOTPW" "$NAME" mysql -uroot -h 127.0.0.1 --protocol=tcp -N -e 'SELECT 1' >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
[[ "$ready" -eq 1 ]] || fail "mysql_not_ready"

pos_line=$(age -d -i "$AGE_IDENTITY" "$dump_dir/all.sql.zst.age" | zstd -d | awk 'NR<=100 && /CHANGE (MASTER|REPLICATION SOURCE) TO/{print; exit}')
[[ -n "$pos_line" ]] || fail "no_binlog_position"
log_file=$(printf '%s\n' "$pos_line" | sed -n "s/.*\\(SOURCE_LOG_FILE\\|MASTER_LOG_FILE\\)='\\([^']*\\)'.*/\\2/p" | head -1)
log_pos=$(printf '%s\n' "$pos_line" | sed -n "s/.*\\(SOURCE_LOG_POS\\|MASTER_LOG_POS\\)=\\([0-9][0-9]*\\).*/\\2/p" | head -1)
[[ -n "$log_file" && -n "$log_pos" ]] || fail "bad_binlog_position"

age -d -i "$AGE_IDENTITY" "$dump_dir/all.sql.zst.age" | zstd -d \
  | docker exec -i -e MYSQL_PWD="$ROOTPW" "$NAME" mysql -uroot || fail "load_dump"

declare -A seen=()
binlog_args=()
started_file=0
while IFS= read -r enc; do
  [[ -n "$enc" ]] || continue
  base=$(basename "$enc")
  base=${base%.zst.age}
  if [[ -n "${seen[$base]:-}" ]]; then
    continue
  fi
  seen[$base]=1
  if [[ "$started_file" -eq 0 && "$base" < "$log_file" ]]; then
    continue
  fi
  plain="$WORKDIR/$base"
  age -d -i "$AGE_IDENTITY" "$enc" | zstd -d > "$plain" || fail "decrypt_binlog"
  if [[ "$base" == "$log_file" ]]; then
    binlog_args+=(--start-position="$log_pos" "$plain")
    started_file=1
  elif [[ "$started_file" -eq 1 ]]; then
    binlog_args+=("$plain")
  fi
done < <(find "$BACKUP_DIR/binlog" -type f -name 'binlog.*.zst.age' | sort; find "$BACKUP_DIR/binlog-open" -type f -name 'binlog.*.zst.age' | sort)

if [[ ${#binlog_args[@]} -gt 0 ]]; then
  docker run --rm -v "$WORKDIR:/work:ro" mysql:8.0.43 \
    mysqlbinlog "${binlog_args[@]/$WORKDIR//work}" \
    | docker exec -i -e MYSQL_PWD="$ROOTPW" "$NAME" mysql -uroot \
    || fail "binlog_apply"
fi

mysql_exec -N -e "SELECT table_schema, COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('mysql','sys','information_schema','performance_schema') GROUP BY table_schema" \
  > "$WORKDIR/counts.tsv" || fail "count_tables"

python3 - "$latest" "$WORKDIR/counts.tsv" <<'PY'
import json, sys
manifest = json.load(open(sys.argv[1]))
expected = {k: int(v) for k, v in (manifest.get("tables") or {}).items()}
got = {}
for line in open(sys.argv[2]):
    line = line.rstrip("\n")
    if not line:
        continue
    schema, count = line.split("\t")
    got[schema] = int(count)
missing = sorted(set(expected) - set(got))
mismatch = sorted(k for k in expected if k in got and got[k] != expected[k])
if missing or mismatch:
    print(f"schema_mismatch missing={missing} mismatch={mismatch}")
    sys.exit(1)
print(f"schemas={len(got)}")
PY
[[ $? -eq 0 ]] || fail "schema_mismatch"

elapsed=$(( $(date +%s) - started ))
log "restore_ok seconds=$elapsed"
curl -fsS -m 10 --retry 2 "$HC_RESTORE_URL" >/dev/null || log "healthcheck_restore_failed"
exit 0
