// Package podman wraps the Podman CLI (`podman`).
//
// Podman (https://podman.io) is a daemonless, rootless OCI container engine
// from Red Hat. It is the locally-installed Docker replacement on this machine.
//
// Key architectural traits:
//   - Daemonless: each `podman` invocation is a standalone process.
//   - Rootless by default: containers run as the calling user.
//   - OCI-compatible: uses the same image format and registry protocol.
//   - Uses kernel namespaces + cgroups for isolation (Linux-side).
//   - On macOS, Podman runs inside a Linux VM managed by Podman Machine.
//
// This package mirrors the applecontainer package API so the comparator
// can drive both runtimes with the same interface.
//
// Security: all user-supplied strings are validated against strict
// allowlists before use; no shell expansion takes place.
package podman

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// imageNameRE matches valid OCI image reference characters.
var imageNameRE = regexp.MustCompile(`^[a-zA-Z0-9_./:@-]+$`)

// containerNameRE matches valid container name characters.
var containerNameRE = regexp.MustCompile(`^[a-zA-Z0-9_.:-]+$`)

// safeArgRE allows only characters that appear in legitimate container
// command arguments. Shell metacharacters are excluded.
var safeArgRE = regexp.MustCompile(`^[a-zA-Z0-9_./:@=,+ -]+$`)

func validateImage(image string) error {
	if image == "" {
		return fmt.Errorf("image name must not be empty")
	}
	if !imageNameRE.MatchString(image) {
		return fmt.Errorf("image name %q contains invalid characters", image)
	}
	return nil
}

func validateContainerName(name string) error {
	if name == "" {
		return fmt.Errorf("container name must not be empty")
	}
	if !containerNameRE.MatchString(name) {
		return fmt.Errorf("container name %q contains invalid characters", name)
	}
	return nil
}

func validateSafeArg(arg string) error {
	if !safeArgRE.MatchString(arg) {
		return fmt.Errorf("argument %q contains disallowed characters", arg)
	}
	return nil
}

// CmdRunner executes a podman sub-command.
// Exported so test packages can provide fake implementations.
type CmdRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// cmdRunner is the internal alias.
type cmdRunner = CmdRunner

// Client wraps the Podman CLI.
type Client struct {
	verbose bool
	runner  cmdRunner
}

// New creates a Client, verifying that `podman` exists on PATH.
func New(verbose bool) (*Client, error) {
	_, err := exec.LookPath("podman")
	if err != nil {
		return nil, fmt.Errorf(
			"podman binary not found on PATH: %w\n"+
				"  Install: https://podman.io/getting-started/installation",
			err,
		)
	}
	return &Client{verbose: verbose, runner: &realRunner{verbose: verbose}}, nil
}

// NewWithRunner creates a Client backed by a custom runner.
// Exported so external test packages can inject fake CmdRunners.
func NewWithRunner(r cmdRunner, verbose bool) *Client {
	return &Client{verbose: verbose, runner: r}
}

// Version returns the Podman version string.
//
// Equivalent CLI command:
//
//	podman version --format {{.Client.Version}}
func (c *Client) Version() (string, error) {
	out, err := c.runner.Run(context.Background(), "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// PullImage pulls an OCI image from a registry.
//
// Educational note:
//   - Podman by default pulls the image for the host architecture.
//   - On macOS, images run inside the Podman Machine Linux VM.
//   - Unlike Apple Container, Podman does NOT create a per-container VM;
//     all containers share the same Podman Machine VM.
//
// Equivalent CLI command:
//
//	podman pull <image>
func (c *Client) PullImage(ctx context.Context, image string) (string, time.Duration, error) {
	if err := validateImage(image); err != nil {
		return "", 0, err
	}
	start := time.Now()
	out, err := c.runner.Run(ctx, "pull", image)
	return out, time.Since(start), err
}

// ListImages returns the local image list as JSON.
//
// Equivalent CLI command:
//
//	podman images --format json
func (c *Client) ListImages(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "images", "--format", "json")
}

// RunContainer starts a container and waits for exit (foreground).
//
// Podman run flags compared to Apple Container:
//   - `--rm`    → remove on exit   (same)
//   - `--name`  → assign name      (same)
//   - `--cpus`  → CPU quota        (cgroup-based, unlike Apple Container VM CPU)
//   - `--memory`→ memory limit     (cgroup limit, not VM RAM)
//   - `--userns=keep-id` → rootless UID mapping (Podman-specific)
//
// Equivalent CLI command:
//
//	podman run --rm --name <name> <image> [<args>...]
func (c *Client) RunContainer(
	ctx context.Context,
	name, image string,
	cmdArgs ...string,
) (string, time.Duration, error) {
	if err := validateContainerName(name); err != nil {
		return "", 0, err
	}
	if err := validateImage(image); err != nil {
		return "", 0, err
	}
	for _, a := range cmdArgs {
		if err := validateSafeArg(a); err != nil {
			return "", 0, fmt.Errorf("unsafe container argument: %w", err)
		}
	}
	args := append([]string{"run", "--rm", "--name", name, image}, cmdArgs...)
	start := time.Now()
	out, err := c.runner.Run(ctx, args...)
	return out, time.Since(start), err
}

// RunContainerDetached starts a container in the background.
//
// Equivalent CLI command:
//
//	podman run --detach --name <name> <image> [<args>...]
func (c *Client) RunContainerDetached(
	ctx context.Context,
	name, image string,
	cmdArgs ...string,
) (string, time.Duration, error) {
	if err := validateContainerName(name); err != nil {
		return "", 0, err
	}
	if err := validateImage(image); err != nil {
		return "", 0, err
	}
	for _, a := range cmdArgs {
		if err := validateSafeArg(a); err != nil {
			return "", 0, fmt.Errorf("unsafe container argument: %w", err)
		}
	}
	args := append([]string{"run", "--detach", "--name", name, image}, cmdArgs...)
	start := time.Now()
	out, err := c.runner.Run(ctx, args...)
	return strings.TrimSpace(out), time.Since(start), err
}

// ListContainers returns containers as JSON.
//
// Educational note:
//   - Podman list output does NOT include an IP column by default
//     (unlike Apple Container which always shows VM IPs).
//   - Use `podman inspect` to retrieve IP info for Podman containers.
//
// Equivalent CLI command:
//
//	podman ps [--all] --format json
func (c *Client) ListContainers(ctx context.Context, all bool) (string, error) {
	args := []string{"ps", "--format", "json"}
	if all {
		args = append(args, "--all")
	}
	return c.runner.Run(ctx, args...)
}

// InspectContainer returns detailed JSON for a named container.
//
// Key JSON fields:
//   - State.Status              → "running" | "exited"
//   - NetworkSettings.IPAddress → container IP (bridge network)
//   - HostConfig.NanoCpus       → CPU limit
//   - HostConfig.Memory         → memory limit in bytes
//
// Equivalent CLI command:
//
//	podman inspect <name>
func (c *Client) InspectContainer(ctx context.Context, name string) (string, error) {
	if err := validateContainerName(name); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "inspect", name)
}

