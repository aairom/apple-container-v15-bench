// Package comparator defines the data models and comparison matrix used to
// describe and display differences between Apple Container and Podman.
//
// Comparison categories:
//  1. Architecture     — how isolation is achieved on macOS
//  2. CLI commands     — command mapping between the two tools
//  3. Features         — what each runtime supports / lacks
//  4. Performance      — startup, pull, and run timing data
//  5. Output formats   — JSON field name differences
package comparator

import (
	"strings"
	"time"
)

// ── Data models ───────────────────────────────────────────────────────────────

// RuntimeName identifies a container runtime.
type RuntimeName string

const (
	AppleContainer RuntimeName = "Apple Container"
	Podman         RuntimeName = "Podman"
)

// OperationResult records the outcome of a single container operation.
type OperationResult struct {
	// Runtime identifies which tool was used.
	Runtime RuntimeName
	// OperationName is a short human-readable description (e.g. "Pull image").
	OperationName string
	// Command is the exact shell command that was (or would be) run.
	Command string
	// Duration is how long the operation took.
	Duration time.Duration
	// Output is a (possibly truncated) excerpt of the command's stdout.
	Output string
	// Err is non-nil when the operation failed.
	Err error
}

// ComparisonRow pairs the Apple Container result and Podman result for
// the same logical operation.
type ComparisonRow struct {
	OperationName string
	Apple         OperationResult
	Podman        OperationResult
}

// FeatureRow describes a single feature dimension and its availability
// on each runtime.
type FeatureRow struct {
	Feature      string
	Apple        string // e.g. "✅ yes", "⚠️ macOS 26+", "❌ no"
	Podman       string
	Notes        string
}

// ArchRow compares an architectural dimension.
type ArchRow struct {
	Dimension    string
	Apple        string
	Podman       string
}

// CLIMapping maps the same logical operation to commands on each runtime.
type CLIMapping struct {
	Operation    string
	AppleCLI     string
	PodmanCLI    string
	Notes        string
}

// ── Static comparison tables ──────────────────────────────────────────────────

// ArchComparison returns the architectural comparison matrix.
func ArchComparison() []ArchRow {
	return []ArchRow{
		{
			Dimension: "Isolation technology",
			Apple:     "Lightweight VM (Virtualization.framework)",
			Podman:    "Linux namespaces + cgroups (inside Podman Machine VM on macOS)",
		},
		{
			Dimension: "VM per container",
			Apple:     "Yes — each container gets its own VM",
			Podman:    "No — all containers share the Podman Machine VM",
		},
		{
			Dimension: "Host OS integration",
			Apple:     "Native macOS (Swift, Virtualization.framework, vmnet)",
			Podman:    "Linux-native; macOS via Podman Machine (QEMU/Apple HV)",
		},
		{
			Dimension: "Daemon",
			Apple:     "API server (launchd: com.apple.container.apiserver)",
			Podman:    "Daemonless — each `podman` call is a standalone process",
		},
		{
			Dimension: "Rootless support",
			Apple:     "Yes (runs as current user via Virtualization.framework)",
			Podman:    "Yes (default mode on macOS via Podman Machine)",
		},
		{
			Dimension: "Kernel used",
			Apple:     "Kata Containers kernel (configurable; default 3.32.0-debug)",
			Podman:    "Podman Machine VM kernel (Fedora-based)",
		},
		{
			Dimension: "Rosetta 2 support",
			Apple:     "Yes — run x86_64 images with --rosetta flag",
			Podman:    "Not directly; requires a multi-arch QEMU emulation layer",
		},
		{
			Dimension: "Networking (macOS 26+)",
			Apple:     "Full vmnet isolation; per-container IP from VM NIC",
			Podman:    "CNI/Netavark bridge inside Podman Machine; NAT to macOS",
		},
		{
			Dimension: "OCI compatibility",
			Apple:     "Full — pulls/pushes any OCI-compliant registry",
			Podman:    "Full — pulls/pushes any OCI-compliant registry",
		},
		{
			Dimension: "Image format",
			Apple:     "OCI Image Layout (arm64 native by default)",
			Podman:    "OCI Image Layout (multi-arch aware)",
		},
		{
			Dimension: "Build support",
			Apple:     "container build (BuildKit integration)",
			Podman:    "podman build (Buildah integration)",
		},
		{
			Dimension: "Kubernetes compatibility",
			Apple:     "container k8s (via container-k8s plugin)",
			Podman:    "podman kube play / podman kube generate",
		},
	}
}

