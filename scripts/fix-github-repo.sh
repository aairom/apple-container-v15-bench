#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────────────────────
# scripts/fix-github-repo.sh — ONE-TIME fix for the "parent-directory push" bug.
#
# PROBLEM: GitHub repo contains 'apple-container-update/' and
# 'subgents-implementation/subagents-demo/' as subdirectories because a push
# was made from the parent Devs/ directory instead of the project root.
#
# THIS SCRIPT force-replaces the remote with ONLY this project's files.
#
# Usage (run from macOS Terminal.app — NOT from Bob):
#   cd /Users/alainairom/Devs/apple-container-update
#   chmod +x scripts/fix-github-repo.sh
#   ./scripts/fix-github-repo.sh
# ──────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo ""
echo "══════════════════════════════════════════════════════════════════════"
echo "  GitHub Repository Fix"
echo "  Project root: $PROJECT_DIR"
echo "══════════════════════════════════════════════════════════════════════"
echo ""

# ── Change into project root ──────────────────────────────────────────────────
cd "$PROJECT_DIR"

# ── CRITICAL: Set GIT_DIR explicitly to THIS project's .git only ─────────────
# This prevents Git from walking up to a parent directory's .git.
# We force Git to use ONLY the .git inside our project root.
export GIT_DIR="$PROJECT_DIR/.git"
export GIT_WORK_TREE="$PROJECT_DIR"

# ── Initialise .git here if missing ──────────────────────────────────────────
if [ ! -d "$GIT_DIR" ]; then
    echo "⚙️  Initialising Git repository at: $GIT_DIR"
    # Temporarily unset to allow git init to work normally
    unset GIT_DIR GIT_WORK_TREE
    git init "$PROJECT_DIR"
    export GIT_DIR="$PROJECT_DIR/.git"
    export GIT_WORK_TREE="$PROJECT_DIR"
    echo "✅  Git initialised."
else
    echo "✅  Found existing .git at: $GIT_DIR"
fi

# ── Confirm toplevel is now correct ───────────────────────────────────────────
TOPLEVEL="$(git rev-parse --show-toplevel)"
echo "✅  Git working tree root: $TOPLEVEL"

if [ "$TOPLEVEL" != "$PROJECT_DIR" ]; then
    echo ""
    echo "🚨  ERROR: Even after setting GIT_DIR, toplevel is wrong."
    echo "    Expected: $PROJECT_DIR"
    echo "    Got:      $TOPLEVEL"
    echo ""
    echo "    Please run these commands manually in Terminal:"
    echo "      cd $PROJECT_DIR"
    echo "      rm -rf .git"
    echo "      git init"
    echo "      git checkout --orphan clean-start"
    echo "      git add ."
    echo "      git commit -m 'initial commit'"
    echo "      git branch -D main 2>/dev/null; git branch -m main"
    echo "      git remote add origin https://github.com/aairom/apple-container-v15-bench.git"
    echo "      git push origin main --force"
    exit 1
fi

# ── Configure remote ──────────────────────────────────────────────────────────
CURRENT_REMOTE="$(git remote get-url origin 2>/dev/null || true)"
DEFAULT_REMOTE="https://github.com/aairom/apple-container-v15-bench.git"

if [ -z "$CURRENT_REMOTE" ]; then
    echo "⚙️  Adding remote origin: $DEFAULT_REMOTE"
    git remote add origin "$DEFAULT_REMOTE"
    CURRENT_REMOTE="$DEFAULT_REMOTE"
else
    echo "ℹ️  Remote origin: $CURRENT_REMOTE"
    read -rp "    Press Enter to keep, or type a new URL: " new_url
    if [ -n "$new_url" ]; then
        git remote set-url origin "$new_url"
        CURRENT_REMOTE="$new_url"
        echo "✅  Remote updated to: $CURRENT_REMOTE"
    fi
fi

# ── Confirm before destructive force-push ─────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ⚠️  About to FORCE-PUSH to: $CURRENT_REMOTE"
echo ""
echo "  This will COMPLETELY REPLACE the remote repo history."
echo "  The subdirectory layout (apple-container-update/ and"
echo "  subgents-implementation/) will be GONE."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
read -rp "  Type 'yes' to continue: " confirm
[ "$confirm" = "yes" ] || { echo "Aborted."; exit 0; }

# ── Create orphan branch (zero history) ───────────────────────────────────────
echo ""
echo "⚙️  Creating orphan branch…"
git branch -D fresh-main 2>/dev/null || true
git checkout --orphan fresh-main

# ── Stage everything ──────────────────────────────────────────────────────────
echo "⚙️  Staging all files…"
git add .

TOTAL="$(git diff --cached --name-only | wc -l | tr -d ' ')"
echo "    $TOTAL files staged."
echo ""
echo "📋  Sample (first 15):"
git diff --cached --name-only | head -15
echo ""

# ── Commit ────────────────────────────────────────────────────────────────────
git commit -m "feat: apple-container-update — clean initial commit

Educational Go app comparing Apple Container vs Podman.
Files are at repository root (no subdirectory wrapping).

Removed: old parent-directory commit history."

echo "✅  Commit created."

# ── Replace main ──────────────────────────────────────────────────────────────
git branch -D main 2>/dev/null && echo "✅  Deleted old 'main'." || echo "ℹ️  No 'main' to delete."
git branch -m main
echo "✅  Branch renamed to 'main'."

# ── Force push ────────────────────────────────────────────────────────────────
echo ""
echo "🚀  Force-pushing to origin/main…"
git push origin main --force

echo ""
echo "══════════════════════════════════════════════════════════════════════"
echo "  ✅  DONE!"
echo ""
echo "  GitHub repo now contains ONLY this project's files at root level."
echo "  View: $CURRENT_REMOTE"
echo "══════════════════════════════════════════════════════════════════════"