// InspectImage returns detailed JSON for a local image.
//
// Equivalent CLI command:
//
//	podman inspect --type image <image>
func (c *Client) InspectImage(ctx context.Context, image string) (string, error) {
	if err := validateImage(image); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "inspect", "--type", "image", image)
}

// StopContainer stops a container gracefully.
//
// Educational note:
//   - Podman sends SIGTERM then SIGKILL (after --time seconds).
//   - Unlike Apple Container, no VM is shut down — only the Linux process.
//
// Equivalent CLI command:
//
//	podman stop <name>
func (c *Client) StopContainer(ctx context.Context, name string) (string, time.Duration, error) {
	if err := validateContainerName(name); err != nil {
		return "", 0, err
	}
	start := time.Now()
	out, err := c.runner.Run(ctx, "stop", name)
	return out, time.Since(start), err
}

// RemoveContainer deletes a stopped container.
//
// Equivalent CLI command:
//
//	podman rm <name>
func (c *Client) RemoveContainer(ctx context.Context, name string) (string, error) {
	if err := validateContainerName(name); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "rm", name)
}

// RemoveImage removes a local image.
//
// Equivalent CLI command:
//
//	podman rmi <image>
func (c *Client) RemoveImage(ctx context.Context, image string) (string, error) {
	if err := validateImage(image); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "rmi", image)
}

// Logs returns container stdout/stderr logs.
//
// Educational note:
//   - Podman does not have a --boot flag (no VM boot logs).
//   - Boot parameter is accepted for API compatibility but ignored.
//
// Equivalent CLI command:
//
//	podman logs <name>
func (c *Client) Logs(ctx context.Context, name string, _ bool) (string, error) {
	if err := validateContainerName(name); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "logs", name)
}

// Stats returns a one-shot resource stats snapshot.
//
// Educational note:
//   - `podman stats --no-stream` returns one sample then exits.
//   - Output format: CONTAINER  CPU%  MEM USAGE/LIMIT  NET I/O  BLOCK I/O
//
// Equivalent CLI command:
//
//	podman stats --no-stream <name>
func (c *Client) Stats(ctx context.Context, name string) (string, error) {
	if err := validateContainerName(name); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "stats", "--no-stream", name)
}

// SystemInfo returns Podman system information.
//
// Educational note:
//   - Podman has no "system start/stop" — it is daemonless.
//   - `podman system info` shows host + store + registry info.
//
// Equivalent CLI command:
//
//	podman system info --format json
func (c *Client) SystemInfo(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "system", "info", "--format", "json")
}

// DiskUsage reports disk usage of images/containers/volumes.
//
// Equivalent CLI command:
//
//	podman system df --format json
func (c *Client) DiskUsage(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "system", "df", "--format", "json")
}

// NetworkList lists container networks.
//
// Equivalent CLI command:
//
//	podman network ls --format json
func (c *Client) NetworkList(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "network", "ls", "--format", "json")
}

// VolumeList lists named volumes.
//
// Equivalent CLI command:
//
//	podman volume ls --format json
func (c *Client) VolumeList(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "volume", "ls", "--format", "json")
}

// CommandLine returns the shell command string for educational display.
func CommandLine(args ...string) string {
	return "podman " + strings.Join(args, " ")
}

// ── realRunner ────────────────────────────────────────────────────────────────

// realRunner implements cmdRunner using the installed `podman` binary.
//
// Security: the executable is the literal string "podman", resolved by the
// OS PATH at runtime. User-supplied data flows only into args, which are
// validated before reaching here. No shell is invoked.
type realRunner struct{ verbose bool }

// Run implements CmdRunner.
func (r *realRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if r.verbose {
		fmt.Printf("[podman] podman %s\n", strings.Join(args, " "))
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"podman %q failed: %w\n  stderr: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()),
		)
	}
	out := stdout.String()
	if r.verbose && out != "" {
		fmt.Printf("[podman] output:\n%s\n", out)
	}
	return out, nil
}
