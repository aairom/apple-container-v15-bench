// Package reporter_test — dashboard_test.go
// Tests for WriteHTMLDashboard — verifies the generated HTML file
// is well-formed, contains all expected sections, and handles nil Benchmark.
package reporter_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apple-container-update/internal/comparator"
	"github.com/apple-container-update/internal/reporter"
)

// fullDashboardData returns a DashboardData fixture with all sections populated.
func fullDashboardData() reporter.DashboardData {
	return reporter.DashboardData{
		GeneratedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		ToolVersion: "1.0.0-test",
		ArchRows:    archRows(),
		FeatureRows: featureRows(),
		CLIRows:     cliRows(),
		CompRows:    compRows(),
		Annotations: annotations(),
		Benchmark: &comparator.BenchmarkSummary{
			Image: "alpine:latest",
			Results: []comparator.BenchmarkPair{
				{
					OperationName: "Run echo",
					Apple: comparator.BenchmarkResult{
						Runtime:    comparator.AppleContainer,
						Iterations: 2,
						Mean:       800 * time.Millisecond,
						Min:        700 * time.Millisecond,
						Max:        900 * time.Millisecond,
					},
					Podman: comparator.BenchmarkResult{
						Runtime:    comparator.Podman,
						Iterations: 2,
						Mean:       200 * time.Millisecond,
						Min:        180 * time.Millisecond,
						Max:        240 * time.Millisecond,
					},
				},
			},
		},
		AppleAvailable:  true,
		PodmanAvailable: false,
	}
}

func TestWriteHTMLDashboard_createsHTMLFile(t *testing.T) {
	withTempOutput(t, func(_ string) {
		data := fullDashboardData()
		path, err := reporter.WriteHTMLDashboard(data)
		if err != nil {
			t.Fatalf("WriteHTMLDashboard: %v", err)
		}
		if !strings.HasSuffix(path, ".html") {
			t.Errorf("expected .html path, got %q", path)
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat output file: %v", statErr)
		}
		if info.Size() == 0 {
			t.Error("output file is empty")
		}
	})
}

func TestWriteHTMLDashboard_isValidHTML(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, err := reporter.WriteHTMLDashboard(fullDashboardData())
		if err != nil {
			t.Fatalf("WriteHTMLDashboard: %v", err)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile: %v", readErr)
		}
		html := string(data)
		for _, want := range []string{"<!DOCTYPE html>", "<html", "</html>"} {
			if !strings.Contains(html, want) {
				t.Errorf("dashboard HTML missing %q", want)
			}
		}
	})
}

func TestWriteHTMLDashboard_containsAllTabs(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, _ := reporter.WriteHTMLDashboard(fullDashboardData())
		data, _ := os.ReadFile(path)
		html := string(data)
		tabs := []string{
			"Overview", "Architecture", "CLI", "Features",
			"Live Results", "Benchmarks", "Lessons",
		}
		for _, tab := range tabs {
			if !strings.Contains(html, tab) {
				t.Errorf("dashboard HTML missing tab/section %q", tab)
			}
		}
	})
}

func TestWriteHTMLDashboard_containsRuntimeData(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, _ := reporter.WriteHTMLDashboard(fullDashboardData())
		data, _ := os.ReadFile(path)
		html := string(data)
		for _, want := range []string{
			"Apple Container",
			"Podman",
			"1.0.0-test",      // ToolVersion
			"2026-01-01",      // GeneratedAt date
			"alpine:latest",   // Benchmark image
			"Run echo",        // BenchmarkPair operation
		} {
			if !strings.Contains(html, want) {
				t.Errorf("dashboard HTML missing expected content %q", want)
			}
		}
	})
}

func TestWriteHTMLDashboard_availabilityBadges(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, _ := reporter.WriteHTMLDashboard(fullDashboardData())
		data, _ := os.ReadFile(path)
		html := string(data)
		// Apple is available, Podman is not.
		if !strings.Contains(html, "Apple: Available") {
			t.Error("expected 'Apple: Available' badge")
		}
		if !strings.Contains(html, "Podman: Not Available") {
			t.Error("expected 'Podman: Not Available' badge")
		}
	})
}

func TestWriteHTMLDashboard_nilBenchmark(t *testing.T) {
	withTempOutput(t, func(_ string) {
		data := fullDashboardData()
		data.Benchmark = nil
		path, err := reporter.WriteHTMLDashboard(data)
		if err != nil {
			t.Fatalf("WriteHTMLDashboard with nil Benchmark: %v", err)
		}
		html, _ := os.ReadFile(path)
		// Should still render without the benchmark section data.
		if !strings.Contains(string(html), "<!DOCTYPE html>") {
			t.Error("nil Benchmark produced invalid HTML")
		}
	})
}

func TestWriteHTMLDashboard_emptySections(t *testing.T) {
	withTempOutput(t, func(_ string) {
		data := reporter.DashboardData{
			GeneratedAt: time.Now(),
			ToolVersion: "0.0.0",
		}
		path, err := reporter.WriteHTMLDashboard(data)
		if err != nil {
			t.Fatalf("WriteHTMLDashboard with empty data: %v", err)
		}
		html, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile: %v", readErr)
		}
		if !strings.Contains(string(html), "<!DOCTYPE html>") {
			t.Error("empty data produced invalid HTML")
		}
	})
}

func TestWriteHTMLDashboard_benchmarkBarChart(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, _ := reporter.WriteHTMLDashboard(fullDashboardData())
		data, _ := os.ReadFile(path)
		html := string(data)
		// SVG elements for bar chart should be present.
		if !strings.Contains(html, "<svg") {
			t.Error("benchmark bar chart SVG missing from dashboard")
		}
		// Bar colours for Apple (blue) and Podman (red/pink).
		if !strings.Contains(html, "#5b9bd5") {
			t.Error("Apple bar colour #5b9bd5 missing from SVG chart")
		}
		if !strings.Contains(html, "#f38ba8") {
			t.Error("Podman bar colour #f38ba8 missing from SVG chart")
		}
	})
}

func TestWriteHTMLDashboard_noExternalResources(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, _ := reporter.WriteHTMLDashboard(fullDashboardData())
		data, _ := os.ReadFile(path)
		html := string(data)
		// Must not load external resources via src/href-style asset loading.
		// We check for patterns that would cause the browser to fetch external
		// assets (script src, link href to stylesheet, img src, @import).
		// Plain <a href="https://..."> hyperlinks are fine — they are not
		// loaded automatically by the browser.
		forbidden := []string{
			"cdn.jsdelivr",
			"fonts.googleapis",
			"fonts.gstatic",
			"<script src=",
			"<link rel=\"stylesheet\"",
			"@import url(",
		}
		for _, f := range forbidden {
			if strings.Contains(html, f) {
				t.Errorf("dashboard HTML loads external resource %q — must be self-contained", f)
			}
		}
	})
}

func TestWriteHTMLDashboard_liveResultsBadges(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, _ := reporter.WriteHTMLDashboard(fullDashboardData())
		data, _ := os.ReadFile(path)
		html := string(data)
		// compRows() has one OK and one ERROR result.
		if !strings.Contains(html, "badge-ok") {
			t.Error("expected badge-ok class for successful live result")
		}
		if !strings.Contains(html, "badge-err") {
			t.Error("expected badge-err class for failed live result")
		}
	})
}
