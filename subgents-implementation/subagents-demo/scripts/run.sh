#!/usr/bin/env bash
# =============================================================================
# scripts/run.sh – Run the Sub-Agents Demo interactively (foreground)
#
# Usage:  ./scripts/run.sh
#
# This is the simplest way to run the demo.  It sets up the venv, installs
# deps, and runs the orchestrator in the foreground so you can see live output.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
VENV_DIR="$PROJECT_DIR/venv"
ENV_FILE="$PROJECT_DIR/.env"
ENV_EXAMPLE="$PROJECT_DIR/.env.example"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}[run]${NC}  $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }

# Virtual environment
if [ ! -d "$VENV_DIR" ]; then
    info "Creating virtual environment …"
    python3 -m venv "$VENV_DIR"
fi
# shellcheck disable=SC1091
source "$VENV_DIR/bin/activate"

# Dependencies
info "Installing dependencies …"
pip install --quiet --upgrade pip
pip install --quiet -r "$PROJECT_DIR/requirements.txt"

# .env
if [ ! -f "$ENV_FILE" ]; then
    warn ".env not found – copying .env.example (edit $ENV_FILE to configure)"
    cp "$ENV_EXAMPLE" "$ENV_FILE"
fi

# Output dir
mkdir -p "$PROJECT_DIR/output"

# Run
info "Starting orchestrator …"
echo ""
cd "$PROJECT_DIR/src"
python3 orchestrator.py
