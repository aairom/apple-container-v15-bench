#!/usr/bin/env bash
# =============================================================================
# scripts/start.sh – Launch the Sub-Agents Demo in detached mode
#
# Usage:  ./scripts/start.sh
#
# What this does:
#   1. Creates a Python virtual environment (venv/) if one does not exist.
#   2. Installs dependencies from requirements.txt.
#   3. Copies .env.example → .env if no .env is present.
#   4. Creates the output/ and logs/ directories.
#   5. Starts the orchestrator in the background → logs/subagents.log
#   6. Starts the Streamlit web UI in the background → logs/ui.log
#   7. Prints the URL to access the UI on the console (AGENTS.md requirement).
#
# To stop all detached processes: ./scripts/stop.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
VENV_DIR="$PROJECT_DIR/venv"
LOG_DIR="$PROJECT_DIR/logs"
ORCH_LOG="$LOG_DIR/subagents.log"
UI_LOG="$LOG_DIR/ui.log"
ORCH_PID_FILE="$LOG_DIR/subagents.pid"
UI_PID_FILE="$LOG_DIR/ui.pid"
ENV_FILE="$PROJECT_DIR/.env"
ENV_EXAMPLE="$PROJECT_DIR/.env.example"

# ── Colour helpers ─────────────────────────────────────────────────────────
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${GREEN}[start]${NC} $*"; }
warn()  { echo -e "${YELLOW}[warn]${NC}  $*"; }
link()  { echo -e "${CYAN}${BOLD}$*${NC}"; }

# ── 1. Virtual environment ─────────────────────────────────────────────────
if [ ! -d "$VENV_DIR" ]; then
    info "Creating virtual environment at $VENV_DIR …"
    python3 -m venv "$VENV_DIR"
fi

# Activate venv
# shellcheck disable=SC1091
source "$VENV_DIR/bin/activate"

# ── 2. Install dependencies ────────────────────────────────────────────────
info "Installing dependencies …"
pip install --quiet --upgrade pip
pip install --quiet -r "$PROJECT_DIR/requirements.txt"

# ── 3. Copy .env.example if no .env exists ────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
    warn ".env not found – copying from .env.example (edit before running)"
    cp "$ENV_EXAMPLE" "$ENV_FILE"
fi

# ── 4. Prepare directories ─────────────────────────────────────────────────
mkdir -p "$LOG_DIR"
mkdir -p "$PROJECT_DIR/output"

# ── 5. Determine UI port ───────────────────────────────────────────────────
UI_PORT=$(grep -E '^UI_PORT=' "$ENV_FILE" 2>/dev/null | cut -d= -f2 | tr -d ' ' || true)
UI_PORT="${UI_PORT:-8501}"
# Guard: never use port 5000 (macOS AirDrop)
if [ "$UI_PORT" = "5000" ]; then
    warn "Port 5000 is reserved (macOS AirDrop). Switching to 8501."
    UI_PORT=8501
fi

# ── 6. Launch orchestrator in detached mode ────────────────────────────────
# Stop any previously running orchestrator first
if [ -f "$ORCH_PID_FILE" ]; then
    OLD_PID=$(cat "$ORCH_PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        info "Stopping previous orchestrator (PID: $OLD_PID) …"
        kill -TERM "$OLD_PID" 2>/dev/null || true
        sleep 1
    fi
    rm -f "$ORCH_PID_FILE"
fi

info "Starting orchestrator (detached) …"
cd "$PROJECT_DIR/src"
nohup python3 orchestrator.py > "$ORCH_LOG" 2>&1 &
ORCH_PID=$!
echo "$ORCH_PID" > "$ORCH_PID_FILE"
info "Orchestrator started  (PID: $ORCH_PID)"

# ── 7. Launch Streamlit UI in detached mode ────────────────────────────────
# Stop any previously running UI first
if [ -f "$UI_PID_FILE" ]; then
    OLD_UI_PID=$(cat "$UI_PID_FILE")
    if kill -0 "$OLD_UI_PID" 2>/dev/null; then
        info "Stopping previous UI (PID: $OLD_UI_PID) …"
        kill -TERM "$OLD_UI_PID" 2>/dev/null || true
        sleep 1
    fi
    rm -f "$UI_PID_FILE"
fi

info "Starting Streamlit UI on port $UI_PORT (detached) …"
nohup streamlit run "$PROJECT_DIR/src/ui.py" \
    --server.port "$UI_PORT" \
    --server.headless true \
    --server.address 0.0.0.0 \
    --browser.gatherUsageStats false \
    > "$UI_LOG" 2>&1 &
UI_PID=$!
echo "$UI_PID" > "$UI_PID_FILE"
info "Streamlit UI started  (PID: $UI_PID)"

# ── 8. Print access URL and helper commands ────────────────────────────────
echo ""
echo "  ┌──────────────────────────────────────────────────────────┐"
echo "  │  Sub-Agents Demo is running                              │"
echo "  └──────────────────────────────────────────────────────────┘"
echo ""
link "  ➜  Web UI:  http://localhost:${UI_PORT}"
echo ""
echo "  Orchestrator log : $ORCH_LOG"
echo "  UI log           : $UI_LOG"
echo "  Output directory : $PROJECT_DIR/output/"
echo ""
echo "  Useful commands:"
echo "    tail -f $ORCH_LOG   # follow orchestrator output"
echo "    tail -f $UI_LOG     # follow UI log"
echo "    ./scripts/stop.sh   # stop all detached processes"
echo ""
