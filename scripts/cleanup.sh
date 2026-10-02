#!/usr/bin/env bash
# scripts/cleanup.sh — Remove build artefacts and caches.
# Removes: compiled binary, Go build cache (optional), output logs.
# Does NOT remove source files or pulled container images.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "🧹  Cleaning project artefacts..."

# Remove compiled binary
if [[ -f "${PROJECT_ROOT}/container-compare" ]]; then
    rm -f "${PROJECT_ROOT}/container-compare"
    echo "  ✅  Removed binary: container-compare"
fi

# Remove test cache
go clean -testcache 2>/dev/null || true
echo "  ✅  Cleared Go test cache"

# Remove output log file (keep reports — they are in output/)
if [[ -f "${PROJECT_ROOT}/output/container-compare.log" ]]; then
    rm -f "${PROJECT_ROOT}/output/container-compare.log"
    echo "  ✅  Removed log file"
fi

echo "✅  Cleanup complete"
