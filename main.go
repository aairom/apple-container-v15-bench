// Package main is the entry point for the Apple Container vs Podman
// educational comparison tool.
//
// This application provides a didactical, side-by-side demonstration of
// container operations using two runtimes available on macOS:
//
//   - Apple Container (apple/container v1.3.1) – a lightweight VM-based
//     container runtime built in Swift and optimized for Apple Silicon.
//   - Podman – the daemonless, rootless container engine from Red Hat.
//
// # Learning Objectives
//
//  1. Understand how each runtime executes the same container operations.
//  2. Compare CLI command parity and differences.
//  3. Observe and interpret timing/performance characteristics.
//  4. Review JSON output formats for inspect, list, and stats.
//
// Usage:
//
//	container-compare <command> [flags]
//
// Commands:
//
//	info        Detect and display installed runtime versions
//	demo        Walk through the container lifecycle (both runtimes)
//	compare     Side-by-side operation comparison
//	benchmark   Time identical operations on both runtimes
//	dashboard   Generate self-contained HTML visual dashboard
//	help        Print this help message
//
// Examples:
//
//	container-compare info
//	container-compare demo --image alpine:latest --runtime both
//	container-compare compare --image alpine:latest
//	container-compare benchmark --image alpine:latest --iterations 3
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/apple-container-update/internal/runner"
)

// version is the application version.
const version = "1.0.0"

// appleContainerVersion documents the Apple Container release this tool
// was validated against.
const appleContainerVersion = "1.3.1"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "info":
		runInfo()
	case "demo":
		runDemo()
	case "compare":
		runCompare()
	case "benchmark":
		runBenchmark()
	case "dashboard":
		runDashboard()
	case "version", "--version", "-v":
		fmt.Printf("container-compare v%s (validated against Apple Container v%s)\n",
			version, appleContainerVersion)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// ── command handlers ──────────────────────────────────────────────────────────

func runInfo() {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	if err := fs.Parse(os.Args[2:]); err != nil {
		exitErr(err)
	}
	if err := runner.PrintRuntimeInfo(); err != nil {
		exitErr(err)
	}
}

func runDemo() {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	image := fs.String("image", "alpine:latest", "OCI image to use for the demo")
	runtime := fs.String("runtime", "both", `Runtime(s) to use: "apple", "podman", or "both"`)
	if err := fs.Parse(os.Args[2:]); err != nil {
		exitErr(err)
	}
	if err := runner.RunDemo(*image, *runtime); err != nil {
		exitErr(err)
	}
}

func runCompare() {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	image := fs.String("image", "alpine:latest", "OCI image for comparison operations")
	if err := fs.Parse(os.Args[2:]); err != nil {
		exitErr(err)
	}
	if err := runner.RunComparison(*image); err != nil {
		exitErr(err)
	}
}

func runBenchmark() {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	image := fs.String("image", "alpine:latest", "OCI image to benchmark")
	iterations := fs.Int("iterations", 3, "Number of iterations per operation (1–10)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		exitErr(err)
	}
	if err := runner.RunBenchmark(*image, *iterations); err != nil {
		exitErr(err)
	}
}

func runDashboard() {
	fs := flag.NewFlagSet("dashboard", flag.ExitOnError)
	image := fs.String("image", "alpine:latest", "OCI image for live operations and benchmarks")
	iterations := fs.Int("iterations", 1, "Benchmark iterations per operation (1–10)")
	noBrowser := fs.Bool("no-browser", false, "Do not open the dashboard in the browser automatically")
	if err := fs.Parse(os.Args[2:]); err != nil {
		exitErr(err)
	}
	if err := runner.RunDashboard(*image, *iterations, version, !*noBrowser); err != nil {
		exitErr(err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Printf(`container-compare v%s

An educational Go application that demonstrates and compares container operations
between Apple Container %s and Podman on macOS.

Usage:
  container-compare <command> [flags]

Commands:
  info        Detect and display installed runtime versions + architecture overview
  demo        Walk through the full container lifecycle on one or both runtimes
  compare     Side-by-side comparison table (live operations + static matrix)
  benchmark   Time identical operations on both runtimes (N iterations)
  dashboard   Generate self-contained HTML visual dashboard (opens in browser)
  version     Print version information
  help        Print this help message

Flags for 'demo':
  --image string     OCI image to use (default "alpine:latest")
  --runtime string   "apple", "podman", or "both" (default "both")

Flags for 'compare':
  --image string     OCI image (default "alpine:latest")

Flags for 'benchmark':
  --image string        OCI image (default "alpine:latest")
  --iterations int      Iterations per operation, 1–10 (default 3)

Flags for 'dashboard':
  --image string        OCI image (default "alpine:latest")
  --iterations int      Benchmark iterations, 1–10 (default 1)
  --no-browser          Do not open the dashboard automatically

Examples:
  container-compare info
  container-compare demo --runtime apple
  container-compare compare --image nginx:alpine
  container-compare benchmark --iterations 5
  container-compare dashboard
  container-compare dashboard --iterations 3 --no-browser

Reports are saved to ./output/ as timestamped HTML, Markdown, and JSON files.
`, version, appleContainerVersion)
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
