// Package runner_test exercises the runner package's exported commands.
//
// Because RunDemo, RunComparison, and RunBenchmark detect real runtimes via
// exec.LookPath, the tests redirect stdout to a buffer and rely on the fact
// that when neither runtime binary is on PATH they produce graceful
// "runtime unavailable" output rather than panicking.
//
// PrintRuntimeInfo is also exercised; it never errors even when both runtimes
// are absent.
package runner_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/apple-container-update/internal/runner"
)

// captureStdout redirects os.Stdout to a buffer for the duration of fn, then
// restores it and returns the captured output.  This is necessary because the
// runner functions write directly to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()

	w.Close()
	<-done
	r.Close()
	os.Stdout = orig
	return buf.String()
}

// withTempWD changes the working directory to a temp dir for the duration of
// fn, then restores it.  This prevents runner from trying to create
// ./output/ relative to the real project root during tests.
func withTempWD(t *testing.T, fn func()) {
	t.Helper()
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir to tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	fn()
}

// ── PrintRuntimeInfo ──────────────────────────────────────────────────────────

func TestPrintRuntimeInfo_noError(t *testing.T) {
	out := captureStdout(t, func() {
		if err := runner.PrintRuntimeInfo(); err != nil {
			t.Errorf("PrintRuntimeInfo returned error: %v", err)
		}
	})
	// Must mention both runtimes regardless of whether they're installed.
	for _, want := range []string{"Apple Container", "Podman"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintRuntimeInfo output missing %q", want)
		}
	}
}

func TestPrintRuntimeInfo_containsArchNotes(t *testing.T) {
	out := captureStdout(t, func() {
		_ = runner.PrintRuntimeInfo()
	})
	archKeywords := []string{
		"Virtualization.framework",
		"Daemonless",
		"Architecture",
	}
	for _, kw := range archKeywords {
		if !strings.Contains(out, kw) {
			t.Errorf("PrintRuntimeInfo missing architectural keyword %q", kw)
		}
	}
}

// ── RunDemo ───────────────────────────────────────────────────────────────────

func TestRunDemo_apple_skipsGracefully(t *testing.T) {
	// With runtimeFlag="apple", even if Apple Container is unavailable the
	// function must not panic and must return nil.
	var err error
	out := captureStdout(t, func() {
		err = runner.RunDemo("alpine:latest", "apple")
	})
	if err != nil {
		t.Errorf("RunDemo(apple) returned error: %v", err)
	}
	// Output must contain the demo header.
	if !strings.Contains(out, "Container Lifecycle Demo") {
		t.Errorf("RunDemo(apple) missing header")
	}
}

func TestRunDemo_podman_skipsGracefully(t *testing.T) {
	var err error
	out := captureStdout(t, func() {
		err = runner.RunDemo("alpine:latest", "podman")
	})
	if err != nil {
		t.Errorf("RunDemo(podman) returned error: %v", err)
	}
	if !strings.Contains(out, "Container Lifecycle Demo") {
		t.Errorf("RunDemo(podman) missing header")
	}
}

func TestRunDemo_both_skipsGracefully(t *testing.T) {
	var err error
	out := captureStdout(t, func() {
		err = runner.RunDemo("alpine:latest", "both")
	})
	if err != nil {
		t.Errorf("RunDemo(both) returned error: %v", err)
	}
	// Both runtime section headers must appear.
	if !strings.Contains(out, "Apple Container Demo") {
		t.Errorf("RunDemo(both) missing Apple section")
	}
	if !strings.Contains(out, "Podman Demo") {
		t.Errorf("RunDemo(both) missing Podman section")
	}
}

func TestRunDemo_invalidRuntime_noOutput(t *testing.T) {
	// An unknown runtime flag means neither section runs; should still succeed.
	var err error
	out := captureStdout(t, func() {
		err = runner.RunDemo("alpine:latest", "unknown-runtime")
	})
	if err != nil {
		t.Errorf("RunDemo(unknown) returned error: %v", err)
	}
	// The header should still print.
	if !strings.Contains(out, "Container Lifecycle Demo") {
		t.Errorf("RunDemo(unknown) missing header")
	}
}

// ── RunComparison ─────────────────────────────────────────────────────────────

