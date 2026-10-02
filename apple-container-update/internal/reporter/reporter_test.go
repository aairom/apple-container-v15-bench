// Package reporter_test exercises the reporter package using in-memory writers
// and a temporary output directory so no real filesystem side-effects occur.
package reporter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apple-container-update/internal/comparator"
	"github.com/apple-container-update/internal/reporter"
)

// ── helper fixtures ───────────────────────────────────────────────────────────

func archRows() []comparator.ArchRow {
	return []comparator.ArchRow{
		{Dimension: "Isolation", Apple: "VM per container", Podman: "cgroups/namespaces"},
		{Dimension: "Daemon", Apple: "launchd API server", Podman: "daemonless"},
	}
}

func featureRows() []comparator.FeatureRow {
	return []comparator.FeatureRow{
		{Feature: "Rootless", Apple: "Yes (VM)", Podman: "Yes", Notes: "both rootless"},
		{Feature: "GPU", Apple: "No", Podman: "Limited", Notes: ""},
	}
}

func cliRows() []comparator.CLIMapping {
	return []comparator.CLIMapping{
		{Operation: "Pull image", AppleCLI: "container image pull", PodmanCLI: "podman pull", Notes: ""},
		{Operation: "List containers", AppleCLI: "container list", PodmanCLI: "podman ps", Notes: ""},
	}
}

func compRows() []comparator.ComparisonRow {
	return []comparator.ComparisonRow{
		{
			OperationName: "Pull image",
			Apple: comparator.OperationResult{
				Runtime: comparator.AppleContainer, Command: "container image pull alpine",
				Duration: 500 * time.Millisecond,
			},
			Podman: comparator.OperationResult{
				Runtime: comparator.Podman, Command: "podman pull alpine",
				Duration: 300 * time.Millisecond,
			},
		},
		{
			OperationName: "Run (echo)",
			Apple: comparator.OperationResult{
				Runtime: comparator.AppleContainer, Command: "container run alpine echo hi",
				Err: errors.New("apple error"),
			},
			Podman: comparator.OperationResult{
				Runtime: comparator.Podman, Command: "podman run alpine echo hi",
				Duration: 150 * time.Millisecond,
			},
		},
	}
}

func annotations() []comparator.AnnotatedOperation {
	return []comparator.AnnotatedOperation{
		{
			Title:     "VM isolation",
			Lesson:    "Each Apple Container container runs as a separate VM.",
			AppleCLI:  "container run alpine",
			PodmanCLI: "podman run alpine",
		},
	}
}

func benchmarkSummary() comparator.BenchmarkSummary {
	return comparator.BenchmarkSummary{
		Image: "alpine:latest",
		Results: []comparator.BenchmarkPair{
			{
				OperationName: "Run echo",
				Apple: comparator.BenchmarkResult{
					Runtime: comparator.AppleContainer,
					Mean:    800 * time.Millisecond,
					Min:     700 * time.Millisecond,
					Max:     900 * time.Millisecond,
					Errors:  0,
				},
				Podman: comparator.BenchmarkResult{
					Runtime: comparator.Podman,
					Mean:    200 * time.Millisecond,
					Min:     180 * time.Millisecond,
					Max:     240 * time.Millisecond,
					Errors:  0,
				},
			},
		},
	}
}

// ── terminal table helpers ────────────────────────────────────────────────────

func TestPrintSection_containsTitle(t *testing.T) {
	var buf bytes.Buffer
	reporter.PrintSection(&buf, "Test Section")
	if !strings.Contains(buf.String(), "Test Section") {
		t.Errorf("PrintSection output missing title, got: %q", buf.String())
	}
}

func TestPrintArchTable_containsHeaders(t *testing.T) {
	var buf bytes.Buffer
	reporter.PrintArchTable(&buf, archRows())
	out := buf.String()
	for _, want := range []string{"DIMENSION", "APPLE CONTAINER", "PODMAN", "Isolation", "Daemon"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintArchTable missing %q in output", want)
		}
	}
}

func TestPrintFeatureTable_containsRows(t *testing.T) {
	var buf bytes.Buffer
	reporter.PrintFeatureTable(&buf, featureRows())
	out := buf.String()
	if !strings.Contains(out, "Rootless") {
		t.Errorf("PrintFeatureTable missing 'Rootless' row")
	}
}

