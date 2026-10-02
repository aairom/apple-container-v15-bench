# Architecture

This document describes the technical architecture of `container-compare` and the two runtimes it exercises.

---

## System Architecture

```mermaid
flowchart TD
    subgraph HOST["macOS Host (Apple Silicon)"]
        subgraph APP["container-compare (Go binary)"]
            MAIN["main.go\nCobra CLI"]
            RUNNER["internal/runner\nCommand handlers"]
            ACPKG["internal/applecontainer\nClient wrapper"]
            PMPKG["internal/podman\nClient wrapper"]
            CMPKG["internal/comparator\nMatrix + models"]
            REPKG["internal/reporter\nTable + file output"]
            MAIN --> RUNNER
            RUNNER --> ACPKG
            RUNNER --> PMPKG
            RUNNER --> CMPKG
            RUNNER --> REPKG
        end

        subgraph APPLE["Apple Container Runtime"]
            APISERVER["com.apple.container.apiserver\n(launchd service)"]
            VM1["Lightweight VM 1\n(Virtualization.framework)"]
            VM2["Lightweight VM 2\n(Virtualization.framework)"]
            APISERVER --> VM1
            APISERVER --> VM2
        end

        subgraph PODMAN["Podman Runtime"]
            PMACHINE["Podman Machine VM\n(QEMU / Apple Hypervisor)"]
            CTR1["Container process 1\n(Linux namespaces)"]
            CTR2["Container process 2\n(Linux namespaces)"]
            PMACHINE --> CTR1
            PMACHINE --> CTR2
        end

        ACPKG -->|"exec container ..."| APISERVER
        PMPKG -->|"exec podman ..."| PMACHINE
        REPKG -->|"write"| OUTPUT["./output/\n*.md *.json"]
    end
```

---

## Apple Container Architecture

```mermaid
flowchart LR
    subgraph macOS["macOS 26 (Tahoe)"]
        CLI["container CLI"] --> API["API Server\ncom.apple.container.apiserver"]
        API --> VF["Virtualization.framework"]
        VF --> VM["Lightweight Linux VM"]
        VM --> KERNEL["Kata Kernel (3.32.0)"]
        VM --> CONTAINER["Container Process\n(init + app)"]
    end

    REGISTRY["OCI Registry\n(Docker Hub, GHCR...)"] -->|"image pull"| API
    CONTAINER -->|"network (vmnet)"| NETWORK["macOS vmnet\n(isolated, macOS 26+)"]
```

### Key concepts

- **API Server** (`com.apple.container.apiserver`): The launchd background service that manages the container lifecycle. Start with `container system start`, stop with `container system stop`.
- **Lightweight VM**: Each container gets its own isolated VM via `Virtualization.framework`. This provides stronger isolation than namespace-based containers at the cost of slightly higher startup time.
- **Kata Kernel**: Apple Container uses a Kata Containers kernel as the guest OS kernel inside each VM. The default is `3.32.0-debug` as of v1.3.0.
- **vmnet networking**: On macOS 26+, each VM gets its own virtual NIC with a unique IP. The IP is visible in `container list` output.
- **Rosetta 2**: The `--rosetta` flag enables Apple's binary translation layer inside the VM, allowing x86_64 images to run natively on Apple Silicon.

---

## Podman Architecture (macOS)

```mermaid
flowchart LR
    subgraph macOS["macOS"]
        CLI["podman CLI\n(daemonless)"]
    end

    subgraph PM["Podman Machine VM\n(Fedora Linux)"]
        CONMON["conmon\ncontainer monitor"]
        RUNC["runc / crun\nOCI runtime"]
        CTR["Container\n(namespaces + cgroups)"]
        CONMON --> RUNC --> CTR
    end

    CLI -->|"Unix socket / SSH"| PM
    REGISTRY["OCI Registry"] -->|"image pull"| PM
    CTR -->|"CNI / Netavark bridge"| NETWORK["NAT → macOS host"]
```

### Key concepts

- **Daemonless**: Every `podman` invocation is a self-contained process. There is no persistent Podman daemon on macOS.
- **Podman Machine**: On macOS, Podman runs containers inside a Linux VM (Podman Machine). The VM is shared among all containers.
- **conmon**: A lightweight container monitor process that maintains the container state and pipes I/O.
- **crun / runc**: The OCI runtime that creates Linux namespaces and cgroups for isolation.
- **Netavark / CNI**: Podman uses either CNI or Netavark for container networking inside the Linux VM.

---

## Security Model

Both runtimes validate all input before constructing exec.Command calls:

```mermaid
flowchart TD
    INPUT["User input\n(image name, container name)"] --> REGEX["Allowlist regex\nvalidateImage()\nvalidateContainerName()\nvalidateSafeArg()"]
    REGEX -->|"valid"| CMD["exec.Command('container', args...)"]
    REGEX -->|"invalid"| ERR["error returned\nno process spawned"]
    CMD --> NOSHELL["No shell involved\nargs passed directly to OS"]
```

---

## Data Flow

```mermaid
flowchart LR
    USER["User"] --> COBRA["Cobra CLI\nmain.go"]
    COBRA --> RUNNER["runner.go\nOrchestrates operations"]
    RUNNER --> AC["applecontainer.Client"]
    RUNNER --> PM["podman.Client"]
    RUNNER --> MATRIX["comparator.CLIMappings()\ncomparator.FeatureComparison()\ncomparator.ArchComparison()"]
    AC --> ACR["realRunner\nexec.Command('container',...)"]
    PM --> PMR["realRunner\nexec.Command('podman',...)"]
    RUNNER --> REPORT["reporter.PrintComparisonRows()\nreporter.WriteMarkdownReport()\nreporter.WriteJSONBenchmark()"]
    REPORT --> OUT["./output/*.md\n./output/*.json"]
```