// FeatureComparison returns the feature comparison matrix.
func FeatureComparison() []FeatureRow {
	return []FeatureRow{
		{
			Feature: "Start system daemon",
			Apple:   "✅  container system start",
			Podman:  "➖  N/A (daemonless)",
			Notes:   "Apple Container requires the apiserver to be running.",
		},
		{
			Feature: "Run container",
			Apple:   "✅  container run",
			Podman:  "✅  podman run",
			Notes:   "Flags are mostly identical; --rosetta is Apple-only.",
		},
		{
			Feature: "Pull image",
			Apple:   "✅  container image pull",
			Podman:  "✅  podman pull",
			Notes:   "Apple Container uses `image pull`; Podman uses top-level `pull`.",
		},
		{
			Feature: "List images",
			Apple:   "✅  container image list",
			Podman:  "✅  podman images",
			Notes:   "Different sub-command naming conventions.",
		},
		{
			Feature: "List containers",
			Apple:   "✅  container list / ls",
			Podman:  "✅  podman ps",
			Notes:   "Apple uses `list`; Podman uses classic Docker `ps`.",
		},
		{
			Feature: "Inspect container",
			Apple:   "✅  container inspect",
			Podman:  "✅  podman inspect",
			Notes:   "JSON schema differs; Apple includes VM IP in networks[].",
		},
		{
			Feature: "Container IP address",
			Apple:   "✅  shown in `container list` (ADDR column)",
			Podman:  "⚠️   requires `podman inspect` or `podman ps --format`",
			Notes:   "Apple Container always assigns a unique VM IP.",
		},
		{
			Feature: "Logs",
			Apple:   "✅  container logs (+ --boot for VM boot log)",
			Podman:  "✅  podman logs",
			Notes:   "--boot is Apple Container specific (VM boot messages).",
		},
		{
			Feature: "Stats",
			Apple:   "✅  container stats",
			Podman:  "✅  podman stats",
			Notes:   "Both support --no-stream for a single snapshot.",
		},
		{
			Feature: "Delete container",
			Apple:   "✅  container delete / rm",
			Podman:  "✅  podman rm",
			Notes:   "Apple Container supports both `delete` and `rm` aliases.",
		},
		{
			Feature: "Delete image",
			Apple:   "✅  container image delete / rm",
			Podman:  "✅  podman rmi",
			Notes:   "Apple Container uses `image delete`; Podman uses `rmi`.",
		},
		{
			Feature: "Volume management",
			Apple:   "✅  container volume {create,list,inspect,delete,prune}",
			Podman:  "✅  podman volume {create,ls,inspect,rm,prune}",
			Notes:   "Both support named volumes; `list` vs `ls` convention differs.",
		},
		{
			Feature: "Network management",
			Apple:   "⚠️  macOS 26+ — container network {create,list,inspect,delete}",
			Podman:  "✅  podman network {create,ls,inspect,rm}",
			Notes:   "Apple Container network isolation requires macOS 26.",
		},
		{
			Feature: "Disk usage report",
			Apple:   "✅  container system df",
			Podman:  "✅  podman system df",
			Notes:   "Both report per-type usage; output format differs.",
		},
		{
			Feature: "Copy file to/from container",
			Apple:   "✅  container cp (v1.0.0+)",
			Podman:  "✅  podman cp",
			Notes:   "",
		},
		{
			Feature: "Export container filesystem",
			Apple:   "✅  container export (v1.0.0+)",
			Podman:  "✅  podman export",
			Notes:   "",
		},
		{
			Feature: "Exec into running container",
			Apple:   "✅  container exec",
			Podman:  "✅  podman exec",
			Notes:   "",
		},
		{
			Feature: "Rosetta 2 (x86_64 in container)",
			Apple:   "✅  container run --rosetta",
			Podman:  "❌  not supported natively",
			Notes:   "Apple-only: translate x86_64 instructions on Apple Silicon.",
		},
		{
			Feature: "SSH agent forwarding",
			Apple:   "✅  container run --ssh",
			Podman:  "✅  podman run --ssh (rootless mode)",
			Notes:   "",
		},
		{
			Feature: "Machine management",
			Apple:   "✅  container machine {create,run,list,stop,delete}",
			Podman:  "✅  podman machine {init,start,list,stop,rm}",
			Notes:   "Both support managing the underlying VM.",
		},
	}
}

