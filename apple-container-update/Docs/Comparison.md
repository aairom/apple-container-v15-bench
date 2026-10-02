# Apple Container vs Podman — Detailed Comparison

_Based on Apple Container v1.3.1 and Podman v5.x on macOS._

---

## 1. Architecture

| Dimension | Apple Container | Podman |
|-----------|-----------------|--------|
| Isolation technology | Lightweight VM (Virtualization.framework) | Linux namespaces + cgroups |
| VM per container | **Yes** — dedicated VM per container | **No** — shared Podman Machine VM |
| Host OS integration | Native macOS (Swift, Virtualization.framework) | Linux-native via Podman Machine (QEMU) |
| Background service | `com.apple.container.apiserver` (launchd) | **Daemonless** |
| Rootless support | Yes (current user via Virtualization.framework) | Yes (default on macOS via Podman Machine) |
| Guest kernel | Kata Containers 3.32.0-debug (configurable) | Fedora-based Podman Machine kernel |
| Rosetta 2 (x86_64) | **`--rosetta` flag** | Not supported natively |
| Network isolation (macOS 26+) | Full vmnet per-container IP | CNI/Netavark bridge inside Machine VM |
| OCI compatibility | Full | Full |

---

## 2. CLI Command Mapping

| Operation | Apple Container | Podman |
|-----------|-----------------|--------|
| Start runtime | `container system start` | N/A (daemonless) |
| Stop runtime | `container system stop` | N/A (daemonless) |
| Runtime version | `container system version` | `podman version` |
| Pull image | `container image pull <img>` | `podman pull <img>` |
| List images | `container image list` | `podman images` |
| Inspect image | `container image inspect <img>` | `podman inspect --type image <img>` |
| Build image | `container build -t <tag> .` | `podman build -t <tag> .` |
| Remove image | `container image delete <img>` | `podman rmi <img>` |
| Run container | `container run <img>` | `podman run <img>` |
| Run detached | `container run -d --name n <img>` | `podman run -d --name n <img>` |
| List containers | `container list` / `container ls` | `podman ps` |
| List all containers | `container list --all` | `podman ps --all` |
| Inspect container | `container inspect <name>` | `podman inspect <name>` |
| Execute in container | `container exec <name> <cmd>` | `podman exec <name> <cmd>` |
| Container logs | `container logs <name>` | `podman logs <name>` |
| VM boot logs | `container logs --boot <name>` | N/A (no VM boot) |
| Stats (one-shot) | `container stats --no-stream` | `podman stats --no-stream` |
| Copy file | `container cp <name>:<src> <dst>` | `podman cp <name>:<src> <dst>` |
| Stop container | `container stop <name>` | `podman stop <name>` |
| Delete container | `container delete <name>` | `podman rm <name>` |
| Volume create | `container volume create <v>` | `podman volume create <v>` |
| Volume list | `container volume list` | `podman volume ls` |
| Volume delete | `container volume delete <v>` | `podman volume rm <v>` |
| Network create | `container network create <n>` (macOS 26+) | `podman network create <n>` |
| Network list | `container network list` | `podman network ls` |
| Disk usage | `container system df` | `podman system df` |
| Prune containers | `container prune` | `podman container prune` |
| Prune images | `container image prune` | `podman image prune` |

---

## 3. Feature Matrix

| Feature | Apple Container | Podman | Notes |
|---------|-----------------|--------|-------|
| Per-container IP address | ✅ visible in `container list` | ⚠️ requires `podman inspect` | Apple shows VM IP in ADDR column |
| Network isolation (macOS 26+) | ✅ Full vmnet bridge | ✅ CNI/Netavark inside Machine | Apple requires macOS 26 for isolation |
| Rosetta 2 | ✅ `--rosetta` flag | ❌ | Apple Silicon–specific feature |
| SSH agent forwarding | ✅ `--ssh` | ✅ rootless | |
| Volume mounts | ✅ `--volume` / `--mount` | ✅ `--volume` / `--mount` | |
| Anonymous volumes | ✅ | ✅ | |
| tmpfs mounts | ✅ `--tmpfs` | ✅ `--tmpfs` | |
| CPU limiting | ✅ `--cpus` (VM count) | ✅ `--cpus` (cgroup quota) | Different semantics |
| Memory limiting | ✅ `--memory` (VM RAM) | ✅ `--memory` (cgroup limit) | Different semantics |
| Read-only root FS | ✅ `--read-only` | ✅ `--read-only` | |
| Custom networking | ✅ `--network <name>` (macOS 26+) | ✅ `--network <name>` | |
| No network | ✅ `--network none` | ✅ `--network none` | |
| VM boot logs | ✅ `container logs --boot` | ❌ | Apple-only |
| System daemon | ✅ Required | ❌ Daemonless | |
| Kubernetes support | ✅ `container machine` / k8s plugin | ✅ `podman kube` | |
| Buildah integration | ❌ (uses BuildKit) | ✅ `podman build` uses Buildah | |
| BuildKit integration | ✅ `container build` | ❌ | |

