# container-compare — Project Artifact Summary

**Generated:** 2026-10-02  
**Version:** 1.0.0  
**Apple Container validated against:** v1.3.1

---

## Files Created

### Source Code
| File | Purpose |
|------|---------|
| `main.go` | Entry point; stdlib flag-based CLI dispatcher |
| `internal/applecontainer/client.go` | Apple Container CLI wrapper with input validation |
| `internal/applecontainer/client_test.go` | Unit tests (fake runner, no real binary required) |
| `internal/podman/client.go` | Podman CLI wrapper with input validation |
| `internal/podman/client_test.go` | Unit tests (fake runner, no real binary required) |
| `internal/comparator/matrix.go` | Static comparison matrices + benchmark data models |
| `internal/comparator/matrix_test.go` | Unit tests for comparison data |
| `internal/reporter/reporter.go` | Terminal tables + timestamped Markdown/JSON report writer |
| `internal/runner/runner.go` | Top-level command implementations (info, demo, compare, benchmark) |

### Configuration
| File | Purpose |
|------|---------|
| `go.mod` | Go module definition (stdlib only, no external deps) |
| `go.sum` | Dependency checksums |
| `Containerfile` | Multi-stage OCI build (non-root user, HEALTHCHECK NONE) |
| `.env.example` | Environment variable template for private registries |
| `.gitignore` | Git exclusion rules per AGENTS.md specification |
| `LICENSE` | Apache License 2.0 |

### Documentation
| File | Purpose |
|------|---------|
| `README.md` | Project overview, architecture diagrams, quick start, licence |
| `Docs/Quickstart.md` | Step-by-step setup and usage guide |
| `Docs/Architecture.md` | Mermaid architecture + data flow diagrams |
| `Docs/Comparison.md` | Full comparison matrix: architecture, CLI, features, JSON formats |

### Scripts
| File | Purpose |
|------|---------|
| `scripts/start.sh` | Build + run in background; prints log path |
| `scripts/stop.sh` | Gracefully stop running process(es) |
| `scripts/cleanup.sh` | Remove binary and build artefacts |
| `scripts/delete-images.sh` | Remove pulled demo images from both runtimes |

---

## Architecture Summary

- **Apple Container v1.3.1**: Each container = isolated lightweight VM (Virtualization.framework)
- **Podman**: Daemonless; namespaces + cgroups inside shared Podman Machine VM
- **Both**: OCI-compatible, support volumes, networks, exec, logs, stats

## Security Measures Applied

- All user input (image names, container names, command args) validated with strict allowlist regexes
- `exec.Command` called with literal binary name strings (`"container"`, `"podman"`) — no shell invocation
- `CmdRunner` interface enables full unit testing without spawning real processes
- Containerfile: non-root user (UID 10001), multi-stage build, `HEALTHCHECK NONE`
- `.env.example` provided; no secrets hard-coded anywhere

## Testing

```bash
go test ./...
go test -cover ./...
```

All packages have unit tests using fake runners — no real container binary required.
