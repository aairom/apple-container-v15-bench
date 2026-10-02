// Package applecontainer wraps the Apple Container CLI (`container`).
//
// Apple Container (https://github.com/apple/container) is an open-source
// tool from Apple that creates and runs Linux containers as lightweight
// virtual machines on macOS. It is written in Swift and optimised for
// Apple Silicon.
//
// Key architectural traits (v1.3.1):
//   - Each container runs as a dedicated lightweight VM using
//     Virtualization.framework (macOS 26+).
//   - OCI-compatible: pulls/pushes to any standard OCI registry.
//   - CLI convention mirrors Docker/Podman closely.
//   - macOS 26 (Tahoe) required for full network isolation.
//
// Security: all user-supplied strings are validated against strict
// allowlists before use; arguments are passed as a []string to
// exec.Command — no shell is ever invoked.
//
// Validated against Apple Container v1.3.1 (released 2026-08-29).
package applecontainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// imageNameRE allows only characters that appear in valid OCI image refs.
var imageNameRE = regexp.MustCompile(`^[a-zA-Z0-9_./:@-]+$`)

// containerNameRE matches characters Apple Container accepts for names/IDs.
var containerNameRE = regexp.MustCompile(`^[a-zA-Z0-9_.:-]+$`)

// safeArgRE allows only characters that appear in legitimate container
// command arguments. Shell metacharacters are excluded.
var safeArgRE = regexp.MustCompile(`^[a-zA-Z0-9_./:@=,+ -]+$`)

// validateImage returns an error for invalid OCI image references.
func validateImage(image string) error {
	if image == "" {
		return fmt.Errorf("image name must not be empty")
	}
	if !imageNameRE.MatchString(image) {
		return fmt.Errorf("image name %q contains invalid characters", image)
	}
	return nil
}

// validateContainerName returns an error for empty or non-safe names.
func validateContainerName(name string) error {
	if name == "" {
		return fmt.Errorf("container name must not be empty")
	}
	if !containerNameRE.MatchString(name) {
		return fmt.Errorf("container name %q contains invalid characters", name)
	}
	return nil
}

// validateSafeArg rejects strings that could confuse the container binary.
func validateSafeArg(arg string) error {
	if !safeArgRE.MatchString(arg) {
		return fmt.Errorf("argument %q contains disallowed characters", arg)
	}
	return nil
}

// CmdRunner is satisfied by any type that can execute a container sub-command.
// Exported so test packages can provide fake implementations.
type CmdRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// cmdRunner is the internal alias used throughout this package.
type cmdRunner = CmdRunner

// Client wraps the Apple Container CLI.
type Client struct {
	verbose bool
	runner  cmdRunner
}

// New creates a Client, verifying that `container` exists on PATH.
// Returns an error when Apple Container is not installed.
func New(verbose bool) (*Client, error) {
	_, err := exec.LookPath("container")
	if err != nil {
		return nil, fmt.Errorf(
			"apple container binary not found on PATH: %w\n"+
				"  Install: https://github.com/apple/container/releases",
			err,
		)
	}
	return &Client{verbose: verbose, runner: &realRunner{verbose: verbose}}, nil
}

// NewWithRunner creates a Client backed by a custom runner.
// Exported so external test packages can inject fake cmdRunners without
// spawning a real `container` process.
func NewWithRunner(r cmdRunner, verbose bool) *Client {
	return &Client{verbose: verbose, runner: r}
}

// Version returns the installed Apple Container version string.
//
// Equivalent CLI command:
//
//	container system version
func (c *Client) Version() (string, error) {
	out, err := c.runner.Run(context.Background(), "system", "version")
	if err != nil {
		return "", err
	}
	var v struct {
		Version string `json:"version"`
	}
	if jsonErr := json.Unmarshal([]byte(out), &v); jsonErr == nil && v.Version != "" {
		return v.Version, nil
	}
	return strings.TrimSpace(out), nil
}

// PullImage pulls an OCI image from a registry.
//
// Educational note:
//   - On Apple Silicon, Apple Container pulls only the arm64 layer.
//   - Pull progress is streamed to stderr.
//
// Equivalent CLI command:
//
//	container image pull <image>
func (c *Client) PullImage(ctx context.Context, image string) (string, time.Duration, error) {
	if err := validateImage(image); err != nil {
		return "", 0, err
	}
	start := time.Now()
	out, err := c.runner.Run(ctx, "image", "pull", image)
	return out, time.Since(start), err
}

// ListImages returns the local image list as a JSON string.
//
// Equivalent CLI command:
//
//	container image list --format json
func (c *Client) ListImages(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "image", "list", "--format", "json")
}

