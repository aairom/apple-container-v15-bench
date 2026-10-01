#!/usr/bin/env bash
# =============================================================================
# scripts/ui.sh – Launch the Sub-Agents Demo Streamlit web UI
#
# Usage:
#   ./scripts/ui.sh           # starts on the port in .env (UI_PORT, default 8501)
#   ./scripts/ui.sh --port 8080  # override port
#
# What this does:
#   1. Activates the virtual environment (creates it if absent)
#   2. Installs / verifies dependencies
#   3. Copies .env.example → .env if .env is missing
#   4. Launches Streamlit on the configured port
#   5. Prints the URL to access the UI
#
# NOTE: Port 5000 is reserved for macOS AirDrop and is never used.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
VENV_DIR="$PROJECT_DIR/venv"
ENV_FILE="$PROJECT_DIR/.env"
ENV_EXAMPLE="$PROJECT_DIR/.env.example"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${GREEN}[ui]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[warn]${NC} $*"; }
link()  { echo -e "${CYAN}$*${NC}"; }

# ── Parse optional --port flag ─────────────────────────────────────────────
PORT=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --port) PORT="$2"; shift 2 ;;
        *) shift ;;
    esac
done

# ── Virtual environment ────────────────────────────────────────────────────
if [ ! -d "$VENV_DIR" ]; then
    info "Creating virtual environment …"
    python3 -m venv "$VENV_DIR"
fi
# shellcheck disable=SC1091
source "$VENV_DIR/bin/activate"

# ── Dependencies ───────────────────────────────────────────────────────────
info "Checking dependencies …"
pip install --quiet --upgrade pip
pip install --quiet -r "$PROJECT_DIR/requirements.txt"

# ── .env ───────────────────────────────────────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
    warn ".env not found – copying from .env.example"
    cp "$ENV_EXAMPLE" "$ENV_FILE"
fi

# ── Determine port ─────────────────────────────────────────────────────────
if [ -z "$PORT" ]; then
    # Read UI_PORT from .env file (grep out comments, take first match)
    PORT=$(grep -E '^UI_PORT=' "$ENV_FILE" 2>/dev/null | cut -d= -f2 | tr -d ' ' || echo "8501")
    PORT="${PORT:-8501}"
fi
# Guard: never use port 5000 (macOS AirDrop)
if [ "$PORT" = "5000" ]; then
    warn "Port 5000 is reserved (macOS AirDrop). Switching to 8501."
    PORT=8501
fi

mkdir -p "$PROJECT_DIR/output"

# ── Launch Streamlit ───────────────────────────────────────────────────────
info "Launching Streamlit UI on port $PORT …"
echo ""
link "  ➜  Open in browser:  http://localhost:${PORT}"
echo ""

cd "$PROJECT_DIR/src"
exec streamlit run ui.py \
    --server.port "$PORT" \
    --server.headless true \
    --server.address 0.0.0.0 \
    --browser.gatherUsageStats false
