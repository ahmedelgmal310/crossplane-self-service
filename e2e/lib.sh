#!/usr/bin/env bash
# Shared pins and helpers for the e2e scripts. Sourced, not executed.
# shellcheck disable=SC2034 # variables are used by the scripts that source this file

set -euo pipefail

CROSSPLANE_CHART_VERSION="2.4.3"
CROSSPLANE_IMAGE_TAG="v2.4.3@sha256:1c66261d805a73cade46e2ac94b665c73aebd4a00844de5fcae4bcadaa2be638"
CNPG_CHART_VERSION="0.29.1"
CNPG_IMAGE_TAG="1.30.1@sha256:923c267ec29636db3bee20f993d0ec4973fa22998e1adad37da79e4d32b5bc07"
# Same image the Composition gives the database; used for network-policy probes.
PG_IMAGE="ghcr.io/cloudnative-pg/postgresql:18.6-minimal-trixie@sha256:64abc98a4387361afd6821a3a99f975763bbd87bbdde9449e75ad6002528a318"

TEAM_NS="team-payments"
DEV_PAYMENTS=(--as=alice --as-group=team-payments-developers)
DEV_GROWTH=(--as=bob --as-group=team-growth-developers)

RESULTS_DIR="${RESULTS_DIR:-e2e-results}"
mkdir -p "$RESULTS_DIR"
RESULTS_TSV="$RESULTS_DIR/results.tsv"

log() { printf '\n\033[1m[%s] %s\033[0m\n' "$(date -u +%H:%M:%S)" "$*"; }
pass() { printf '  PASS  %s\n' "$*"; }
fail() {
  printf '::error::%s\n' "$*"
  exit 1
}

now_ms() { date +%s%3N; }
# seconds between two millisecond timestamps, one decimal
secs() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.1f", (b - a) / 1000 }'; }

# record <key> <value> <description>: one measured result for the summary
record() { printf '%s\t%s\t%s\n' "$1" "$2" "$3" >>"$RESULTS_TSV"; }

# wait_until <timeout-seconds> <description> <command...>
# Polls every 0.5s; returns when the command succeeds, fails the run on timeout.
wait_until() {
  local timeout=$1 desc=$2 deadline
  shift 2
  deadline=$(($(date +%s) + timeout))
  until "$@" >/dev/null 2>&1; do
    if (($(date +%s) >= deadline)); then
      fail "timed out after ${timeout}s waiting for: ${desc}"
    fi
    sleep 0.5
  done
}

crd_exists() { kubectl get crd "$1"; }

xr_condition() { # <name> <condition-type>
  kubectl get appenvironment "$1" -n "$TEAM_NS" \
    -o jsonpath="{.status.conditions[?(@.type==\"$2\")].status}" 2>/dev/null
}
xr_ready() { [[ "$(xr_condition "$1" Ready)" == "True" ]]; }
