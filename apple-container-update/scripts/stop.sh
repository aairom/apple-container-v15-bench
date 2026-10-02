#!/usr/bin/env bash
# scripts/stop.sh — Gracefully stop a running container-compare process.
# Usage: ./scripts/stop.sh [PID]
# If no PID is given, kills all container-compare processes.
set -euo pipefail

if [[ $# -gt 0 ]]; then
    PID="$1"
    if kill -0 "${PID}" 2>/dev/null; then
        echo "⏹️   Sending SIGTERM to PID ${PID}..."
        kill -TERM "${PID}"
        echo "✅  Stopped PID ${PID}"
    else
        echo "⚠️   No process with PID ${PID} found"
    fi
else
    PIDS=$(pgrep -f "container-compare" || true)
    if [[ -z "${PIDS}" ]]; then
        echo "ℹ️   No container-compare processes running"
    else
        echo "⏹️   Stopping container-compare processes: ${PIDS}"
        # shellcheck disable=SC2086
        kill -TERM ${PIDS}
        echo "✅  Stopped"
    fi
fi
