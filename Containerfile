# Multi-stage Containerfile — build container-compare as a minimal OCI image.
# Compatible with both Podman (podman build) and Apple Container (container build).
# Uses a multi-stage build to keep the final image as small as possible.

# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Create a non-root user for the final image.
RUN adduser -D -u 10001 appuser

WORKDIR /build

# Copy dependency manifests first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy source code.
COPY . .

# Build the static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /container-compare .

# ── Stage 2: Minimal runtime image ────────────────────────────────────────────
FROM scratch

# Copy the non-root user from the builder.
COPY --from=builder /etc/passwd /etc/passwd

# Copy the static binary from the builder stage.
COPY --from=builder /container-compare /container-compare

# Run as non-root (UID 10001 created above).
USER 10001

# HEALTHCHECK: this is a CLI tool with no network service.
# Set to NONE to explicitly declare there is no health check endpoint.
HEALTHCHECK NONE

# Default command.
ENTRYPOINT ["/container-compare"]
CMD ["info"]

# ── Labels ────────────────────────────────────────────────────────────────────
LABEL org.opencontainers.image.title="container-compare"
LABEL org.opencontainers.image.description="Educational comparison of Apple Container vs Podman"
LABEL org.opencontainers.image.source="https://github.com/your-org/apple-container-update"
LABEL org.opencontainers.image.licenses="Apache-2.0"
