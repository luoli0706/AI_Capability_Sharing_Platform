#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ENV_FILE:-${PROJECT_ROOT}/docker/.env}"
ENV_EXAMPLE_FILE="${ENV_EXAMPLE_FILE:-${PROJECT_ROOT}/docker/.env.example}"

DRY_RUN=0
APPLY_SYSCTL=0

usage() {
  cat <<'EOF'
Usage: configure_docker_limits.sh [OPTIONS]

Calculate Docker resource limits from host RAM (80% soft mem_reservation,
90% hard mem_limit) and CPU cores, then update docker/.env while preserving
existing secrets and configuration.

Options:
  --dry-run       Print calculated limits without modifying files or sysctl
  --apply-sysctl  Set vm.swappiness=20 and persist in /etc/sysctl.d/99-ascp-memory.conf
  -h, --help      Show this help message
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    --apply-sysctl)
      APPLY_SYSCTL=1
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

upsert_env() {
  local file="$1"
  local key="$2"
  local val="$3"
  local tmp_file
  tmp_file="$(mktemp)"

  awk -v k="$key" -v v="$val" '
    BEGIN { found = 0 }
    $0 ~ "^" k "=" {
      print k "=" v
      found = 1
      next
    }
    { print }
    END {
      if (!found) {
        print k "=" v
      }
    }
  ' "$file" > "$tmp_file"

  mv -f "$tmp_file" "$file"
}

TOTAL_MEM_MB="$(free -m | awk '/^Mem:/ {print $2}')"
CPU_CORES="$(nproc)"

if ! [[ "$TOTAL_MEM_MB" =~ ^[0-9]+$ ]] || [ "$TOTAL_MEM_MB" -le 0 ]; then
  printf 'ERROR: Failed to detect valid host memory from free -m (got: %s)\n' "$TOTAL_MEM_MB" >&2
  exit 1
fi

if ! [[ "$CPU_CORES" =~ ^[0-9]+$ ]] || [ "$CPU_CORES" -le 0 ]; then
  printf 'ERROR: Failed to detect valid CPU count from nproc (got: %s)\n' "$CPU_CORES" >&2
  exit 1
fi

# 80% soft reservation sum (smooth memory pressure), 90% hard limit sum (OOM clamp)
TOTAL_RES_MB=$(( TOTAL_MEM_MB * 80 / 100 ))
TOTAL_LIM_MB=$(( TOTAL_MEM_MB * 90 / 100 ))

# Ratio allocation: MySQL 40%, Backend 45%, Frontend 15%
MYSQL_RES_MB=$(( TOTAL_RES_MB * 40 / 100 ))
BACKEND_RES_MB=$(( TOTAL_RES_MB * 45 / 100 ))
FRONTEND_RES_MB=$(( TOTAL_RES_MB - MYSQL_RES_MB - BACKEND_RES_MB ))

MYSQL_LIM_MB=$(( TOTAL_LIM_MB * 40 / 100 ))
BACKEND_LIM_MB=$(( TOTAL_LIM_MB * 45 / 100 ))
FRONTEND_LIM_MB=$(( TOTAL_LIM_MB - MYSQL_LIM_MB - BACKEND_LIM_MB ))

# CPU allocation based on host cores (with sensible minimums and capped at host cores)
MYSQL_CPUS="$(LC_ALL=C awk -v n="$CPU_CORES" 'BEGIN { v = n * 0.40; if (v < 0.50) v = 0.50; if (v > n) v = n; printf "%.2f", v }')"
BACKEND_CPUS="$(LC_ALL=C awk -v n="$CPU_CORES" 'BEGIN { v = n * 0.50; if (v < 0.50) v = 0.50; if (v > n) v = n; printf "%.2f", v }')"
FRONTEND_CPUS="$(LC_ALL=C awk -v n="$CPU_CORES" 'BEGIN { v = n * 0.25; if (v < 0.25) v = 0.25; if (v > n) v = n; printf "%.2f", v }')"

