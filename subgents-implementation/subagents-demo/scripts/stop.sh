#!/usr/bin/env bash
# =============================================================================
# scripts/stop.sh – Gracefully stop all Sub-Agents Demo background processes
#
# Usage:  ./scripts/stop.sh
#
# Stops:
#   - Orchestrator   (PID tracked in logs/subagents.pid)
#   - Streamlit UI   (PID tracked in logs/ui.pid)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_DIR="$PROJECT_DIR/logs"
ORCH_PID_FILE="$LOG_DIR/subagents.pid"
UI_PID_FILE="$LOG_DIR/ui.pid"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[stop]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[stop]${NC}  $*"; }
error() { echo -e "${RED}[stop]${NC}  $*"; }

# ── Helper: stop a single process by PID file ──────────────────────────────
stop_process() {
    local label="$1"
    local pid_file="$2"

    if [ ! -f "$pid_file" ]; then
        warn "$label: no PID file found ($pid_file) – may not be running."
        return 0
    fi

    local pid
    pid=$(cat "$pid_file")

    if kill -0 "$pid" 2>/dev/null; then
        info "Sending SIGTERM to $label (PID: $pid) …"
        kill -TERM "$pid"
        # Wait up to 5 seconds for graceful shutdown
        for _ in $(seq 1 5); do
            sleep 1
            if ! kill -0 "$pid" 2>/dev/null; then
                info "$label stopped."
                rm -f "$pid_file"
                return 0
            fi
        done
        # Force kill if still running
        warn "$label did not stop within 5s – sending SIGKILL …"
        kill -9 "$pid" 2>/dev/null || true
        rm -f "$pid_file"
        info "$label force-stopped."
    else
        error "$label: no process found for PID $pid – already stopped?"
        rm -f "$pid_file"
    fi
}

# ── Stop orchestrator and UI ───────────────────────────────────────────────
stop_process "Orchestrator" "$ORCH_PID_FILE"
stop_process "Streamlit UI" "$UI_PID_FILE"

info "All processes stopped."
