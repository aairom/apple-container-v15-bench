#!/usr/bin/env bash
# =============================================================================
# scripts/cleanup.sh – Remove all generated artefacts, caches, and temp files
#
# Usage:
#   ./scripts/cleanup.sh           # interactive – confirms before destructive ops
#   ./scripts/cleanup.sh --force   # skip confirmation prompts
#   ./scripts/cleanup.sh --all     # also removes venv and downloaded models
#
# What is cleaned:
#   - output/          generated Markdown reports
#   - logs/            orchestrator log files and PID file
#   - src/__pycache__/ Python bytecode caches
#   - tests/__pycache__/
#   - .pytest_cache/
#   - src/output/      (stray output written inside src/ directory)
#
# With --all:
#   - venv/            Python virtual environment (reinstall with run.sh)
#   - models/          downloaded GGUF model files (re-download manually)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# ── Colour helpers ─────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${GREEN}[cleanup]${NC} $*"; }
warn()  { echo -e "${YELLOW}[cleanup]${NC} $*"; }
danger(){ echo -e "${RED}[cleanup]${NC} $*"; }
step()  { echo -e "${CYAN}  →${NC} $*"; }

FORCE=false
CLEAN_ALL=false

for arg in "$@"; do
    case "$arg" in
        --force) FORCE=true ;;
        --all)   CLEAN_ALL=true ;;
    esac
done

# ── Confirmation helper ────────────────────────────────────────────────────
confirm() {
    local msg="$1"
    if [ "$FORCE" = true ]; then
        return 0
    fi
    read -r -p "$(echo -e "${YELLOW}[cleanup]${NC} ${msg} [y/N] ")" response
    [[ "$response" =~ ^[Yy]$ ]]
}

# ─────────────────────────────────────────────────────────────────────────────
# Standard cleanup (always runs)
# ─────────────────────────────────────────────────────────────────────────────

info "Starting cleanup in: $PROJECT_DIR"
echo ""

# 1. Generated reports
REPORT_COUNT=$(find "$PROJECT_DIR/output" -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
if [ "$REPORT_COUNT" -gt 0 ]; then
    step "Removing $REPORT_COUNT report(s) from output/ …"
    rm -f "$PROJECT_DIR"/output/*.md
    info "output/ reports removed."
else
    info "output/ is already empty."
fi

# 2. Stray output inside src/
if [ -d "$PROJECT_DIR/src/output" ]; then
    step "Removing stray src/output/ directory …"
    rm -rf "$PROJECT_DIR/src/output"
    info "src/output/ removed."
fi

# 3. Log files
if [ -d "$PROJECT_DIR/logs" ]; then
    LOG_COUNT=$(find "$PROJECT_DIR/logs" -type f 2>/dev/null | wc -l | tr -d ' ')
    if [ "$LOG_COUNT" -gt 0 ]; then
        step "Removing $LOG_COUNT log file(s) from logs/ …"
        rm -f "$PROJECT_DIR"/logs/*.log
        rm -f "$PROJECT_DIR"/logs/*.pid
        info "logs/ cleared."
    fi
fi

# 4. Python bytecode caches
step "Removing Python __pycache__ directories …"
find "$PROJECT_DIR" \
    -not -path "*/venv/*" \
    -type d -name "__pycache__" \
    -exec rm -rf {} + 2>/dev/null || true
find "$PROJECT_DIR" \
    -not -path "*/venv/*" \
    -name "*.pyc" -o -name "*.pyo" \
    -delete 2>/dev/null || true
info "__pycache__ and .pyc files removed."

# 5. pytest cache
if [ -d "$PROJECT_DIR/.pytest_cache" ]; then
    step "Removing .pytest_cache/ …"
    rm -rf "$PROJECT_DIR/.pytest_cache"
    info ".pytest_cache removed."
fi

# 6. Streamlit cache
if [ -d "$HOME/.streamlit/cache" ]; then
    step "Removing Streamlit cache …"
    rm -rf "$HOME/.streamlit/cache"
    info "Streamlit cache removed."
fi

echo ""
info "Standard cleanup complete."

# ─────────────────────────────────────────────────────────────────────────────
# Deep cleanup (only with --all)
# ─────────────────────────────────────────────────────────────────────────────

if [ "$CLEAN_ALL" = true ]; then
    echo ""
    warn "Deep cleanup requested (--all)."
    echo ""

    # Virtual environment
    if [ -d "$PROJECT_DIR/venv" ]; then
        if confirm "Remove venv/ virtual environment? (requires reinstall via run.sh)"; then
            step "Removing venv/ …"
            rm -rf "$PROJECT_DIR/venv"
            info "venv/ removed. Re-create with:  ./scripts/run.sh"
        fi
    fi

    # Downloaded GGUF models
    if [ -d "$PROJECT_DIR/models" ]; then
        MODEL_COUNT=$(find "$PROJECT_DIR/models" -name "*.gguf" 2>/dev/null | wc -l | tr -d ' ')
        if [ "$MODEL_COUNT" -gt 0 ]; then
            danger "Found $MODEL_COUNT GGUF model file(s) in models/ (potentially large)."
            if confirm "Remove GGUF model files? (you must re-download them to use llamacpp backend)"; then
                step "Removing GGUF models …"
                rm -f "$PROJECT_DIR"/models/*.gguf
                info "GGUF models removed."
            fi
        fi
    fi

    echo ""
    info "Deep cleanup complete."
fi

echo ""
info "All done. Run  ./scripts/run.sh  to start fresh."