// CLIMappings returns the command-to-command mapping table.
func CLIMappings() []CLIMapping {
	return []CLIMapping{
		{"Start runtime",          "container system start",         "N/A (daemonless)",            ""},
		{"Stop runtime",           "container system stop",          "N/A (daemonless)",            ""},
		{"Runtime status",         "container system status",        "N/A (daemonless)",            ""},
		{"Runtime version",        "container system version",       "podman version",              ""},
		{"Pull image",             "container image pull <img>",     "podman pull <img>",           ""},
		{"List images",            "container image list",           "podman images",               ""},
		{"Inspect image",          "container image inspect <img>",  "podman inspect --type image", ""},
		{"Remove image",           "container image delete <img>",   "podman rmi <img>",            ""},
		{"Build image",            "container build -t <tag> .",     "podman build -t <tag> .",     ""},
		{"Run container",          "container run <img>",            "podman run <img>",            ""},
		{"Run detached",           "container run -d <img>",         "podman run -d <img>",         ""},
		{"List containers",        "container list / ls",            "podman ps",                   ""},
		{"List all containers",    "container list --all",           "podman ps --all",             ""},
		{"Inspect container",      "container inspect <name>",       "podman inspect <name>",       ""},
		{"Stop container",         "container stop <name>",          "podman stop <name>",          ""},
		{"Delete container",       "container delete <name>",        "podman rm <name>",            ""},
		{"Exec in container",      "container exec <name> <cmd>",    "podman exec <name> <cmd>",    ""},
		{"Container logs",         "container logs <name>",          "podman logs <name>",          ""},
		{"Container stats",        "container stats --no-stream",    "podman stats --no-stream",    ""},
		{"Copy file",              "container cp <name>:<src> <dst>","podman cp <name>:<src> <dst>",""},
		{"Volume create",          "container volume create <v>",    "podman volume create <v>",    ""},
		{"Volume list",            "container volume list",          "podman volume ls",            ""},
		{"Volume inspect",         "container volume inspect <v>",   "podman volume inspect <v>",   ""},
		{"Volume delete",          "container volume delete <v>",    "podman volume rm <v>",        ""},
		{"Network create",         "container network create <n>",   "podman network create <n>",   "macOS 26+ for Apple"},
		{"Network list",           "container network list",         "podman network ls",           ""},
		{"Network inspect",        "container network inspect <n>",  "podman network inspect <n>",  ""},
		{"Disk usage",             "container system df",            "podman system df",            ""},
		{"Prune containers",       "container prune",                "podman container prune",      ""},
		{"Prune images",           "container image prune",          "podman image prune",          ""},
	}
}

// ── Timing collection ─────────────────────────────────────────────────────────

// BenchmarkResult stores timing stats for one operation across multiple runs.
type BenchmarkResult struct {
	OperationName string
	Runtime       RuntimeName
	Iterations    int
	Mean          time.Duration
	Min           time.Duration
	Max           time.Duration
	Errors        int
}

// BenchmarkSummary holds paired Apple Container + Podman results.
type BenchmarkSummary struct {
	Image   string
	Results []BenchmarkPair
}

// BenchmarkPair holds the Apple and Podman result for the same operation.
type BenchmarkPair struct {
	OperationName string
	Apple         BenchmarkResult
	Podman        BenchmarkResult
}