---

## 4. JSON Output Format Differences

### `inspect` container — Apple Container

```json
[
  {
    "status": "running",
    "networks": [
      {
        "address": "192.168.64.3/24",
        "gateway": "192.168.64.1",
        "hostname": "my-container.test.",
        "network": "default"
      }
    ],
    "configuration": {
      "id": "my-container",
      "hostname": "my-container",
      "resources": {
        "cpus": 4,
        "memoryInBytes": 1073741824
      },
      "mounts": []
    }
  }
]
```

### `inspect` container — Podman

```json
[
  {
    "Id": "abc123...",
    "Name": "/my-container",
    "State": {
      "Status": "running",
      "Running": true,
      "ExitCode": 0
    },
    "NetworkSettings": {
      "IPAddress": "",
      "Networks": {
        "podman": { "IPAddress": "10.88.0.2" }
      }
    },
    "HostConfig": {
      "Memory": 0,
      "NanoCpus": 0
    }
  }
]
```

**Key differences:**
- Apple Container: `status` (lowercase string) vs Podman: `State.Status`
- Apple Container: `networks[].address` (CIDR notation, always present) vs Podman: `NetworkSettings.IPAddress` (empty for rootless bridge)
- Apple Container: `configuration.resources.memoryInBytes` vs Podman: `HostConfig.Memory`

---

## 5. Performance Characteristics

> These are general guidelines. Actual numbers vary by machine, image, and network.

| Operation | Apple Container | Podman | Notes |
|-----------|-----------------|--------|-------|
| First container startup | ~1–3 s | ~0.3–1 s | Apple creates a new VM; Podman forks a process |
| Subsequent container startup | ~0.5–1.5 s | ~0.3–0.8 s | VM init overhead persists |
| Image pull (cached) | Fast (local store) | Fast (local store) | Both cache layers |
| Container stop | ~0.5–2 s | ~0.3–1 s | Apple shuts down VM; Podman sends SIGTERM |
| `list` / `ps` | Fast | Fast | No measurable difference |
| Memory overhead per container | ~100–200 MB (VM overhead) | ~1–10 MB (namespaces) | VM carries fixed overhead |

Run `container-compare benchmark` for actual measurements on your machine.

---

## 6. Security Comparison

| Property | Apple Container | Podman |
|----------|-----------------|--------|
| Default rootless | Yes | Yes |
| Kernel isolation | VM boundary (strong) | Namespace boundary (moderate) |
| Capabilities | Dropped (within VM) | Dropped (rootless default) |
| Seccomp | VM syscall filtering | Linux seccomp profile |
| CVE exposure | Smaller attack surface (VM hypervisor) | Full Linux kernel exposure |

Apple Container's VM boundary provides stronger kernel-level isolation. Podman's namespace model is well-hardened but shares the host (Machine VM) kernel.

---

## 7. When to Use Each

### Use Apple Container when:
- You need strong per-container isolation (each container has its own kernel)
- You want to run x86_64 images natively via Rosetta 2 (`--rosetta`)
- You are developing on macOS 26+ and want native vmnet networking
- You are contributing to Apple Container itself or testing Apple Silicon workloads

### Use Podman when:
- You need maximum compatibility with Docker-based workflows
- You want faster container startup (process fork vs VM boot)
- You need rootless containers on Linux CI/CD runners
- You need `podman kube` for Kubernetes Pod testing
- You are on macOS 15 (Sequoia) without macOS 26 features
