#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/www/ASCP}"
COMPOSE_FILE="${COMPOSE_FILE:-docker/docker-compose.prod.yml}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:6657/api/v1/healthz}"
SERVICE_NAME="${SERVICE_NAME:-ascp}"
BRANCH="${BRANCH:-master}"
HEALTH_RETRIES="${HEALTH_RETRIES:-20}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-3}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

check_health() {
  local attempt=1
  while [ "$attempt" -le "$HEALTH_RETRIES" ]; do
    local code
    code=$(curl -s -o /dev/null -w '%{http_code}' "$HEALTH_URL" || true)
    if [ "$code" = "200" ]; then
      log "Healthcheck passed (HTTP 200) on attempt $attempt/$HEALTH_RETRIES"
      return 0
    fi
    log "Waiting for healthcheck ($attempt/$HEALTH_RETRIES): HTTP $code"
    sleep "$HEALTH_INTERVAL"
    attempt=$((attempt + 1))
  done
  return 1
}

cd "$APP_DIR"

PREV_COMMIT=""
if git rev-parse --verify HEAD >/dev/null 2>&1; then
  PREV_COMMIT=$(git rev-parse HEAD)
  log "Current commit: $PREV_COMMIT"
fi

if [ "${SKIP_GIT_PULL:-0}" != "1" ]; then
  log "Pulling latest changes from origin/$BRANCH..."
  git fetch origin "$BRANCH"
  git checkout "$BRANCH"
  git pull --ff-only origin "$BRANCH"
fi

log "Building production containers..."
docker compose -f "$COMPOSE_FILE" build

log "Restarting systemd service $SERVICE_NAME..."
if systemctl list-unit-files | grep -q "^${SERVICE_NAME}\.service"; then
  systemctl restart "$SERVICE_NAME"
else
  docker compose -f "$COMPOSE_FILE" up -d --remove-orphans
fi

if check_health; then
  log "Deployment succeeded!"
  exit 0
fi

log "ERROR: Healthcheck failed at $HEALTH_URL!"
if [ -n "$PREV_COMMIT" ] && [ "${SKIP_GIT_PULL:-0}" != "1" ]; then
  log "Rolling back to commit $PREV_COMMIT..."
  git checkout "$PREV_COMMIT"
  docker compose -f "$COMPOSE_FILE" build
  if systemctl list-unit-files | grep -q "^${SERVICE_NAME}\.service"; then
    systemctl restart "$SERVICE_NAME"
  else
    docker compose -f "$COMPOSE_FILE" up -d --remove-orphans
  fi
  if check_health; then
    log "Rollback succeeded; service restored to $PREV_COMMIT."
  else
    log "CRITICAL: Rollback healthcheck also failed!"
  fi
fi
exit 1
