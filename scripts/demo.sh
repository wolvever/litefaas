#!/usr/bin/env bash
# demo.sh — one-command Go function smoke (init → build → deploy → invoke).
# Requires: docker, lf + litefaasd on PATH (or run via `make demo`).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATA_DIR="${LITEFAAS_DEMO_DATA:-$ROOT/.demo-data}"
CFG_DIR="${LITEFAAS_DEMO_CFG:-$DATA_DIR/cfg}"
ADDR="${LITEFAAS_DEMO_ADDR:-127.0.0.1:18081}"
GATEWAY="http://${ADDR}"

die() { echo "demo.sh: $*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "need $1 on PATH"; }
need docker
need lf
need litefaasd

if ! docker info >/dev/null 2>&1; then
  die "docker daemon not reachable"
fi

mkdir -p "$DATA_DIR" "$CFG_DIR"
WORKDIR=""
cleanup() {
  if [[ -n "${WORKDIR}" && -d "${WORKDIR}" ]]; then
    rm -rf "${WORKDIR}"
  fi
  if [[ -f "$DATA_DIR/litefaasd.pid" ]]; then
    kill "$(cat "$DATA_DIR/litefaasd.pid")" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT HUP

echo "demo.sh: lf up --no-auth (data-dir=$DATA_DIR)"
lf up --no-auth --addr "$ADDR" --data-dir "$DATA_DIR" --config-dir "$CFG_DIR" --timeout 20s

WORKDIR=$(mktemp -d)
NAME="demo-hello"
cd "$WORKDIR"
lf init "$NAME" --runtime go
cd "$NAME"
lf build
lf deploy --gateway "$GATEWAY" --config-dir "$CFG_DIR"
out=$(lf invoke "$NAME" -d '{"name":"litefaas"}' --gateway "$GATEWAY" --config-dir "$CFG_DIR")
echo "$out"
echo "$out" | grep -q 'hello from litefaas' || die "unexpected invoke body: $out"
lf delete "$NAME" --gateway "$GATEWAY" --config-dir "$CFG_DIR" || true

echo
echo "demo.sh: ok — Go function path passed"
echo "Multi-service examples: see examples/ (lf build examples && lf deploy examples)"