func TestPrintCLITable_containsMapping(t *testing.T) {
	var buf bytes.Buffer
	reporter.PrintCLITable(&buf, cliRows())
	out := buf.String()
	if !strings.Contains(out, "Pull image") {
		t.Errorf("PrintCLITable missing 'Pull image'")
	}
	if !strings.Contains(out, "podman pull") {
		t.Errorf("PrintCLITable missing 'podman pull'")
	}
}

func TestPrintComparisonRows_okAndError(t *testing.T) {
	var buf bytes.Buffer
	reporter.PrintComparisonRows(&buf, compRows())
	out := buf.String()
	if !strings.Contains(out, "Pull image") {
		t.Errorf("PrintComparisonRows missing 'Pull image'")
	}
	if !strings.Contains(out, "ERROR") {
		t.Errorf("PrintComparisonRows should show ERROR for failed Apple result")
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("PrintComparisonRows should show OK for successful Podman result")
	}
}

func TestPrintBenchmarkTable_containsRuntimes(t *testing.T) {
	var buf bytes.Buffer
	reporter.PrintBenchmarkTable(&buf, benchmarkSummary())
	out := buf.String()
	if !strings.Contains(out, "Run echo") {
		t.Errorf("PrintBenchmarkTable missing 'Run echo'")
	}
}

func TestPrintAnnotations_containsLesson(t *testing.T) {
	var buf bytes.Buffer
	reporter.PrintAnnotations(&buf, annotations())
	out := buf.String()
	if !strings.Contains(out, "VM isolation") {
		t.Errorf("PrintAnnotations missing title")
	}
	if !strings.Contains(out, "Apple Container") {
		t.Errorf("PrintAnnotations missing Apple CLI label")
	}
}

// ── file writers ──────────────────────────────────────────────────────────────

// overrideOutputDir patches the outputDir used inside reporter by temporarily
// setting the working directory to a temp directory whose "output" sub-dir we
// control.  The reporter package uses the literal constant "output" resolved
// relative to the process cwd.
func withTempOutput(t *testing.T, fn func(dir string)) {
	t.Helper()
	tmp := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	fn(filepath.Join(tmp, "output"))
}

func TestWriteMarkdownReport_createsFile(t *testing.T) {
	withTempOutput(t, func(outDir string) {
		path, err := reporter.WriteMarkdownReport(
			archRows(), featureRows(), cliRows(), compRows(), annotations(),
		)
		if err != nil {
			t.Fatalf("WriteMarkdownReport: %v", err)
		}
		if !strings.HasSuffix(path, ".md") {
			t.Errorf("expected .md path, got %q", path)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile: %v", readErr)
		}
		content := string(data)
		for _, want := range []string{"Architecture", "Features", "CLI Command Mapping", "Educational"} {
			if !strings.Contains(content, want) {
				t.Errorf("markdown report missing section %q", want)
			}
		}
	})
}

func TestWriteMarkdownReport_includesLiveResults(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, err := reporter.WriteMarkdownReport(
			archRows(), featureRows(), cliRows(), compRows(), annotations(),
		)
		if err != nil {
			t.Fatalf("WriteMarkdownReport: %v", err)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "Live Operation Results") {
			t.Errorf("expected Live Operation Results section when compRows is non-empty")
		}
	})
}

func TestWriteMarkdownReport_noLiveResultsWhenEmpty(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, err := reporter.WriteMarkdownReport(
			archRows(), featureRows(), cliRows(),
			nil, // empty compRows
			annotations(),
		)
		if err != nil {
			t.Fatalf("WriteMarkdownReport: %v", err)
		}
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "Live Operation Results") {
			t.Errorf("unexpected Live Operation Results section when compRows is nil")
		}
	})
}

func TestWriteJSONBenchmark_validJSON(t *testing.T) {
	withTempOutput(t, func(_ string) {
		path, err := reporter.WriteJSONBenchmark(benchmarkSummary())
		if err != nil {
			t.Fatalf("WriteJSONBenchmark: %v", err)
		}
		if !strings.HasSuffix(path, ".json") {
			t.Errorf("expected .json path, got %q", path)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile: %v", readErr)
		}
		var m map[string]interface{}
		if jsonErr := json.Unmarshal(data, &m); jsonErr != nil {
			t.Errorf("output is not valid JSON: %v", jsonErr)
		}
		if _, ok := m["Image"]; !ok {
			t.Errorf("JSON missing 'Image' key")
		}
	})
}
