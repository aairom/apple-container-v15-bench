package podman_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/apple-container-update/internal/podman"
)

// ── fake cmdRunner ────────────────────────────────────────────────────────────

type fakeRunner struct {
	responses map[string]fakeResponse
}

type fakeResponse struct {
	out string
	err error
}

// Run implements podman.CmdRunner.
func (f *fakeRunner) Run(_ context.Context, args ...string) (string, error) {
	key := ""
	if len(args) >= 2 {
		key = args[0] + " " + args[1]
	} else if len(args) == 1 {
		key = args[0]
	}
	if r, ok := f.responses[key]; ok {
		return r.out, r.err
	}
	return "", nil
}

func newFake(responses map[string]fakeResponse) *podman.Client {
	return podman.NewWithRunner(&fakeRunner{responses: responses}, false)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestVersion(t *testing.T) {
	c := newFake(map[string]fakeResponse{
		"version --format": {out: "5.2.3\n", err: nil},
	})
	ver, err := c.Version()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "5.2.3" {
		t.Errorf("expected 5.2.3, got %q", ver)
	}
}

func TestVersion_Error(t *testing.T) {
	c := newFake(map[string]fakeResponse{
		"version --format": {out: "", err: errors.New("not running")},
	})
	_, err := c.Version()
	if err == nil {
		t.Error("expected error to propagate")
	}
}

func TestPullImage_ValidatesEmpty(t *testing.T) {
	c := newFake(nil)
	_, _, err := c.PullImage(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty image name")
	}
}

func TestPullImage_ValidatesChars(t *testing.T) {
	c := newFake(nil)
	_, _, err := c.PullImage(context.Background(), "alpine; rm -rf /")
	if err == nil {
		t.Error("expected error for shell-injection in image name")
	}
}

func TestPullImage_Success(t *testing.T) {
	c := newFake(map[string]fakeResponse{
		"pull alpine:latest": {out: "Trying to pull ...", err: nil},
	})
	out, dur, err := c.PullImage(context.Background(), "alpine:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = out
	if dur < 0 {
		t.Error("duration must be non-negative")
	}
}

func TestRunContainer_ValidatesContainerName(t *testing.T) {
	c := newFake(nil)
	_, _, err := c.RunContainer(context.Background(), "bad name!", "alpine:latest")
	if err == nil {
		t.Error("expected error for container name with special chars")
	}
}

func TestRunContainer_ValidatesImage(t *testing.T) {
	c := newFake(nil)
	_, _, err := c.RunContainer(context.Background(), "good-name", "al|pine")
	if err == nil {
		t.Error("expected error for image with pipe character")
	}
}

func TestRunContainer_ValidatesCmdArgs(t *testing.T) {
	c := newFake(nil)
	_, _, err := c.RunContainer(context.Background(), "good-name", "alpine:latest", "sh", "-c", "$(evil)")
	if err == nil {
		t.Error("expected error for shell-injection in cmd args")
	}
}

func TestRunContainerDetached_TrimsOutput(t *testing.T) {
	c := newFake(map[string]fakeResponse{
		"run --detach": {out: "containerid123\n\n", err: nil},
	})
	out, _, err := c.RunContainerDetached(context.Background(), "my-ctr", "alpine:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "containerid123" {
		t.Errorf("expected trimmed output, got %q", out)
	}
}

func TestListContainers_All(t *testing.T) {
	c := newFake(map[string]fakeResponse{
		"ps --format": {out: `[{"Id":"abc"}]`, err: nil},
	})
	out, err := c.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "abc") {
		t.Errorf("expected output to contain container id, got %q", out)
	}
}

func TestInspectContainer_ValidatesName(t *testing.T) {
	c := newFake(nil)
	_, err := c.InspectContainer(context.Background(), "bad name")
	if err == nil {
		t.Error("expected error for name with space")
	}
}

func TestStopContainer_ValidatesName(t *testing.T) {
	c := newFake(nil)
	_, _, err := c.StopContainer(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRemoveContainer_Success(t *testing.T) {
	c := newFake(map[string]fakeResponse{
		"rm my-ctr": {out: "my-ctr", err: nil},
	})
	_, err := c.RemoveContainer(context.Background(), "my-ctr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveImage_ValidatesImage(t *testing.T) {
	c := newFake(nil)
	_, err := c.RemoveImage(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty image")
	}
}

func TestLogs_IgnoresBootFlag(t *testing.T) {
	// Podman ignores the boot parameter (no VM boot logs).
	c := newFake(map[string]fakeResponse{
		"logs my-app": {out: "log output", err: nil},
	})
	out, err := c.Logs(context.Background(), "my-app", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = out
}

func TestStats_ValidatesName(t *testing.T) {
	c := newFake(nil)
	_, err := c.Stats(context.Background(), "bad|name")
	if err == nil {
		t.Error("expected error for name with pipe")
	}
}

func TestCommandLine(t *testing.T) {
	got := podman.CommandLine("run", "--rm", "alpine:latest")
	want := "podman run --rm alpine:latest"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDiskUsage(t *testing.T) {
	c := newFake(map[string]fakeResponse{
		"system df": {out: `{"Images":{"Total":3}}`, err: nil},
	})
	out, err := c.DiskUsage(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}
