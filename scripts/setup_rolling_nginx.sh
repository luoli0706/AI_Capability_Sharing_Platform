#!/usr/bin/env bash
set -euo pipefail

NGINX_SITE_CONF="${NGINX_SITE_CONF:-/etc/nginx/sites-available/ascp.overflowpaint.top.conf}"
UPSTREAM_INC="${UPSTREAM_INC:-/etc/nginx/conf.d/ascp_upstream.inc}"
INITIAL_PORT="${INITIAL_PORT:-6657}"

DRY_RUN=0

usage() {
  cat <<'EOF'
Usage: setup_rolling_nginx.sh [OPTIONS]

Migrate host Nginx site config from hardcoded 127.0.0.1:6657 to dynamic
blue-green upstream include (/etc/nginx/conf.d/ascp_upstream.inc).

Options:
  --dry-run   Print planned changes without modifying files or reloading Nginx
  -h, --help  Show this help message
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'ERROR: Unknown option: %s\n' "$1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

if [ ! -f "$NGINX_SITE_CONF" ]; then
  printf 'ERROR: Nginx site config not found: %s\n' "$NGINX_SITE_CONF" >&2
  exit 1
fi

BACKUP_FILE="${NGINX_SITE_CONF}.bak.$(date '+%Y%m%d%H%M%S')"
CREATED_UPSTREAM=0

if [ "$DRY_RUN" -eq 1 ]; then
  if [ ! -f "$UPSTREAM_INC" ]; then
    log "[dry-run] Create ${UPSTREAM_INC} pointing to 127.0.0.1:${INITIAL_PORT}"
  else
    log "[dry-run] Keep existing ${UPSTREAM_INC}"
  fi
  log "[dry-run] Backup ${NGINX_SITE_CONF} -> ${BACKUP_FILE}"
  if ! grep -qF "include ${UPSTREAM_INC};" "$NGINX_SITE_CONF"; then
    log "[dry-run] Prepend 'include ${UPSTREAM_INC};' to ${NGINX_SITE_CONF}"
  fi
  if grep -q 'http://127\.0\.0\.1:6657' "$NGINX_SITE_CONF"; then
    log "[dry-run] Replace 'http://127.0.0.1:6657' with 'http://ascp_backend_pool' in ${NGINX_SITE_CONF}"
  fi
  log "[dry-run] Run: nginx -t && nginx -s reload"
  exit 0
fi

if [ ! -f "$UPSTREAM_INC" ]; then
  log "Initializing ${UPSTREAM_INC} with 127.0.0.1:${INITIAL_PORT}..."
  mkdir -p "$(dirname "$UPSTREAM_INC")"
  tmp_inc="$(mktemp "$(dirname "$UPSTREAM_INC")/.ascp_upstream.tmp.XXXXXX")"
  cat > "$tmp_inc" <<EOF
upstream ascp_backend_pool {
    server 127.0.0.1:${INITIAL_PORT};
    keepalive 16;
}
EOF
  chmod 644 "$tmp_inc"
  mv -f "$tmp_inc" "$UPSTREAM_INC"
  CREATED_UPSTREAM=1
fi

log "Backing up ${NGINX_SITE_CONF} to ${BACKUP_FILE}..."
cp -p "$NGINX_SITE_CONF" "$BACKUP_FILE"

tmp_conf="$(mktemp "$(dirname "$NGINX_SITE_CONF")/.ascp_site.tmp.XXXXXX")"
awk -v inc="include ${UPSTREAM_INC};" '
  BEGIN { has_inc = 0 }
  $0 ~ inc { has_inc = 1 }
  {
    gsub(/http:\/\/127\.0\.0\.1:6657/, "http://ascp_backend_pool")
    lines[NR] = $0
  }
  END {
    if (!has_inc) {
      print inc
      print ""
    }
    for (i = 1; i <= NR; i++) {
      print lines[i]
    }
  }
' "$NGINX_SITE_CONF" > "$tmp_conf"

chmod 644 "$tmp_conf"
mv -f "$tmp_conf" "$NGINX_SITE_CONF"

log "Testing Nginx configuration..."
if ! nginx -t; then
  log "ERROR: nginx -t failed! Restoring ${NGINX_SITE_CONF} from ${BACKUP_FILE}..."
  mv -f "$BACKUP_FILE" "$NGINX_SITE_CONF"
  if [ "$CREATED_UPSTREAM" -eq 1 ]; then
    rm -f "$UPSTREAM_INC"
  fi
  exit 1
fi

log "Reloading Nginx..."
nginx -s reload
log "Nginx migration complete. Backup saved at ${BACKUP_FILE}."
