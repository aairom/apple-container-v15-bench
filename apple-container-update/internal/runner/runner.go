// Package runner implements the five top-level commands:
//   - PrintRuntimeInfo  → detect + display runtime versions
//   - RunDemo           → walk through the container lifecycle step by step
//   - RunComparison     → side-by-side operations on both runtimes
//   - RunBenchmark      → timing benchmarks across both runtimes
//   - RunDashboard      → generate self-contained HTML visual dashboard
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/apple-container-update/internal/applecontainer"
	"github.com/apple-container-update/internal/comparator"
	"github.com/apple-container-update/internal/podman"
	"github.com/apple-container-update/internal/reporter"
)

// ─────────────────────────────────────────────────────────────────────────────
// info
// ─────────────────────────────────────────────────────────────────────────────

// PrintRuntimeInfo detects installed runtimes and prints a summary.
func PrintRuntimeInfo() error {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("  Container Runtime Detection")
	fmt.Println("═══════════════════════════════════════════════════════════════════")

	// ── Apple Container ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("🍎  Apple Container")
	ac, acErr := applecontainer.New(false)
	if acErr != nil {
		fmt.Printf("    ❌  Not installed or not on PATH\n    %v\n", acErr)
	} else {
		ver, verErr := ac.Version()
		if verErr != nil {
			fmt.Printf("    ⚠️   Installed but could not read version: %v\n", verErr)
		} else {
			fmt.Printf("    ✅  Version: %s\n", ver)
		}
		status, statusErr := ac.SystemStatus(context.Background())
		if statusErr != nil {
			fmt.Printf("    ⚠️   System status: %v\n", statusErr)
		} else {
			fmt.Printf("    🔵  System status: %s\n", strings.TrimSpace(status))
		}
	}
	fmt.Println()
	fmt.Println("    Architecture notes:")
	fmt.Println("    • Each container runs as an isolated lightweight VM")
	fmt.Println("    • Virtualization.framework (macOS 26+) / Apple Silicon")
	fmt.Println("    • API server: com.apple.container.apiserver (launchd)")
	fmt.Println("    • GitHub: https://github.com/apple/container")

	// ── Podman ───────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("🦭  Podman")
	pm, pmErr := podman.New(false)
	if pmErr != nil {
		fmt.Printf("    ❌  Not installed or not on PATH\n    %v\n", pmErr)
	} else {
		ver, verErr := pm.Version()
		if verErr != nil {
			fmt.Printf("    ⚠️   Installed but could not read version: %v\n", verErr)
		} else {
			fmt.Printf("    ✅  Version: %s\n", ver)
		}
		info, infoErr := pm.SystemInfo(context.Background())
		if infoErr != nil {
			fmt.Printf("    ⚠️   System info: %v\n", infoErr)
		} else {
			lines := strings.Split(strings.TrimSpace(info), "\n")
			if len(lines) > 0 {
				fmt.Printf("    🔵  System info available (%d bytes)\n", len(info))
				_ = lines
			}
		}
	}
	fmt.Println()
	fmt.Println("    Architecture notes:")
	fmt.Println("    • Daemonless — every invocation is a standalone process")
	fmt.Println("    • On macOS runs inside Podman Machine Linux VM")
	fmt.Println("    • Rootless by default; uses kernel namespaces + cgroups")
	fmt.Println("    • Homepage: https://podman.io")

	// ── Comparison tables ────────────────────────────────────────────────
	fmt.Println()
	reporter.PrintArchTable(os.Stdout, comparator.ArchComparison())
	reporter.PrintAnnotations(os.Stdout, comparator.EducationalAnnotations())

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// demo
// ─────────────────────────────────────────────────────────────────────────────

// RunDemo walks through the container lifecycle for one or both runtimes.
func RunDemo(image, runtimeFlag string) error {
	runApple := runtimeFlag == "apple" || runtimeFlag == "both"
	runPodman := runtimeFlag == "podman" || runtimeFlag == "both"

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  Container Lifecycle Demo   image=%s\n", image)
	fmt.Println("═══════════════════════════════════════════════════════════════════")

	ctx := context.Background()

	if runApple {
		fmt.Println()
		fmt.Println("── 🍎  Apple Container Demo ────────────────────────────────────────")
		ac, err := applecontainer.New(true)
		if err != nil {
			fmt.Printf("⚠️   Skipping Apple Container: %v\n", err)
		} else {
			runAppleDemo(ctx, ac, image)
		}
	}

	if runPodman {
		fmt.Println()
		fmt.Println("── 🦭  Podman Demo ──────────────────────────────────────────────────")
		pm, err := podman.New(true)
		if err != nil {
			fmt.Printf("⚠️   Skipping Podman: %v\n", err)
		} else {
			runPodmanDemo(ctx, pm, image)
		}
	}

	fmt.Println()
	reporter.PrintAnnotations(os.Stdout, comparator.EducationalAnnotations())
	return nil
}

// runAppleDemo executes the lifecycle steps on Apple Container.
func runAppleDemo(ctx context.Context, ac *applecontainer.Client, image string) {
	demoStep := func(n int, title, cmd string) {
		fmt.Printf("\n  [Step %d] %s\n  → %s\n", n, title, cmd)
	}

	// Step 1 – Pull
	demoStep(1, "Pull image", applecontainer.CommandLine("image", "pull", image))
	out, dur, err := ac.PullImage(ctx, image)
	printResult(out, dur, err)

	// Step 2 – Run (foreground, echo)
	name := "demo-apple-echo"
	demoStep(2, "Run container (foreground)",
		applecontainer.CommandLine("run", "--rm", "--name", name, image, "echo", "hello from Apple Container"))
	out, dur, err = ac.RunContainer(ctx, name, image, "echo", "hello from Apple Container")
	printResult(out, dur, err)

	// Step 3 – Run detached (httpd)
	bgName := "demo-apple-httpd"
	demoStep(3, "Run container (background)",
		applecontainer.CommandLine("run", "--detach", "--name", bgName, "httpd:alpine"))
	out, dur, err = ac.RunContainerDetached(ctx, bgName, "httpd:alpine")
	printResult(out, dur, err)

	// Step 4 – List containers
	demoStep(4, "List running containers",
		applecontainer.CommandLine("list", "--format", "json"))
	out, err = ac.ListContainers(ctx, false)
	printJSON(out, err, 400)

	// Step 5 – Inspect
	demoStep(5, "Inspect container",
		applecontainer.CommandLine("inspect", bgName))
	out, err = ac.InspectContainer(ctx, bgName)
	printJSON(out, err, 600)

	// Step 6 – Logs
	demoStep(6, "Fetch container logs",
		applecontainer.CommandLine("logs", bgName))
	out, err = ac.Logs(ctx, bgName, false)
	printResult(out, 0, err)

	// Step 7 – Stop
	demoStep(7, "Stop container",
		applecontainer.CommandLine("stop", bgName))
	out, dur, err = ac.StopContainer(ctx, bgName)
	printResult(out, dur, err)

	// Step 8 – Remove container
	demoStep(8, "Remove container",
		applecontainer.CommandLine("delete", bgName))
	_, removeErr := ac.RemoveContainer(ctx, bgName)
	if removeErr != nil {
		fmt.Printf("  ⚠️  %v\n", removeErr)
	} else {
		fmt.Println("  ✅  Removed")
	}

	// Step 9 – List images
	demoStep(9, "List images",
		applecontainer.CommandLine("image", "list", "--format", "json"))
	out, err = ac.ListImages(ctx)
	printJSON(out, err, 400)

	fmt.Println("\n  ✅  Apple Container demo complete")
}

// runPodmanDemo executes the lifecycle steps on Podman.
func runPodmanDemo(ctx context.Context, pm *podman.Client, image string) {
	demoStep := func(n int, title, cmd string) {
		fmt.Printf("\n  [Step %d] %s\n  → %s\n", n, title, cmd)
	}

	// Step 1 – Pull
	demoStep(1, "Pull image", podman.CommandLine("pull", image))
	out, dur, err := pm.PullImage(ctx, image)
	printResult(out, dur, err)

	// Step 2 – Run (foreground, echo)
	name := "demo-podman-echo"
	demoStep(2, "Run container (foreground)",
		podman.CommandLine("run", "--rm", "--name", name, image, "echo", "hello from Podman"))
	out, dur, err = pm.RunContainer(ctx, name, image, "echo", "hello from Podman")
	printResult(out, dur, err)

	// Step 3 – Run detached (httpd)
	bgName := "demo-podman-httpd"
	demoStep(3, "Run container (background)",
		podman.CommandLine("run", "--detach", "--name", bgName, "httpd:alpine"))
	out, dur, err = pm.RunContainerDetached(ctx, bgName, "httpd:alpine")
	printResult(out, dur, err)

	// Step 4 – List containers
	demoStep(4, "List running containers",
		podman.CommandLine("ps", "--format", "json"))
	out, err = pm.ListContainers(ctx, false)
	printJSON(out, err, 400)

	// Step 5 – Inspect
	demoStep(5, "Inspect container",
		podman.CommandLine("inspect", bgName))
	out, err = pm.InspectContainer(ctx, bgName)
	printJSON(out, err, 600)

	// Step 6 – Logs
	demoStep(6, "Fetch container logs",
		podman.CommandLine("logs", bgName))
	out, err = pm.Logs(ctx, bgName, false)
	printResult(out, 0, err)

	// Step 7 – Stop
	demoStep(7, "Stop container",
		podman.CommandLine("stop", bgName))
	out, dur, err = pm.StopContainer(ctx, bgName)
	printResult(out, dur, err)

	// Step 8 – Remove
	demoStep(8, "Remove container",
		podman.CommandLine("rm", bgName))
	_, removeErr := pm.RemoveContainer(ctx, bgName)
	if removeErr != nil {
		fmt.Printf("  ⚠️  %v\n", removeErr)
	} else {
		fmt.Println("  ✅  Removed")
	}

	// Step 9 – List images
	demoStep(9, "List images",
		podman.CommandLine("images", "--format", "json"))
	out, err = pm.ListImages(ctx)
	printJSON(out, err, 400)

	fmt.Println("\n  ✅  Podman demo complete")
}

// ─────────────────────────────────────────────────────────────────────────────
// compare
// ─────────────────────────────────────────────────────────────────────────────

// RunComparison executes the same operations on both runtimes and prints
// a side-by-side table plus static comparison matrices.
func RunComparison(image string) error {
	ctx := context.Background()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  Side-by-side Comparison   image=%s\n", image)
	fmt.Println("═══════════════════════════════════════════════════════════════════")

	// Print static tables first.
	reporter.PrintFeatureTable(os.Stdout, comparator.FeatureComparison())
	reporter.PrintCLITable(os.Stdout, comparator.CLIMappings())

	// Attempt live comparison.
	ac, acErr := applecontainer.New(false)
	pm, pmErr := podman.New(false)

	if acErr != nil && pmErr != nil {
		fmt.Println("\n⚠️   Neither runtime is available — showing static tables only.")
	} else {
		fmt.Println("\n📊  Running live operations on available runtimes...")
		rows := runLiveComparison(ctx, ac, acErr, pm, pmErr, image)
		reporter.PrintComparisonRows(os.Stdout, rows)

		// Write Markdown report.
		path, writeErr := reporter.WriteMarkdownReport(
			comparator.ArchComparison(),
			comparator.FeatureComparison(),
			comparator.CLIMappings(),
			rows,
			comparator.EducationalAnnotations(),
		)
		if writeErr != nil {
			fmt.Printf("\n⚠️   Could not write report: %v\n", writeErr)
		} else {
			fmt.Printf("\n📄  Report saved: %s\n", path)
		}
	}

	return nil
}

// runLiveComparison executes pull + run + inspect + stop on both runtimes.
func runLiveComparison(
	ctx context.Context,
	ac *applecontainer.Client, acErr error,
	pm *podman.Client, pmErr error,
	image string,
) []comparator.ComparisonRow {
	var rows []comparator.ComparisonRow

	// Helper to produce a skipped result.
	skipped := func(runtime comparator.RuntimeName, op, cmd string, err error) comparator.OperationResult {
		return comparator.OperationResult{
			Runtime:       runtime,
			OperationName: op,
			Command:       cmd,
			Err:           err,
		}
	}

	// ── Pull ─────────────────────────────────────────────────────────────
	appleCmd := applecontainer.CommandLine("image", "pull", image)
	podmanCmd := podman.CommandLine("pull", image)

	var appleRes, podmanRes comparator.OperationResult
	if acErr != nil {
		appleRes = skipped(comparator.AppleContainer, "Pull image", appleCmd, acErr)
	} else {
		appleRes = timed(ctx, comparator.AppleContainer, "Pull image", appleCmd, func() (string, error) {
			out, _, err := ac.PullImage(ctx, image)
			return out, err
		})
	}
	if pmErr != nil {
		podmanRes = skipped(comparator.Podman, "Pull image", podmanCmd, pmErr)
	} else {
		podmanRes = timed(ctx, comparator.Podman, "Pull image", podmanCmd, func() (string, error) {
			out, _, err := pm.PullImage(ctx, image)
			return out, err
		})
	}
	rows = append(rows, comparator.ComparisonRow{
		OperationName: "Pull image",
		Apple:         appleRes,
		Podman:        podmanRes,
	})

	// ── Run (foreground) ─────────────────────────────────────────────────
	appleRunCmd := applecontainer.CommandLine("run", "--rm", "--name", "cmp-apple", image, "echo", "hello")
	podmanRunCmd := podman.CommandLine("run", "--rm", "--name", "cmp-podman", image, "echo", "hello")

	if acErr != nil {
		appleRes = skipped(comparator.AppleContainer, "Run (echo)", appleRunCmd, acErr)
	} else {
		appleRes = timed(ctx, comparator.AppleContainer, "Run (echo)", appleRunCmd, func() (string, error) {
			out, _, err := ac.RunContainer(ctx, "cmp-apple", image, "echo", "hello")
			return out, err
		})
	}
	if pmErr != nil {
		podmanRes = skipped(comparator.Podman, "Run (echo)", podmanRunCmd, pmErr)
	} else {
		podmanRes = timed(ctx, comparator.Podman, "Run (echo)", podmanRunCmd, func() (string, error) {
			out, _, err := pm.RunContainer(ctx, "cmp-podman", image, "echo", "hello")
			return out, err
		})
	}
	rows = append(rows, comparator.ComparisonRow{
		OperationName: "Run (echo)",
		Apple:         appleRes,
		Podman:        podmanRes,
	})

	// ── List images ───────────────────────────────────────────────────────
	appleListCmd := applecontainer.CommandLine("image", "list", "--format", "json")
	podmanListCmd := podman.CommandLine("images", "--format", "json")

	if acErr != nil {
		appleRes = skipped(comparator.AppleContainer, "List images", appleListCmd, acErr)
	} else {
		appleRes = timed(ctx, comparator.AppleContainer, "List images", appleListCmd, func() (string, error) {
			return ac.ListImages(ctx)
		})
	}
	if pmErr != nil {
		podmanRes = skipped(comparator.Podman, "List images", podmanListCmd, pmErr)
	} else {
		podmanRes = timed(ctx, comparator.Podman, "List images", podmanListCmd, func() (string, error) {
			return pm.ListImages(ctx)
		})
	}
	rows = append(rows, comparator.ComparisonRow{
		OperationName: "List images",
		Apple:         appleRes,
		Podman:        podmanRes,
	})

	return rows
}

// ─────────────────────────────────────────────────────────────────────────────
// benchmark
// ─────────────────────────────────────────────────────────────────────────────

// RunBenchmark times the same operations N times on each runtime.
func RunBenchmark(image string, iterations int) error {
	if iterations < 1 {
		iterations = 1
	}
	if iterations > 10 {
		iterations = 10
	}

	ctx := context.Background()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  Benchmark   image=%s   iterations=%d\n", image, iterations)
	fmt.Println("═══════════════════════════════════════════════════════════════════")

	ac, acErr := applecontainer.New(false)
	pm, pmErr := podman.New(false)

	if acErr != nil && pmErr != nil {
		return fmt.Errorf("neither runtime is available: apple=%v, podman=%v", acErr, pmErr)
	}

	summary := comparator.BenchmarkSummary{Image: image}

	// ── Benchmark: Run (echo) ────────────────────────────────────────────
	fmt.Printf("\n  Benchmarking 'run echo' × %d iterations ...\n", iterations)

	appleRunDurations := benchOp(iterations, func(i int) (time.Duration, error) {
		if acErr != nil {
			return 0, acErr
		}
		name := fmt.Sprintf("bench-apple-%d", i)
		_, dur, err := ac.RunContainer(ctx, name, image, "echo", "benchmark")
		return dur, err
	})
	podmanRunDurations := benchOp(iterations, func(i int) (time.Duration, error) {
		if pmErr != nil {
			return 0, pmErr
		}
		name := fmt.Sprintf("bench-podman-%d", i)
		_, dur, err := pm.RunContainer(ctx, name, image, "echo", "benchmark")
		return dur, err
	})

	summary.Results = append(summary.Results, comparator.BenchmarkPair{
		OperationName: "Run echo",
		Apple:         comparator.ComputeStats("Run echo", comparator.AppleContainer, appleRunDurations.durations, appleRunDurations.errors),
		Podman:        comparator.ComputeStats("Run echo", comparator.Podman, podmanRunDurations.durations, podmanRunDurations.errors),
	})

	// ── Benchmark: List images ────────────────────────────────────────────
	fmt.Printf("  Benchmarking 'list images' × %d iterations ...\n", iterations)

	appleListDurations := benchOp(iterations, func(_ int) (time.Duration, error) {
		if acErr != nil {
			return 0, acErr
		}
		t := time.Now()
		_, err := ac.ListImages(ctx)
		return time.Since(t), err
	})
	podmanListDurations := benchOp(iterations, func(_ int) (time.Duration, error) {
		if pmErr != nil {
			return 0, pmErr
		}
		t := time.Now()
		_, err := pm.ListImages(ctx)
		return time.Since(t), err
	})

	summary.Results = append(summary.Results, comparator.BenchmarkPair{
		OperationName: "List images",
		Apple:         comparator.ComputeStats("List images", comparator.AppleContainer, appleListDurations.durations, appleListDurations.errors),
		Podman:        comparator.ComputeStats("List images", comparator.Podman, podmanListDurations.durations, podmanListDurations.errors),
	})

	// ── Print + save results ─────────────────────────────────────────────
	reporter.PrintBenchmarkTable(os.Stdout, summary)

	path, writeErr := reporter.WriteJSONBenchmark(summary)
	if writeErr != nil {
		fmt.Printf("\n⚠️   Could not write JSON: %v\n", writeErr)
	} else {
		fmt.Printf("\n📄  JSON saved: %s\n", path)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// dashboard
// ─────────────────────────────────────────────────────────────────────────────

// RunDashboard generates a self-contained HTML visual dashboard by running the
// same comparison and benchmark logic as RunComparison + RunBenchmark, then
// writing a single timestamped HTML file to ./output/.
//
// If openBrowser is true and the `open` binary is available (macOS), the
// dashboard is opened automatically in the default browser.
func RunDashboard(image string, iterations int, toolVersion string, openBrowser bool) error {
	if iterations < 1 {
		iterations = 1
	}
	if iterations > 10 {
		iterations = 10
	}

	ctx := context.Background()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  Building Visual Dashboard   image=%s\n", image)
	fmt.Println("═══════════════════════════════════════════════════════════════════")

	// Detect runtimes.
	ac, acErr := applecontainer.New(false)
	pm, pmErr := podman.New(false)

	data := reporter.DashboardData{
		GeneratedAt:     time.Now(),
		ToolVersion:     toolVersion,
		ArchRows:        comparator.ArchComparison(),
		FeatureRows:     comparator.FeatureComparison(),
		CLIRows:         comparator.CLIMappings(),
		Annotations:     comparator.EducationalAnnotations(),
		AppleAvailable:  acErr == nil,
		PodmanAvailable: pmErr == nil,
	}

	// Live comparison rows (best-effort).
	if acErr == nil || pmErr == nil {
		fmt.Println("\n  Running live comparison operations...")
		data.CompRows = runLiveComparison(ctx, ac, acErr, pm, pmErr, image)
	} else {
		fmt.Println("\n  ⚠️  No runtimes available — dashboard will show static data only.")
	}

	// Benchmark (best-effort).
	if acErr == nil || pmErr == nil {
		fmt.Printf("  Running benchmark × %d iterations...\n", iterations)
		summary := comparator.BenchmarkSummary{Image: image}

		appleRunD := benchOp(iterations, func(i int) (time.Duration, error) {
			if acErr != nil {
				return 0, acErr
			}
			name := fmt.Sprintf("dash-apple-%d", i)
			_, dur, err := ac.RunContainer(ctx, name, image, "echo", "benchmark")
			return dur, err
		})
		podmanRunD := benchOp(iterations, func(i int) (time.Duration, error) {
			if pmErr != nil {
				return 0, pmErr
			}
			name := fmt.Sprintf("dash-podman-%d", i)
			_, dur, err := pm.RunContainer(ctx, name, image, "echo", "benchmark")
			return dur, err
		})
		summary.Results = append(summary.Results, comparator.BenchmarkPair{
			OperationName: "Run echo",
			Apple:         comparator.ComputeStats("Run echo", comparator.AppleContainer, appleRunD.durations, appleRunD.errors),
			Podman:        comparator.ComputeStats("Run echo", comparator.Podman, podmanRunD.durations, podmanRunD.errors),
		})

		appleListD := benchOp(iterations, func(_ int) (time.Duration, error) {
			if acErr != nil {
				return 0, acErr
			}
			t := time.Now()
			_, err := ac.ListImages(ctx)
			return time.Since(t), err
		})
		podmanListD := benchOp(iterations, func(_ int) (time.Duration, error) {
			if pmErr != nil {
				return 0, pmErr
			}
			t := time.Now()
			_, err := pm.ListImages(ctx)
			return time.Since(t), err
		})
		summary.Results = append(summary.Results, comparator.BenchmarkPair{
			OperationName: "List images",
			Apple:         comparator.ComputeStats("List images", comparator.AppleContainer, appleListD.durations, appleListD.errors),
			Podman:        comparator.ComputeStats("List images", comparator.Podman, podmanListD.durations, podmanListD.errors),
		})

		data.Benchmark = &summary
	}

	// Write HTML.
	fmt.Println("\n  Generating HTML dashboard...")
	path, err := reporter.WriteHTMLDashboard(data)
	if err != nil {
		return fmt.Errorf("write dashboard: %w", err)
	}
	fmt.Printf("\n  ✅  Dashboard saved: %s\n", path)

	// Open in browser (macOS).
	if openBrowser {
		openCmd := exec.CommandContext(ctx, "open", path) //nolint:gosec // path is reporter-generated, not user input
		if openErr := openCmd.Run(); openErr != nil {
			fmt.Printf("  ⚠️  Could not open browser automatically: %v\n", openErr)
			fmt.Printf("  👉  Open manually: open %s\n", path)
		} else {
			fmt.Println("  🌐  Opened in default browser.")
		}
	} else {
		fmt.Printf("  👉  Open with: open %s\n", path)
	}

	return nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// benchResults holds durations and error count from a benchmark loop.
type benchResults struct {
	durations []time.Duration
	errors    int
}

// benchOp runs fn n times, collecting durations.
func benchOp(n int, fn func(i int) (time.Duration, error)) benchResults {
	r := benchResults{}
	for i := 0; i < n; i++ {
		d, err := fn(i)
		if err != nil {
			r.errors++
		} else {
			r.durations = append(r.durations, d)
		}
	}
	return r
}

// timed executes fn, measures elapsed time, and returns an OperationResult.
func timed(
	_ context.Context,
	runtime comparator.RuntimeName,
	op, cmd string,
	fn func() (string, error),
) comparator.OperationResult {
	start := time.Now()
	out, err := fn()
	dur := time.Since(start)
	return comparator.OperationResult{
		Runtime:       runtime,
		OperationName: op,
		Command:       cmd,
		Duration:      dur,
		Output:        comparator.Truncate(out, 120),
		Err:           err,
	}
}

// printResult prints a duration + excerpt of output or an error.
func printResult(out string, dur time.Duration, err error) {
	if err != nil {
		fmt.Printf("  ❌  %v\n", err)
		return
	}
	if dur > 0 {
		fmt.Printf("  ✅  done in %s\n", dur.Round(time.Millisecond))
	} else {
		fmt.Println("  ✅  done")
	}
	if out != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, l := range lines {
			if l != "" {
				fmt.Printf("  │  %s\n", l)
			}
		}
	}
}

// printJSON prints the first maxBytes of a JSON string.
func printJSON(out string, err error, maxBytes int) {
	if err != nil {
		fmt.Printf("  ❌  %v\n", err)
		return
	}
	excerpt := out
	if len(excerpt) > maxBytes {
		excerpt = excerpt[:maxBytes] + "\n  … (truncated)"
	}
	fmt.Printf("  %s\n", strings.TrimSpace(excerpt))
}
