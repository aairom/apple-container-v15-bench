// Package reporter formats and writes output reports.
// It renders terminal tables and persists Markdown + JSON reports
// to the ./output/ directory with ISO-8601 timestamps.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/apple-container-update/internal/comparator"
)

// outputDir is the directory where reports are written.
const outputDir = "output"

// ── Terminal table helpers ────────────────────────────────────────────────────

// PrintSection prints a styled section header.
func PrintSection(w io.Writer, title string) {
	bar := strings.Repeat("─", 76)
	fmt.Fprintf(w, "\n%s\n  %s\n%s\n", bar, title, bar)
}

// PrintArchTable renders the architectural comparison as a tab-separated table.
func PrintArchTable(w io.Writer, rows []comparator.ArchRow) {
	PrintSection(w, "🏗  Architecture Comparison: Apple Container vs Podman")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DIMENSION\tAPPLE CONTAINER\tPODMAN")
	fmt.Fprintln(tw, strings.Repeat("-", 25)+"\t"+strings.Repeat("-", 30)+"\t"+strings.Repeat("-", 30))
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			comparator.Truncate(r.Dimension, 24),
			comparator.Truncate(r.Apple, 29),
			comparator.Truncate(r.Podman, 29),
		)
	}
	tw.Flush()
}

// PrintFeatureTable renders the feature comparison as a tab-separated table.
func PrintFeatureTable(w io.Writer, rows []comparator.FeatureRow) {
	PrintSection(w, "🔍  Feature Comparison Matrix")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FEATURE\tAPPLE CONTAINER\tPODMAN\tNOTES")
	fmt.Fprintln(tw,
		strings.Repeat("-", 28)+"\t"+
			strings.Repeat("-", 28)+"\t"+
			strings.Repeat("-", 28)+"\t"+
			strings.Repeat("-", 28),
	)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			comparator.Truncate(r.Feature, 27),
			comparator.Truncate(r.Apple, 27),
			comparator.Truncate(r.Podman, 27),
			comparator.Truncate(r.Notes, 27),
		)
	}
	tw.Flush()
}

// PrintCLITable renders the CLI mapping as a tab-separated table.
func PrintCLITable(w io.Writer, rows []comparator.CLIMapping) {
	PrintSection(w, "⌨️  CLI Command Mapping: Apple Container ↔ Podman")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "OPERATION\tAPPLE CONTAINER\tPODMAN\tNOTES")
	fmt.Fprintln(tw,
		strings.Repeat("-", 22)+"\t"+
			strings.Repeat("-", 32)+"\t"+
			strings.Repeat("-", 32)+"\t"+
			strings.Repeat("-", 22),
	)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			comparator.Truncate(r.Operation, 21),
			comparator.Truncate(r.AppleCLI, 31),
			comparator.Truncate(r.PodmanCLI, 31),
			comparator.Truncate(r.Notes, 21),
		)
	}
	tw.Flush()
}

// PrintComparisonRows renders live operation results side by side.
func PrintComparisonRows(w io.Writer, rows []comparator.ComparisonRow) {
	PrintSection(w, "⚡  Live Operation Results")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "OPERATION\tAPPLE CMD\tAPPLE RESULT\tPODMAN CMD\tPODMAN RESULT")
	fmt.Fprintln(tw, strings.Repeat("-", 120))
	for _, r := range rows {
		appleStatus := statusStr(r.Apple.Err, r.Apple.Duration)
		podmanStatus := statusStr(r.Podman.Err, r.Podman.Duration)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			comparator.Truncate(r.OperationName, 18),
			comparator.Truncate(r.Apple.Command, 28),
			appleStatus,
			comparator.Truncate(r.Podman.Command, 28),
			podmanStatus,
		)
	}
	tw.Flush()
}

// PrintBenchmarkTable renders benchmark timing results.
func PrintBenchmarkTable(w io.Writer, summary comparator.BenchmarkSummary) {
	PrintSection(w, "🚀  Benchmark Results — "+summary.Image)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "OPERATION\tRUNTIME\tMEAN\tMIN\tMAX\tERRORS")
	fmt.Fprintln(tw, strings.Repeat("-", 100))
	for _, p := range summary.Results {
		for _, res := range []comparator.BenchmarkResult{p.Apple, p.Podman} {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
				comparator.Truncate(p.OperationName, 18),
				string(res.Runtime),
				res.Mean.Round(time.Millisecond),
				res.Min.Round(time.Millisecond),
				res.Max.Round(time.Millisecond),
				res.Errors,
			)
		}
	}
	tw.Flush()
}

// PrintAnnotations renders the educational annotations to the terminal.
func PrintAnnotations(w io.Writer, annotations []comparator.AnnotatedOperation) {
	PrintSection(w, "📚  Educational Annotations")
	for i, a := range annotations {
		fmt.Fprintf(w, "\n[%d] %s\n", i+1, a.Title)
		fmt.Fprintln(w, wrapIndent(a.Lesson, 74, "    "))
		fmt.Fprintf(w, "    Apple Container: %s\n", a.AppleCLI)
		fmt.Fprintf(w, "    Podman:          %s\n", a.PodmanCLI)
	}
}