// RunContainer starts a container and waits for it to exit (foreground).
//
// Apple Container run flags:
//   - `--rm`       → remove container on exit
//   - `--name`     → assign a stable ID
//   - `--cpus`     → CPU count visible to the VM
//   - `--memory`   → RAM allocation (e.g. "512m")
//   - `--rosetta`  → enable Rosetta 2 for x86_64 binaries
//   - `--volume`   → bind-mount a host directory
//
// Equivalent CLI command:
//
//	container run --rm --name <name> <image> [<args>...]
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
//	container run --detach --name <name> <image> [<args>...]
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

// ListContainers returns running containers as JSON.
//
// Apple Container specifics:
//   - Output includes an "ADDR" column showing the VM IP address.
//
// Equivalent CLI command:
//
//	container list [--all] --format json
func (c *Client) ListContainers(ctx context.Context, all bool) (string, error) {
	args := []string{"list", "--format", "json"}
	if all {
		args = append(args, "--all")
	}
	return c.runner.Run(ctx, args...)
}

// InspectContainer returns detailed JSON for a named container.
//
// Key JSON fields:
//   - status                              → "running" | "stopped"
//   - networks[].address                 → VM IP (CIDR notation)
//   - configuration.resources.cpus       → VM CPU count
//   - configuration.resources.memoryInBytes → VM RAM in bytes
//
// Equivalent CLI command:
//
//	container inspect <name>
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
//	container image inspect <image>
func (c *Client) InspectImage(ctx context.Context, image string) (string, error) {
	if err := validateImage(image); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "image", "inspect", image)
}

// StopContainer stops a container (SIGTERM → SIGKILL after timeout).
//
// Educational note:
//   - Stopping a container = powering off its lightweight VM.
//
// Equivalent CLI command:
//
//	container stop <name>
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
//	container delete <name>
func (c *Client) RemoveContainer(ctx context.Context, name string) (string, error) {
	if err := validateContainerName(name); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "delete", name)
}

// RemoveImage removes a local image.
//
// Equivalent CLI command:
//
//	container image delete <image>
func (c *Client) RemoveImage(ctx context.Context, image string) (string, error) {
	if err := validateImage(image); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "image", "delete", image)
}

// Logs returns container stdout/stderr logs.
//
// Apple Container specific: boot=true includes VM boot log.
//
// Equivalent CLI command:
//
//	container logs [--boot] <name>
func (c *Client) Logs(ctx context.Context, name string, boot bool) (string, error) {
	if err := validateContainerName(name); err != nil {
		return "", err
	}
	args := []string{"logs", name}
	if boot {
		args = append(args, "--boot")
	}
	return c.runner.Run(ctx, args...)
}

// Stats returns a one-shot resource stats snapshot.
//
// Equivalent CLI command:
//
//	container stats --no-stream <name>
func (c *Client) Stats(ctx context.Context, name string) (string, error) {
	if err := validateContainerName(name); err != nil {
		return "", err
	}
	return c.runner.Run(ctx, "stats", "--no-stream", name)
}

// SystemStart starts the Apple Container API server (launchd service).
//
// Educational note:
//   - Apple Container runs as com.apple.container.apiserver.
//   - Podman is daemonless — there is no equivalent command.
//
// Equivalent CLI command:
//
//	container system start
func (c *Client) SystemStart(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "system", "start")
}

// SystemStop stops the Apple Container API server.
//
// Equivalent CLI command:
//
//	container system stop
func (c *Client) SystemStop(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "system", "stop")
}

// SystemStatus returns the API server status.
//
// Equivalent CLI command:
//
//	container system status
func (c *Client) SystemStatus(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "system", "status")
}

// DiskUsage reports disk usage of images/containers/volumes.
//
// Equivalent CLI command:
//
//	container system df
func (c *Client) DiskUsage(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "system", "df")
}

// NetworkList lists virtual networks (macOS 26+ only).
//
// Equivalent CLI command:
//
//	container network list --format json
func (c *Client) NetworkList(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "network", "list", "--format", "json")
}

// VolumeList lists named volumes.
//
// Equivalent CLI command:
//
//	container volume list --format json
func (c *Client) VolumeList(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, "volume", "list", "--format", "json")
}

// CommandLine returns the shell command string for educational display.
func CommandLine(args ...string) string {
	return "container " + strings.Join(args, " ")
}

// ── realRunner ────────────────────────────────────────────────────────────────

// realRunner implements cmdRunner against the installed `container` binary.
//
// Security: the executable is referenced as the literal string "container",
// resolved by exec.LookPath at startup. User-supplied data flows only into
// args, which have been validated by the caller before reaching here.
type realRunner struct{ verbose bool }

// Run implements CmdRunner (exported method satisfies the exported interface).
func (r *realRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "container", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if r.verbose {
		fmt.Printf("[apple] container %s\n", strings.Join(args, " "))
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"apple container %q failed: %w\n  stderr: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()),
		)
	}
	out := stdout.String()
	if r.verbose && out != "" {
		fmt.Printf("[apple] output:\n%s\n", out)
	}
	return out, nil
}
