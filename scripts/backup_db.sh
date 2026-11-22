#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ENV_FILE:-${PROJECT_ROOT}/docker/.env}"
BACKUP_DIR="${BACKUP_DIR:-/database}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-ascp-mysql}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

read_env_var() {
  local key="$1"
  local file="$2"
  [ -f "$file" ] || return 1

  local line
  line="$(grep -E "^${key}=" "$file" | tail -n 1 || true)"
  [ -n "$line" ] || return 1

  local val="${line#*=}"
  val="${val%$'\r'}"
  # Strip matching surrounding single or double quotes
  if [[ "$val" =~ ^\"(.*)\"$ ]]; then
    val="${BASH_REMATCH[1]}"
  elif [[ "$val" =~ ^\'(.*)\'$ ]]; then
    val="${BASH_REMATCH[1]}"
  fi
  printf '%s' "$val"
}

MYSQL_USER="${MYSQL_USER:-$(read_env_var MYSQL_USER "$ENV_FILE" || printf 'root')}"
MYSQL_DATABASE="${MYSQL_DATABASE:-$(read_env_var MYSQL_DATABASE "$ENV_FILE" || printf 'ascp')}"
if [ -z "${MYSQL_PASSWORD:-}" ]; then
  MYSQL_PASSWORD="$(read_env_var MYSQL_PASSWORD "$ENV_FILE" || read_env_var MYSQL_ROOT_PASSWORD "$ENV_FILE" || printf 'ascp123456')"
fi

mkdir -p "$BACKUP_DIR"

TS="$(date '+%Y%m%d_%H%M%S')"
TMP_FILE="${BACKUP_DIR}/.ascp_backup_${TS}.sql.gz.tmp"
FINAL_FILE="${BACKUP_DIR}/ascp_backup_${TS}.sql.gz"

cleanup_tmp() {
  if [ -n "${TMP_FILE:-}" ] && [ -f "$TMP_FILE" ]; then
    rm -f "$TMP_FILE"
  fi
}
trap cleanup_tmp EXIT

run_mysqldump() {
  if [ -n "${MOCK_MYSQLDUMP_CMD:-}" ]; then
    # Local test hook: allows testing backup/rotation without a running container
    eval "$MOCK_MYSQLDUMP_CMD"
  else
    docker exec -e MYSQL_PWD="$MYSQL_PASSWORD" "$MYSQL_CONTAINER" \
      mysqldump --single-transaction --routines --triggers \
      -u"$MYSQL_USER" "$MYSQL_DATABASE"
  fi
}

log "Starting MySQL backup for database '${MYSQL_DATABASE}' -> ${FINAL_FILE}"

set +e
run_mysqldump | gzip -c > "$TMP_FILE"
rc_dump="${PIPESTATUS[0]}"
rc_gzip="${PIPESTATUS[1]}"
set -e

if [ "$rc_dump" -ne 0 ] || [ "$rc_gzip" -ne 0 ]; then
  printf 'ERROR: Backup failed (mysqldump exit=%s, gzip exit=%s). Preserving existing backups.\n' \
    "$rc_dump" "$rc_gzip" >&2
  rm -f "$TMP_FILE"
  exit 1
fi

if [ ! -s "$TMP_FILE" ] || ! gzip -t "$TMP_FILE" 2>/dev/null; then
  printf 'ERROR: Backup file is empty or corrupt gzip (%s). Preserving existing backups.\n' \
    "$TMP_FILE" >&2
  rm -f "$TMP_FILE"
  exit 1
fi

if [ "$({ gzip -dc "$TMP_FILE" 2>/dev/null || true; } | head -c 1 | wc -c)" -eq 0 ]; then
  printf 'ERROR: Uncompressed SQL dump is empty (%s). Preserving existing backups.\n' \
    "$TMP_FILE" >&2
  rm -f "$TMP_FILE"
  exit 1
fi

mv -f "$TMP_FILE" "$FINAL_FILE"
BACKUP_SIZE="$(du -h "$FINAL_FILE" | awk '{print $1}')"
log "Backup succeeded: ${FINAL_FILE} (${BACKUP_SIZE})"

# Rotate: keep only the latest 1 backup in BACKUP_DIR
old_backups="$(ls -1t "${BACKUP_DIR}"/ascp_backup_*.sql.gz 2>/dev/null | tail -n +2 || true)"
if [ -n "$old_backups" ]; then
  while IFS= read -r old_file; do
    if [ -n "$old_file" ] && [ "$old_file" != "$FINAL_FILE" ]; then
      rm -f "$old_file"
      log "Removed old backup: ${old_file}"
    fi
  done <<< "$old_backups"
fi

log "Backup rotation complete. Latest backup retained: ${FINAL_FILE}"