// ── File report writers ───────────────────────────────────────────────────────

// timestampedFilename returns a filename in the output directory with a
// RFC-3339-based timestamp prefix.
func timestampedFilename(prefix, ext string) string {
	ts := time.Now().UTC().Format("2006-01-02T150405Z")
	name := fmt.Sprintf("%s_%s.%s", prefix, ts, ext)
	return filepath.Join(outputDir, name)
}

// ensureOutputDir creates the output directory if it does not exist.
func ensureOutputDir() error {
	return os.MkdirAll(outputDir, 0o750)
}

// WriteMarkdownReport writes a full Markdown comparison report to ./output/.
func WriteMarkdownReport(
	archRows []comparator.ArchRow,
	featureRows []comparator.FeatureRow,
	cliRows []comparator.CLIMapping,
	compRows []comparator.ComparisonRow,
	annotations []comparator.AnnotatedOperation,
) (string, error) {
	if err := ensureOutputDir(); err != nil {
		return "", err
	}
	path := timestampedFilename("comparison_report", "md")
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer f.Close()

	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Fprintf(f, "# Apple Container vs Podman — Comparison Report\n\n")
	fmt.Fprintf(f, "_Generated: %s_\n\n", ts)
	fmt.Fprintf(f, "---\n\n")

	// Architecture
	fmt.Fprintln(f, "## Architecture")
	fmt.Fprintln(f, "| Dimension | Apple Container | Podman |")
	fmt.Fprintln(f, "|-----------|-----------------|--------|")
	for _, r := range archRows {
		fmt.Fprintf(f, "| %s | %s | %s |\n",
			escapeMD(r.Dimension), escapeMD(r.Apple), escapeMD(r.Podman))
	}
	fmt.Fprintln(f)

	// Features
	fmt.Fprintln(f, "## Features")
	fmt.Fprintln(f, "| Feature | Apple Container | Podman | Notes |")
	fmt.Fprintln(f, "|---------|-----------------|--------|-------|")
	for _, r := range featureRows {
		fmt.Fprintf(f, "| %s | %s | %s | %s |\n",
			escapeMD(r.Feature), escapeMD(r.Apple), escapeMD(r.Podman), escapeMD(r.Notes))
	}
	fmt.Fprintln(f)

	// CLI Mapping
	fmt.Fprintln(f, "## CLI Command Mapping")
	fmt.Fprintln(f, "| Operation | Apple Container | Podman | Notes |")
	fmt.Fprintln(f, "|-----------|-----------------|--------|-------|")
	for _, r := range cliRows {
		fmt.Fprintf(f, "| %s | `%s` | `%s` | %s |\n",
			escapeMD(r.Operation), r.AppleCLI, r.PodmanCLI, escapeMD(r.Notes))
	}
	fmt.Fprintln(f)

	// Live results (if any)
	if len(compRows) > 0 {
		fmt.Fprintln(f, "## Live Operation Results")
		fmt.Fprintln(f, "| Operation | Apple CMD | Apple Duration | Podman CMD | Podman Duration |")
		fmt.Fprintln(f, "|-----------|-----------|----------------|------------|-----------------|")
		for _, r := range compRows {
			ad := r.Apple.Duration.Round(time.Millisecond).String()
			pd := r.Podman.Duration.Round(time.Millisecond).String()
			if r.Apple.Err != nil {
				ad = "ERROR"
			}
			if r.Podman.Err != nil {
				pd = "ERROR"
			}
			fmt.Fprintf(f, "| %s | `%s` | %s | `%s` | %s |\n",
				escapeMD(r.OperationName),
				r.Apple.Command, ad,
				r.Podman.Command, pd)
		}
		fmt.Fprintln(f)
	}

	// Educational notes
	fmt.Fprintln(f, "## Educational Annotations")
	for i, a := range annotations {
		fmt.Fprintf(f, "\n### %d. %s\n\n%s\n\n", i+1, a.Title, a.Lesson)
		fmt.Fprintf(f, "**Apple Container:** `%s`\n\n", a.AppleCLI)
		fmt.Fprintf(f, "**Podman:** `%s`\n\n", a.PodmanCLI)
	}

	return path, nil
}

// WriteJSONBenchmark writes benchmark results as timestamped JSON.
func WriteJSONBenchmark(summary comparator.BenchmarkSummary) (string, error) {
	if err := ensureOutputDir(); err != nil {
		return "", err
	}
	path := timestampedFilename("benchmark", "json")
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		return "", err
	}
	return path, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func statusStr(err error, d time.Duration) string {
	if err != nil {
		return "ERROR"
	}
	return "OK " + d.Round(time.Millisecond).String()
}

func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func wrapIndent(s string, width int, indent string) string {
	words := strings.Fields(s)
	var lines []string
	line := indent
	for _, w := range words {
		if len(line)+len(w)+1 > width {
			lines = append(lines, line)
			line = indent + w
		} else {
			if line == indent {
				line += w
			} else {
				line += " " + w
			}
		}
	}
	if line != indent {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
