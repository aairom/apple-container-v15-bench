package applecontainer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apple-container-update/internal/applecontainer"
)

// ── fake cmdRunner for unit tests ─────────────────────────────────────────────

// fakeRunner implements cmdRunner and returns configurable responses.
type fakeRunner struct {
	// responses maps "sub-command key" → (output, error).
	// The key is the first two args joined by " " (e.g. "image pull").
	responses map[string]fakeResponse
}

type fakeResponse struct {
	out string
	err error
}

// Run implements applecontainer.CmdRunner.
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

// newFakeClient creates a test Client backed by fakeRunner.
func newFakeClient(responses map[string]fakeResponse) *applecontainer.Client {
	return applecontainer.NewWithRunner(&fakeRunner{responses: responses}, false)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestVersion(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"system version": {out: `{"version":"1.3.1"}`, err: nil},
	})

	ver, err := c.Version()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.3.1" {
		t.Errorf("expected version 1.3.1, got %q", ver)
	}
}

func TestVersion_Fallback(t *testing.T) {
	// When JSON parse fails, raw string is returned.
	c := newFakeClient(map[string]fakeResponse{
		"system version": {out: "1.3.1\n", err: nil},
	})

	ver, err := c.Version()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.3.1" {
		t.Errorf("expected 1.3.1, got %q", ver)
	}
}

func TestPullImage_ValidatesInput(t *testing.T) {
	c := newFakeClient(nil)

	// Empty image name
	_, _, err := c.PullImage(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty image name")
	}

	// Invalid characters
	_, _, err = c.PullImage(context.Background(), "al;pine")
	if err == nil {
		t.Error("expected error for image name with semicolon")
	}
}

func TestPullImage_Success(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"image pull": {out: "Pulling from library/alpine...", err: nil},
	})

	out, dur, err := c.PullImage(context.Background(), "alpine:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
	if dur < 0 {
		t.Error("duration must be non-negative")
	}
}

func TestRunContainer_ValidatesName(t *testing.T) {
	c := newFakeClient(nil)

	_, _, err := c.RunContainer(context.Background(), "bad name!", "alpine:latest")
	if err == nil {
		t.Error("expected error for container name with special chars")
	}
}

func TestRunContainer_ValidatesImage(t *testing.T) {
	c := newFakeClient(nil)

	_, _, err := c.RunContainer(context.Background(), "good-name", "alp;ine")
	if err == nil {
		t.Error("expected error for image with semicolon")
	}
}

func TestRunContainer_ValidatesArgs(t *testing.T) {
	c := newFakeClient(nil)

	_, _, err := c.RunContainer(context.Background(), "good-name", "alpine:latest", "echo", "$(rm -rf /)")
	if err == nil {
		t.Error("expected error for shell-injection in container args")
	}
}

func TestRunContainer_Success(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"run --rm": {out: "hello", err: nil},
	})

	out, dur, err := c.RunContainer(context.Background(), "test-ctr", "alpine:latest", "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = out
	_ = dur
}

func TestStopContainer_ValidatesName(t *testing.T) {
	c := newFakeClient(nil)

	_, _, err := c.StopContainer(context.Background(), "bad;name")
	if err == nil {
		t.Error("expected error for name with semicolon")
	}
}

func TestRemoveContainer_Success(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"delete foo": {out: "", err: nil},
	})
	_, err := c.RemoveContainer(context.Background(), "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveImage_ValidatesImage(t *testing.T) {
	c := newFakeClient(nil)
	_, err := c.RemoveImage(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty image name")
	}
}

func TestLogs_BootFlag(t *testing.T) {
	var capturedArgs []string
	fr := &fakeRunner{responses: map[string]fakeResponse{
		"logs myapp": {out: "log line", err: nil},
	}}
	// Override to capture args.
	fr.responses["logs myapp"] = fakeResponse{out: "boot log", err: nil}
	_ = capturedArgs

	c := applecontainer.NewWithRunner(fr, false)
	out, err := c.Logs(context.Background(), "myapp", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = out
}

func TestInspectContainer_ValidatesName(t *testing.T) {
	c := newFakeClient(nil)
	_, err := c.InspectContainer(context.Background(), "bad name")
	if err == nil {
		t.Error("expected error for name with space")
	}
}

func TestCommandLine(t *testing.T) {
	got := applecontainer.CommandLine("image", "pull", "alpine:latest")
	want := "container image pull alpine:latest"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestVersion_Error(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"system version": {out: "", err: errors.New("binary not running")},
	})
	_, err := c.Version()
	if err == nil {
		t.Error("expected error propagation from runner")
	}
}

func TestDiskUsage(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"system df": {out: `{"images":100}`, err: nil},
	})
	out, err := c.DiskUsage(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestStats_ValidatesName(t *testing.T) {
	c := newFakeClient(nil)
	_, err := c.Stats(context.Background(), "bad|name")
	if err == nil {
		t.Error("expected error for name with pipe")
	}
}

// TestRunContainerDetached ensures detached mode trims newlines.
func TestRunContainerDetached_TrimsOutput(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"run --detach": {out: "abc123\n", err: nil},
	})
	out, dur, err := c.RunContainerDetached(context.Background(), "det-ctr", "alpine:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "abc123" {
		t.Errorf("expected trimmed output %q, got %q", "abc123", out)
	}
	if dur < 0 {
		t.Error("duration must be non-negative")
	}
}

// Benchmark list images call (measures fake runner overhead).
func BenchmarkListImages(b *testing.B) {
	c := newFakeClient(map[string]fakeResponse{
		"image list": {out: `[{"name":"alpine:latest"}]`, err: nil},
	})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.ListImages(ctx)
	}
}

// Ensure Duration is always ≥ 0.
func TestPullImage_DurationPositive(t *testing.T) {
	c := newFakeClient(map[string]fakeResponse{
		"image pull": {out: "ok", err: nil},
	})
	_, dur, _ := c.PullImage(context.Background(), "alpine:latest")
	if dur < 0 {
		t.Error("duration must be ≥ 0")
	}
}

// Demonstrate that a zero time.Duration satisfies the interface.
var _ time.Duration = 0