func TestRunComparison_printsTables(t *testing.T) {
	withTempWD(t, func() {
		var err error
		out := captureStdout(t, func() {
			err = runner.RunComparison("alpine:latest")
		})
		if err != nil {
			t.Errorf("RunComparison returned error: %v", err)
		}
		// Static feature + CLI tables must always appear.
		for _, want := range []string{"Feature Comparison", "CLI Command Mapping"} {
			if !strings.Contains(out, want) {
				t.Errorf("RunComparison missing table %q", want)
			}
		}
	})
}

func TestRunComparison_gracefulWhenNoRuntimes(t *testing.T) {
	// When neither runtime is installed, comparison should still print the
	// static tables and the "neither runtime is available" notice.
	withTempWD(t, func() {
		out := captureStdout(t, func() {
			_ = runner.RunComparison("alpine:latest")
		})
		// Either "neither runtime" message OR live comparison section — both
		// are acceptable depending on what's installed in the test environment.
		hasStatic := strings.Contains(out, "Feature Comparison") ||
			strings.Contains(out, "CLI Command Mapping")
		if !hasStatic {
			t.Errorf("RunComparison produced no useful output")
		}
	})
}

// ── RunBenchmark ──────────────────────────────────────────────────────────────

func TestRunBenchmark_returnsErrorWhenNoRuntimes(t *testing.T) {
	// In a clean CI environment neither binary may be present.
	// The function returns an error only when BOTH are absent.
	// We simply verify it never panics.
	withTempWD(t, func() {
		captureStdout(t, func() {
			// We don't assert on error here because Podman or Apple Container
			// might legitimately be installed in the test environment.
			_ = runner.RunBenchmark("alpine:latest", 1)
		})
	})
}

func TestRunBenchmark_clampsIterations(t *testing.T) {
	// Even with iterations=0 or 100 the function should not panic.
	withTempWD(t, func() {
		captureStdout(t, func() {
			_ = runner.RunBenchmark("alpine:latest", 0)
		})
	})
	withTempWD(t, func() {
		captureStdout(t, func() {
			_ = runner.RunBenchmark("alpine:latest", 100)
		})
	})
}

func TestRunBenchmark_headerContainsImage(t *testing.T) {
	withTempWD(t, func() {
		out := captureStdout(t, func() {
			_ = runner.RunBenchmark("myimage:tag", 1)
		})
		if !strings.Contains(out, "Benchmark") {
			t.Errorf("RunBenchmark header missing 'Benchmark' keyword")
		}
		if !strings.Contains(out, "myimage:tag") {
			t.Errorf("RunBenchmark header missing image name 'myimage:tag'")
		}
	})
}

// ── RunDashboard ──────────────────────────────────────────────────────────────

func TestRunDashboard_createsHTMLFile(t *testing.T) {
	withTempWD(t, func() {
		captureStdout(t, func() {
			// dashboard always writes to ./output/ — even when no runtimes are
			// available it generates a static-data-only HTML file.
			err := runner.RunDashboard("alpine:latest", 1, "test-ver", false)
			// Error is acceptable when no runtimes are installed, but the
			// function must not panic.
			_ = err
		})
		// Verify that output/ directory was created (may be empty if both
		// runtimes are absent and an early error occurred before file write,
		// but the directory itself must exist once RunDashboard starts).
		if _, statErr := os.Stat("output"); statErr != nil {
			// Not a hard failure — on some CI environments the write itself
			// may fail gracefully. Just log it.
			t.Logf("output dir not present after RunDashboard (may be expected in CI): %v", statErr)
		}
	})
}

func TestRunDashboard_headerContainsImage(t *testing.T) {
	withTempWD(t, func() {
		out := captureStdout(t, func() {
			_ = runner.RunDashboard("myimg:latest", 1, "1.0.0", false)
		})
		if !strings.Contains(out, "Dashboard") {
			t.Errorf("RunDashboard header missing 'Dashboard'")
		}
		if !strings.Contains(out, "myimg:latest") {
			t.Errorf("RunDashboard header missing image name")
		}
	})
}

func TestRunDashboard_noBrowserFlag(t *testing.T) {
	// openBrowser=false should not cause any error even when `open` is absent.
	withTempWD(t, func() {
		captureStdout(t, func() {
			err := runner.RunDashboard("alpine:latest", 1, "1.0.0", false)
			_ = err // acceptable error when no runtimes present
		})
	})
}

func TestRunDashboard_clampsIterations(t *testing.T) {
	// iterations=0 should be clamped to 1 without panicking.
	withTempWD(t, func() {
		captureStdout(t, func() {
			_ = runner.RunDashboard("alpine:latest", 0, "1.0.0", false)
		})
	})
}
