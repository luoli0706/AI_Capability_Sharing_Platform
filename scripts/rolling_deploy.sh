#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

ENV_FILE="${ENV_FILE:-${PROJECT_ROOT}/docker/.env}"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/docker/docker-compose.rolling.yml}"
UPSTREAM_INC="${UPSTREAM_INC:-/etc/nginx/conf.d/ascp_upstream.inc}"

BLUE_PORT="${BLUE_PORT:-16657}"
GREEN_PORT="${GREEN_PORT:-16658}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-60}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-2}"
DRAIN_SECONDS="${DRAIN_SECONDS:-5}"

DRY_RUN=0
SKIP_BUILD="${SKIP_BUILD:-0}"
LOCK_FILE="${LOCK_FILE:-/tmp/ascp-rolling-deploy.lock}"

usage() {
  cat <<'EOF'
Usage: rolling_deploy.sh [OPTIONS]

Zero-downtime blue-green rolling deployment for ASCP single-host Docker setup.

Options:
  --dry-run       Print resolved slots and commands without executing changes
  --skip-build    Skip 'docker compose build' and reuse existing local images
  -h, --help      Show this help message
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    --skip-build)
      SKIP_BUILD=1
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

read_env_var() {
  local key="$1"
  local file="$2"
  local line val=""
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line%$'\r'}"
    case "$line" in
      ''|\#*) continue ;;
      export\ *) line="${line#export }" ;;
    esac
    case "$line" in
      "${key}="*)
        val="${line#*=}"
        if [[ "$val" =~ ^\"(.*)\"$ ]]; then
          val="${BASH_REMATCH[1]}"
        elif [[ "$val" =~ ^\'(.*)\'$ ]]; then
          val="${BASH_REMATCH[1]}"
        fi
        ;;
    esac
  done < "$file"
  printf '%s' "$val"
}

validate_env_file() {
  local env_file="$1"
  local mysql_root_password jwt_secret

  mysql_root_password="$(read_env_var "MYSQL_ROOT_PASSWORD" "$env_file")"
  jwt_secret="$(read_env_var "JWT_SECRET" "$env_file")"

  if [ -z "$mysql_root_password" ] || [ "$mysql_root_password" = "ascp123456" ]; then
    printf 'ERROR: MYSQL_ROOT_PASSWORD in %s must be non-empty and must not use the weak default from .env.example\n' "$env_file" >&2
    return 1
  fi

  if [ -z "$jwt_secret" ] || [ "$jwt_secret" = "change-me-in-production-ascp-jwt-secret" ]; then
    printf 'ERROR: JWT_SECRET in %s must be non-empty and must not use the weak default from .env.example\n' "$env_file" >&2
    return 1
  fi
}

detect_active_slot() {
  if [ -f "$UPSTREAM_INC" ]; then
    if grep -qE "127\.0\.0\.1:${GREEN_PORT}([[:space:]]|;|$)" "$UPSTREAM_INC"; then
      CUR_SLOT="green"
      CUR_PORT="$GREEN_PORT"
      return 0
    fi
    if grep -qE "127\.0\.0\.1:${BLUE_PORT}([[:space:]]|;|$)" "$UPSTREAM_INC"; then
      CUR_SLOT="blue"
      CUR_PORT="$BLUE_PORT"
      return 0
    fi
  fi

  # Default to blue (16657) when migrating from legacy single-slot (6657) or uninitialized
  CUR_SLOT="blue"
  CUR_PORT="$BLUE_PORT"
}

check_health() {
  local url="http://127.0.0.1:${NEXT_PORT}/api/v1/healthz"
  local deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
  local attempt=1

  while [ "$(date +%s)" -le "$deadline" ]; do
    local tmp_body
    tmp_body="$(mktemp)"
    local code
    code=$(curl -sS -m 5 -o "$tmp_body" -w '%{http_code}' "$url" 2>/dev/null || echo "000")
    local body
    body="$(cat "$tmp_body" 2>/dev/null || true)"
    rm -f "$tmp_body"

    if [ "$code" = "200" ] && printf '%s' "$body" | grep -qE '"status"[[:space:]]*:[[:space:]]*"ok"'; then
      log "Healthcheck passed on attempt ${attempt}: HTTP ${code}"
      return 0
    fi

    log "Waiting for standby healthcheck (${attempt}): HTTP ${code}"
    sleep "$HEALTH_INTERVAL"
    attempt=$(( attempt + 1 ))
  done

  return 1
}

write_upstream_inc() {
  local port="$1"
  local target_dir
  target_dir="$(dirname "$UPSTREAM_INC")"
  mkdir -p "$target_dir"

  local backup_file=""
  if [ -f "$UPSTREAM_INC" ]; then
    backup_file="$(mktemp "${target_dir}/.ascp_upstream.bak.XXXXXX")"
    cp -p "$UPSTREAM_INC" "$backup_file"
  fi

  local tmp_file
  tmp_file="$(mktemp "${target_dir}/.ascp_upstream.tmp.XXXXXX")"
  cat > "$tmp_file" <<EOF
upstream ascp_backend_pool {
    server 127.0.0.1:${port};
}
EOF
  chmod 644 "$tmp_file"
  mv -f "$tmp_file" "$UPSTREAM_INC"

  if ! nginx -t; then
    log "ERROR: nginx -t failed after updating ${UPSTREAM_INC}; restoring previous config..."
    if [ -n "$backup_file" ] && [ -f "$backup_file" ]; then
      mv -f "$backup_file" "$UPSTREAM_INC"
    else
      rm -f "$UPSTREAM_INC"
    fi
    return 1
  fi

  if ! nginx -s reload; then
    log "ERROR: nginx -s reload failed after updating ${UPSTREAM_INC}; restoring previous config..."
    if [ -n "$backup_file" ] && [ -f "$backup_file" ]; then
      mv -f "$backup_file" "$UPSTREAM_INC"
    else
      rm -f "$UPSTREAM_INC"
    fi
    nginx -t >/dev/null 2>&1 && nginx -s reload >/dev/null 2>&1 || true
    return 1
  fi

  if [ -n "$backup_file" ]; then
    rm -f "$backup_file"
  fi
}

cleanup_old_slot() {
  if ASCP_SLOT="$CUR_SLOT" ASCP_PORT="$CUR_PORT" \
    docker compose -p "ascp-${CUR_SLOT}" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps -q 2>/dev/null | grep -q .; then
    log "Stopping and removing old slot (ascp-${CUR_SLOT})..."
    ASCP_SLOT="$CUR_SLOT" ASCP_PORT="$CUR_PORT" \
      docker compose -p "ascp-${CUR_SLOT}" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down --remove-orphans
  else
    log "No running containers found for previous slot ascp-${CUR_SLOT}."
  fi

  local legacy
  for legacy in ascp-nginx ascp-backend; do
    if docker ps -a --format '{{.Names}}' | grep -qx "$legacy"; then
      log "Removing legacy container: ${legacy}"
      docker rm -f "$legacy"
    fi
  done
}

if [ "$DRY_RUN" -eq 0 ]; then
  exec 9>"$LOCK_FILE"
  if ! flock -n 9; then
    printf 'ERROR: Another rolling deployment is already in progress (lock: %s)\n' "$LOCK_FILE" >&2
    exit 1
  fi
fi

if [ ! -f "$COMPOSE_FILE" ]; then
  printf 'ERROR: Compose file not found: %s\n' "$COMPOSE_FILE" >&2
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    log "WARN: Env file not found (${ENV_FILE}); continuing in --dry-run mode."
  else
    printf 'ERROR: Env file not found: %s (create from docker/.env.example)\n' "$ENV_FILE" >&2
    exit 1
  fi
else
  validate_env_file "$ENV_FILE"
fi

detect_active_slot
if [ "$CUR_SLOT" = "blue" ]; then
  NEXT_SLOT="green"
  NEXT_PORT="$GREEN_PORT"
else
  NEXT_SLOT="blue"
  NEXT_PORT="$BLUE_PORT"
fi

log "Active slot: ${CUR_SLOT} (127.0.0.1:${CUR_PORT})"
log "Standby slot: ${NEXT_SLOT} (127.0.0.1:${NEXT_PORT})"

if [ "$DRY_RUN" -eq 1 ]; then
  log "[dry-run] Ensure external volume docker_backend_uploads exists"
  if [ "$SKIP_BUILD" != "1" ]; then
    log "[dry-run] docker compose --env-file \"${ENV_FILE}\" -f \"${COMPOSE_FILE}\" build"
  fi
  log "[dry-run] ASCP_SLOT=\"${NEXT_SLOT}\" ASCP_PORT=\"${NEXT_PORT}\" docker compose -p \"ascp-${NEXT_SLOT}\" --env-file \"${ENV_FILE}\" -f \"${COMPOSE_FILE}\" up -d --no-build --force-recreate"
  log "[dry-run] Poll http://127.0.0.1:${NEXT_PORT}/api/v1/healthz (up to ${HEALTH_TIMEOUT}s) for HTTP 200 and \"status\":\"ok\""
  log "[dry-run] Atomically write ${UPSTREAM_INC} -> server 127.0.0.1:${NEXT_PORT};"
  log "[dry-run] nginx -t && nginx -s reload"
  log "[dry-run] Sleep ${DRAIN_SECONDS}s to drain in-flight requests"
  log "[dry-run] Stop & remove old slot ascp-${CUR_SLOT} and any legacy ascp-nginx/ascp-backend containers"
  exit 0
fi

if ! docker volume inspect docker_backend_uploads >/dev/null 2>&1; then
  log "Creating external volume docker_backend_uploads..."
  docker volume create docker_backend_uploads >/dev/null
fi

if [ "$SKIP_BUILD" != "1" ]; then
  log "Building local images (ascp-backend:local, ascp-frontend:local)..."
  ASCP_SLOT="$NEXT_SLOT" ASCP_PORT="$NEXT_PORT" \
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" build
fi

log "Starting standby slot ascp-${NEXT_SLOT} on 127.0.0.1:${NEXT_PORT}..."
ASCP_SLOT="$NEXT_SLOT" ASCP_PORT="$NEXT_PORT" \
  docker compose -p "ascp-${NEXT_SLOT}" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --no-build --force-recreate

if ! check_health; then
  log "ERROR: Standby slot ascp-${NEXT_SLOT} failed healthcheck within ${HEALTH_TIMEOUT}s; rolling back standby..."
  ASCP_SLOT="$NEXT_SLOT" ASCP_PORT="$NEXT_PORT" \
    docker compose -p "ascp-${NEXT_SLOT}" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down --remove-orphans || true
  exit 1
fi

log "Switching Nginx upstream ${UPSTREAM_INC} to 127.0.0.1:${NEXT_PORT}..."
if ! write_upstream_inc "$NEXT_PORT"; then
  log "ERROR: Failed to reload Nginx with new upstream; tearing down standby slot ascp-${NEXT_SLOT}..."
  ASCP_SLOT="$NEXT_SLOT" ASCP_PORT="$NEXT_PORT" \
    docker compose -p "ascp-${NEXT_SLOT}" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down --remove-orphans || true
  exit 1
fi

log "Waiting ${DRAIN_SECONDS}s for in-flight requests to drain from ${CUR_SLOT}..."
sleep "$DRAIN_SECONDS"

cleanup_old_slot
log "Rolling deployment complete! Active slot is now ${NEXT_SLOT} (127.0.0.1:${NEXT_PORT})."
