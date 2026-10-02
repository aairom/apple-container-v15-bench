package comparator_test

import (
	"testing"
	"time"

	"github.com/apple-container-update/internal/comparator"
)

func TestArchComparison_NotEmpty(t *testing.T) {
	rows := comparator.ArchComparison()
	if len(rows) == 0 {
		t.Error("ArchComparison() returned empty slice")
	}
	for i, r := range rows {
		if r.Dimension == "" {
			t.Errorf("row[%d]: Dimension must not be empty", i)
		}
		if r.Apple == "" {
			t.Errorf("row[%d]: Apple must not be empty", i)
		}
		if r.Podman == "" {
			t.Errorf("row[%d]: Podman must not be empty", i)
		}
	}
}

func TestFeatureComparison_NotEmpty(t *testing.T) {
	rows := comparator.FeatureComparison()
	if len(rows) == 0 {
		t.Error("FeatureComparison() returned empty slice")
	}
	for i, r := range rows {
		if r.Feature == "" {
			t.Errorf("row[%d]: Feature must not be empty", i)
		}
	}
}

func TestCLIMappings_NotEmpty(t *testing.T) {
	rows := comparator.CLIMappings()
	if len(rows) == 0 {
		t.Error("CLIMappings() returned empty slice")
	}
	for i, r := range rows {
		if r.Operation == "" {
			t.Errorf("row[%d]: Operation must not be empty", i)
		}
		if r.AppleCLI == "" {
			t.Errorf("row[%d]: AppleCLI must not be empty", i)
		}
		if r.PodmanCLI == "" {
			t.Errorf("row[%d]: PodmanCLI must not be empty", i)
		}
	}
}

func TestEducationalAnnotations_NotEmpty(t *testing.T) {
	annotations := comparator.EducationalAnnotations()
	if len(annotations) == 0 {
		t.Error("EducationalAnnotations() returned empty slice")
	}
	for i, a := range annotations {
		if a.Title == "" {
			t.Errorf("annotation[%d]: Title must not be empty", i)
		}
		if a.Lesson == "" {
			t.Errorf("annotation[%d]: Lesson must not be empty", i)
		}
	}
}

func TestComputeStats_Empty(t *testing.T) {
	r := comparator.ComputeStats("op", comparator.AppleContainer, nil, 3)
	if r.Iterations != 0 {
		t.Errorf("expected 0 iterations, got %d", r.Iterations)
	}
	if r.Errors != 3 {
		t.Errorf("expected 3 errors, got %d", r.Errors)
	}
	if r.Mean != 0 {
		t.Errorf("expected 0 mean, got %v", r.Mean)
	}
}

func TestComputeStats_Single(t *testing.T) {
	d := 500 * time.Millisecond
	r := comparator.ComputeStats("op", comparator.Podman, []time.Duration{d}, 0)
	if r.Mean != d {
		t.Errorf("expected mean %v, got %v", d, r.Mean)
	}
	if r.Min != d {
		t.Errorf("expected min %v, got %v", d, r.Min)
	}
	if r.Max != d {
		t.Errorf("expected max %v, got %v", d, r.Max)
	}
	if r.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", r.Iterations)
	}
}

func TestComputeStats_Multiple(t *testing.T) {
	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
	}
	r := comparator.ComputeStats("op", comparator.AppleContainer, durations, 0)
	wantMean := 200 * time.Millisecond
	if r.Mean != wantMean {
		t.Errorf("expected mean %v, got %v", wantMean, r.Mean)
	}
	if r.Min != 100*time.Millisecond {
		t.Errorf("expected min 100ms, got %v", r.Min)
	}
	if r.Max != 300*time.Millisecond {
		t.Errorf("expected max 300ms, got %v", r.Max)
	}
	if r.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", r.Iterations)
	}
}

func TestTruncate_Short(t *testing.T) {
	got := comparator.Truncate("hello", 10)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestTruncate_Long(t *testing.T) {
	long := "abcdefghij klmnopqrstuvwxyz"
	got := comparator.Truncate(long, 10)
	if len([]rune(got)) > 12 { // 10 chars + "…"
		t.Errorf("Truncate did not shorten string: %q", got)
	}
}

func TestTruncate_Whitespace(t *testing.T) {
	got := comparator.Truncate("  hello  ", 20)
	if got != "hello" {
		t.Errorf("expected trimmed %q, got %q", "hello", got)
	}
}

func TestRuntimeNames(t *testing.T) {
	if comparator.AppleContainer == "" {
		t.Error("AppleContainer runtime name must not be empty")
	}
	if comparator.Podman == "" {
		t.Error("Podman runtime name must not be empty")
	}
	if comparator.AppleContainer == comparator.Podman {
		t.Error("runtime names must be distinct")
	}
}