// ComputeStats derives Mean/Min/Max from a slice of durations.
func ComputeStats(op string, runtime RuntimeName, durations []time.Duration, errs int) BenchmarkResult {
	if len(durations) == 0 {
		return BenchmarkResult{OperationName: op, Runtime: runtime, Errors: errs}
	}
	var total time.Duration
	mn := durations[0]
	mx := durations[0]
	for _, d := range durations {
		total += d
		if d < mn {
			mn = d
		}
		if d > mx {
			mx = d
		}
	}
	return BenchmarkResult{
		OperationName: op,
		Runtime:       runtime,
		Iterations:    len(durations),
		Mean:          total / time.Duration(len(durations)),
		Min:           mn,
		Max:           mx,
		Errors:        errs,
	}
}

// ── Educational annotations ───────────────────────────────────────────────────

// AnnotatedOperation pairs a human-readable lesson with the commands.
type AnnotatedOperation struct {
	Title       string
	Lesson      string
	AppleCLI    string
	PodmanCLI   string
}

// EducationalAnnotations returns learning notes for key operations.
func EducationalAnnotations() []AnnotatedOperation {
	return []AnnotatedOperation{
		{
			Title:     "Pulling an image",
			Lesson:    `Both tools download from OCI registries. Apple Container pulls platform-native
layers (arm64 on Apple Silicon); Podman pulls multi-arch manifests and selects
the best match. Apple Container stores images in ~/Library/Application Support/
com.apple.container/; Podman stores them inside the Podman Machine VM.`,
			AppleCLI:  "container image pull alpine:latest",
			PodmanCLI: "podman pull alpine:latest",
		},
		{
			Title:     "Running a container (foreground)",
			Lesson:    `Apple Container boots a new lightweight VM for each container — startup
involves VM initialisation (kernel load, network setup). Podman forks a process
inside the shared Podman Machine VM — startup is faster for the first container
but the Machine VM must already be running. The --rm flag behaves identically.`,
			AppleCLI:  "container run --rm alpine:latest echo hello",
			PodmanCLI: "podman run --rm alpine:latest echo hello",
		},
		{
			Title:     "Inspecting a container",
			Lesson:    `Both tools return JSON. Key differences:
• Apple Container: networks[].address contains the VM IP in CIDR notation
  (e.g. "192.168.64.3/24"). The networks[] array is populated immediately.
• Podman: NetworkSettings.IPAddress is empty for rootless containers;
  you need podman network inspect or podman ps --format for IP details.`,
			AppleCLI:  "container inspect <name>",
			PodmanCLI: "podman inspect <name>",
		},
		{
			Title:     "Stopping a container",
			Lesson:    `Apple Container stop = VM power-off sequence (SIGTERM to init process,
then VM shutdown). Podman stop = SIGTERM to the container PID, then SIGKILL
after --time seconds. Apple Container stop typically takes slightly longer
because of VM teardown overhead.`,
			AppleCLI:  "container stop <name>",
			PodmanCLI: "podman stop <name>",
		},
		{
			Title:     "Runtime lifecycle",
			Lesson:    `Apple Container requires a running API server (launchd service). You must
call "container system start" before any container operation. Podman is
fully daemonless — every CLI call is self-contained. On macOS, Podman does
require the Podman Machine VM to be running ("podman machine start").`,
			AppleCLI:  "container system start  # start apiserver\ncontainer system stop   # stop apiserver",
			PodmanCLI: "podman machine start    # start the underlying Linux VM\npodman machine stop     # stop it",
		},
		{
			Title:     "Rosetta 2 (x86_64 containers on Apple Silicon)",
			Lesson:    `Apple Container supports the --rosetta flag, which enables Apple's
Rosetta 2 translation layer inside the VM. This lets you run x86_64 container
images natively on Apple Silicon without hardware emulation. Podman does not
support Rosetta; running x86_64 images requires the slower QEMU emulation.`,
			AppleCLI:  "container run --rosetta --rm amd64/ubuntu:22.04 uname -m",
			PodmanCLI: "# No direct equivalent; requires --platform linux/amd64 with QEMU",
		},
	}
}

// ── Printer helpers ───────────────────────────────────────────────────────────

// Truncate shortens s to maxLen, adding "…" if truncated.
func Truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

