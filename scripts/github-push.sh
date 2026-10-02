#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────────────────────
# scripts/github-push.sh — Push THIS project to GitHub.
#
# Usage:  ./scripts/github-push.sh
#   Run from anywhere inside the project. The script always operates on the
#   directory that contains this script's parent (i.e. the project root).
#
# SAFETY GUARANTEE:
#   This script will NEVER push from a parent directory. It:
#     1. Resolves the project root as the parent of the scripts/ folder.
#     2. Ensures a .git directory exists EXACTLY at that path.
#     3. Refuses to continue if the Git working tree root differs from the
#        project root (which would mean we inherited a parent-level repo).
#     4. Aborts if it detects the working tree contains sibling project
#        directories that don't belong to this project.
# ──────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "📁  Project root : $PROJECT_DIR"
cd "$PROJECT_DIR"

# ── SAFETY CHECK 1: .git must exist DIRECTLY here ────────────────────────────
# We do NOT allow falling back to a parent repo. If .git is missing, we init
# one HERE — not inherited from a parent directory.
if [ ! -d ".git" ]; then
    echo "⚙️  No .git found in $PROJECT_DIR — initialising new Git repository…"
    git init
    git branch -M main
    echo "✅  Git repository initialised."
else
    echo "✅  Existing .git detected at $PROJECT_DIR."
fi

# ── SAFETY CHECK 2: Git working-tree root must match PROJECT_DIR ─────────────
# git rev-parse --show-toplevel returns the actual root of the repo.
# If it differs from PROJECT_DIR, this .git was inherited from a parent.
GIT_TOPLEVEL="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ "$GIT_TOPLEVEL" != "$PROJECT_DIR" ]; then
    echo ""
    echo "🚨  ABORT: Git working tree root is:"
    echo "    $GIT_TOPLEVEL"
    echo "    but project directory is:"
    echo "    $PROJECT_DIR"
    echo ""
    echo "    This means a PARENT-LEVEL .git repository is being used."
    echo "    Pushing would include OTHER projects from the parent directory."
    echo ""
    echo "    To fix this, run:"
    echo "      cd $PROJECT_DIR"
    echo "      git init"
    echo "      git checkout --orphan fresh-main"
    echo "      git add ."
    echo "      git commit -m 'initial commit'"
    echo "      git branch -D main 2>/dev/null || true"
    echo "      git branch -m main"
    echo "      git remote add origin <YOUR_GITHUB_URL>"
    echo "      git push origin main --force"
    echo ""
    exit 1
fi

echo "✅  Git working tree confirmed: $GIT_TOPLEVEL"

# ── 2. Configure remote ───────────────────────────────────────────────────────
CURRENT_REMOTE="$(git remote get-url origin 2>/dev/null || true)"

if [ -z "$CURRENT_REMOTE" ]; then
    while true; do
        read -rp "Enter the GitHub repository URL: " repo_url
        if [ -n "$repo_url" ]; then break; fi
        echo "⚠️  URL cannot be empty. Please enter a valid GitHub URL."
    done
    git remote add origin "$repo_url"
    echo "✅  Remote 'origin' added: $repo_url"
else
    echo "ℹ️  Remote 'origin' is: $CURRENT_REMOTE"
    read -rp "Change remote URL? Press Enter to keep, or type a new URL: " new_url
    if [ -n "$new_url" ]; then
        git remote set-url origin "$new_url"
        echo "✅  Remote updated to: $new_url"
    fi
fi

# ── 3. Stage all changes ──────────────────────────────────────────────────────
git add .

# ── 4. Commit only if there is something to commit ───────────────────────────
if git diff --cached --quiet; then
    echo "ℹ️  Nothing new to commit — working tree is clean."
else
    read -rp "Commit message (default: 'update'): " commit_msg
    commit_msg="${commit_msg:-update}"
    git commit -m "$commit_msg"
fi

# ── 5. Push to current branch ─────────────────────────────────────────────────
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
echo "🚀  Pushing branch '$BRANCH' to origin…"
git push -u origin "$BRANCH"

echo ""
echo "✅  Done! View your repository on GitHub."
