#!/usr/bin/env bash
# scripts/delete-images.sh — Remove container images pulled during demos.
# WARNING: This removes local images from both Apple Container and Podman.
# Usage: ./scripts/delete-images.sh [image1 image2 ...]
# If no images are specified, defaults to the demo image set.
set -euo pipefail

DEFAULT_IMAGES=("alpine:latest" "httpd:alpine")

IMAGES=("${@:-${DEFAULT_IMAGES[@]}}")

echo "🗑️   Removing demo container images..."
echo "    Images: ${IMAGES[*]}"

# ── Apple Container ─────────────────────────────────────────────────────────
if command -v container &>/dev/null; then
    echo ""
    echo "🍎  Apple Container:"
    for img in "${IMAGES[@]}"; do
        if container image inspect "${img}" &>/dev/null 2>&1; then
            container image delete "${img}" && echo "  ✅  Removed ${img}" || echo "  ⚠️   Failed to remove ${img}"
        else
            echo "  ℹ️   ${img} not present"
        fi
    done
else
    echo "  ℹ️   Apple Container not found on PATH — skipping"
fi

# ── Podman ───────────────────────────────────────────────────────────────────
if command -v podman &>/dev/null; then
    echo ""
    echo "🦭  Podman:"
    for img in "${IMAGES[@]}"; do
        if podman image inspect "${img}" &>/dev/null 2>&1; then
            podman rmi "${img}" && echo "  ✅  Removed ${img}" || echo "  ⚠️   Failed to remove ${img}"
        else
            echo "  ℹ️   ${img} not present"
        fi
    done
else
    echo "  ℹ️   Podman not found on PATH — skipping"
fi

echo ""
echo "✅  Done"
