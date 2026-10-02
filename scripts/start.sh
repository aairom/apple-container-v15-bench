#!/usr/bin/env bash
# scripts/start.sh — Build and run container-compare in the background.
# Usage: ./scripts/start.sh [command] [flags]
# Example: ./scripts/start.sh info
#          ./scripts/start.sh compare --image alpine:latest
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BINARY="${PROJECT_ROOT}/container-compare"
LOG_FILE="${PROJECT_ROOT}/output/container-compare.log"

# ── Build ──────────────────────────────────────────────────────────────────
echo "🔨  Building container-compare..."
mkdir -p "${PROJECT_ROOT}/output"
cd "${PROJECT_ROOT}"
go build -o "${BINARY}" . 2>&1
echo "✅  Build complete: ${BINARY}"

# ── Default command ────────────────────────────────────────────────────────
COMMAND="${1:-info}"
shift || true   # allow no extra args

# ── Run in background ──────────────────────────────────────────────────────
echo "🚀  Starting: container-compare ${COMMAND} $*"
echo "📋  Log: ${LOG_FILE}"

nohup "${BINARY}" "${COMMAND}" "$@" > "${LOG_FILE}" 2>&1 &
PID=$!
echo "🟢  PID: ${PID}"
echo "    Access log: tail -f ${LOG_FILE}"
echo "    Stop with: ./scripts/stop.sh ${PID}"
