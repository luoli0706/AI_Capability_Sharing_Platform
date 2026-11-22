#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

BACKUP_SCRIPT="${BACKUP_SCRIPT:-${PROJECT_ROOT}/scripts/backup_db.sh}"
BACKUP_DIR="${BACKUP_DIR:-/database}"
CRON_FILE="${CRON_FILE:-/etc/cron.d/ascp-db-backup}"
LOG_FILE="${LOG_FILE:-/var/log/ascp-backup.log}"
CRON_SCHEDULE="${CRON_SCHEDULE:-0 3 */7 * *}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

if [ ! -f "$BACKUP_SCRIPT" ]; then
  printf 'ERROR: Backup script not found at %s\n' "$BACKUP_SCRIPT" >&2
  exit 1
fi

if [ "$(id -u)" -ne 0 ] && [[ "$CRON_FILE" == /etc/* ]]; then
  printf 'ERROR: Installing %s requires root privileges (run with sudo).\n' "$CRON_FILE" >&2
  exit 1
fi

chmod +x "$BACKUP_SCRIPT"
mkdir -p "$BACKUP_DIR"
mkdir -p "$(dirname "$LOG_FILE")"
touch "$LOG_FILE"
chmod 0644 "$LOG_FILE"

mkdir -p "$(dirname "$CRON_FILE")"
TMP_CRON="$(mktemp)"
cat > "$TMP_CRON" <<EOF
# ASCP MySQL automated backup (every 7 days at 03:00, keeps only latest 1 backup)
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin
${CRON_SCHEDULE} root BACKUP_DIR="${BACKUP_DIR}" "${BACKUP_SCRIPT}" >> "${LOG_FILE}" 2>&1
EOF

mv -f "$TMP_CRON" "$CRON_FILE"
chmod 0644 "$CRON_FILE"

if command -v systemctl >/dev/null 2>&1; then
  if systemctl is-active --quiet cron 2>/dev/null; then
    systemctl reload cron 2>/dev/null || systemctl restart cron 2>/dev/null || true
  elif systemctl is-active --quiet crond 2>/dev/null; then
    systemctl reload crond 2>/dev/null || systemctl restart crond 2>/dev/null || true
  fi
fi

log "Installed cron job at ${CRON_FILE}:"
cat "$CRON_FILE"
log "Logs will be written to ${LOG_FILE}"
