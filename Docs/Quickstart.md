# Quickstart Guide

This guide gets you from zero to running `container-compare` in under five minutes.

---

## 1 · Prerequisites

| Requirement | Install Command |
|-------------|----------------|
| Go 1.22+ | `brew install go` |
| Apple Container v1.3.1 | Download from [github.com/apple/container/releases](https://github.com/apple/container/releases/tag/1.3.1) |
| Podman (+ Desktop) | `brew install podman && podman machine init && podman machine start` |

> **Note:** Apple Container requires **macOS 26 (Tahoe)** for full network isolation. It can run on macOS 15 (Sequoia) with limited functionality.

---

## 2 · Clone & Build

```bash
git clone https://github.com/your-org/apple-container-update
cd apple-container-update
go build -o container-compare .
```

---

## 3 · Start Apple Container's API Server

Apple Container requires its API server to be running before any operation:

```bash
container system start
container system status   # should print "running"
```

Podman is daemonless — no startup needed beyond the Podman Machine:

```bash
podman machine start      # if not already running
podman version            # verify
```

---

## 4 · Detect Runtimes

```bash
./container-compare info
```

Expected output (versions may differ):

```
═══════════════════════════════════════════════════════════════════
  Container Runtime Detection
═══════════════════════════════════════════════════════════════════

🍎  Apple Container
    ✅  Version: 1.3.1
    🔵  System status: running

🦭  Podman
    ✅  Version: 5.2.3
    🔵  System info available
```

---

## 5 · Run the Demo

```bash
./container-compare demo --image alpine:latest --runtime both
```

This walks through the full lifecycle: **pull → run → list → inspect → logs → stop → remove**.

To run only one runtime:

```bash
./container-compare demo --runtime apple
./container-compare demo --runtime podman
```

---

## 6 · Side-by-Side Comparison

```bash
./container-compare compare
```

Produces:
- A terminal table showing operation results for both runtimes
- A Markdown report in `./output/comparison_report_<timestamp>.md`

---

## 7 · Benchmark

```bash
./container-compare benchmark --iterations 5
```

Runs each operation 5 times on both runtimes and prints mean/min/max durations.
Results are saved to `./output/benchmark_<timestamp>.json`.

---

## 8 · Visual Dashboard

Generate a self-contained HTML dashboard that visualises all comparison data,
live operation results, SVG benchmark bar charts, and educational annotations:

```bash
./container-compare dashboard
```

The dashboard is saved to `./output/dashboard_<timestamp>.html` and opened
automatically in your default browser.

Options:

```bash
# Use a specific image and run 3 benchmark iterations
./container-compare dashboard --image nginx:alpine --iterations 3

# Generate the file without auto-opening the browser
./container-compare dashboard --no-browser
# then open manually:
open output/dashboard_*.html
```

The dashboard contains seven tabs:

| Tab | Content |
|-----|---------|
| 🏠 Overview | Runtime availability cards, summary metrics, key insight callout |
| 🏗 Architecture | VM-per-container vs namespace isolation comparison table |
| ⌨️ CLI Mapping | Side-by-side `container` ↔ `podman` command translations |
| 🔍 Features | Capability matrix (rootless, GPU, image format, etc.) |
| ⚡ Live Results | Colour-coded OK / ERROR / SKIP badges for each live operation |
| 🚀 Benchmarks | SVG horizontal bar charts + mean/min/max timing table |
| 📚 Lessons | Educational annotations explaining key behavioural differences |

---

## 9 · Cleanup

```bash
./scripts/cleanup.sh          # remove build artefacts
./scripts/delete-images.sh    # remove pulled container images
container system stop         # stop Apple Container API server
podman machine stop           # stop Podman Machine VM
```

---

## Troubleshooting

### Apple Container: `binary not found on PATH`
Ensure Apple Container is installed:
```bash
which container        # should print /usr/local/bin/container
container --version
```
If missing, install from [github.com/apple/container/releases](https://github.com/apple/container/releases).

### Apple Container: `apiserver not running`
```bash
container system start
```
If it hangs, try:
```bash
launchctl bootout gui/$(id -u)/com.apple.container.apiserver
container system start
```

### Podman: `podman not found`
```bash
brew install podman
podman machine init
podman machine start
```

### Go build errors
```bash
go mod tidy   # re-download dependencies
go vet ./...  # check for issues
```