log "Host resources detected: RAM=${TOTAL_MEM_MB}m, CPUs=${CPU_CORES}"
log "Target totals: mem_reservation=${TOTAL_RES_MB}m (80%), mem_limit=${TOTAL_LIM_MB}m (90%)"
printf '  mysql    : mem_reservation=%sm, mem_limit=%sm, cpus=%s, oom_score_adj=-300\n' \
  "$MYSQL_RES_MB" "$MYSQL_LIM_MB" "$MYSQL_CPUS"
printf '  backend  : mem_reservation=%sm, mem_limit=%sm, cpus=%s, oom_score_adj=100\n' \
  "$BACKEND_RES_MB" "$BACKEND_LIM_MB" "$BACKEND_CPUS"
printf '  frontend : mem_reservation=%sm, mem_limit=%sm, cpus=%s, oom_score_adj=200\n' \
  "$FRONTEND_RES_MB" "$FRONTEND_LIM_MB" "$FRONTEND_CPUS"

if [ "$DRY_RUN" -eq 1 ]; then
  log "Dry-run mode: skipping updates to $ENV_FILE"
else
  if [ ! -f "$ENV_FILE" ]; then
    if [ -f "$ENV_EXAMPLE_FILE" ]; then
      cp "$ENV_EXAMPLE_FILE" "$ENV_FILE"
      log "Created $ENV_FILE from $ENV_EXAMPLE_FILE"
    else
      touch "$ENV_FILE"
      log "Created empty $ENV_FILE"
    fi
  fi

  upsert_env "$ENV_FILE" "MYSQL_MEM_RESERVATION" "${MYSQL_RES_MB}m"
  upsert_env "$ENV_FILE" "MYSQL_MEM_LIMIT" "${MYSQL_LIM_MB}m"
  upsert_env "$ENV_FILE" "MYSQL_CPUS" "$MYSQL_CPUS"
  upsert_env "$ENV_FILE" "BACKEND_MEM_RESERVATION" "${BACKEND_RES_MB}m"
  upsert_env "$ENV_FILE" "BACKEND_MEM_LIMIT" "${BACKEND_LIM_MB}m"
  upsert_env "$ENV_FILE" "BACKEND_CPUS" "$BACKEND_CPUS"
  upsert_env "$ENV_FILE" "FRONTEND_MEM_RESERVATION" "${FRONTEND_RES_MB}m"
  upsert_env "$ENV_FILE" "FRONTEND_MEM_LIMIT" "${FRONTEND_LIM_MB}m"
  upsert_env "$ENV_FILE" "FRONTEND_CPUS" "$FRONTEND_CPUS"

  log "Updated resource limits in $ENV_FILE (existing secrets preserved)."
fi

# Check swap and vm.swappiness for smooth soft-limit memory reclaim
SWAP_MB="$(free -m | awk '/^Swap:/ {print $2}')"
SWAPPINESS="$(sysctl -n vm.swappiness 2>/dev/null || cat /proc/sys/vm/swappiness 2>/dev/null || echo "unknown")"

log "Host Swap=${SWAP_MB:-0}m, vm.swappiness=${SWAPPINESS}"

if [ -z "$SWAP_MB" ] || [ "$SWAP_MB" -eq 0 ]; then
  log "WARNING: Swap is 0 MB. mem_reservation soft-limit smoothing works best with swap enabled."
  log "TIP: Consider creating a 1G-2G swapfile (e.g., fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile)."
fi

if [ "$APPLY_SYSCTL" -eq 1 ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    log "Dry-run mode: would set vm.swappiness=20 in /etc/sysctl.d/99-ascp-memory.conf"
  elif [ "$(id -u)" -ne 0 ]; then
    printf 'ERROR: --apply-sysctl requires root privileges.\n' >&2
    exit 1
  else
    sysctl -w vm.swappiness=20 >/dev/null
    printf 'vm.swappiness = 20\n' > /etc/sysctl.d/99-ascp-memory.conf
    log "Applied and persisted vm.swappiness=20 in /etc/sysctl.d/99-ascp-memory.conf"
  fi
elif [ "$SWAPPINESS" = "0" ] || [ "$SWAPPINESS" = "unknown" ]; then
  log "TIP: vm.swappiness is ${SWAPPINESS}; run with --apply-sysctl (as root) to set vm.swappiness=20 for smoother memory reclaim."
fi
