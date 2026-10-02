// Package reporter — dashboard.go
//
// WriteHTMLDashboard generates a fully self-contained, single-file HTML
// dashboard that visualises all comparison and benchmark data produced by the
// container-compare tool.
//
// Design goals:
//   - Zero external dependencies: no CDN, no JS framework, no fonts fetch.
//     Everything is inlined — CSS, all content, SVG charts built by template.
//   - Tab-based navigation (pure CSS, no JavaScript required).
//   - Colour-coded status badges, SVG horizontal bar charts, timing tables.
//   - Safe HTML generation via html/template throughout; no template.HTML
//     conversions of dynamic data (avoids XSS risk).
//   - Written to ./output/ with an ISO-8601 timestamp prefix.
package reporter

import (
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/apple-container-update/internal/comparator"
)

// ── Public types ──────────────────────────────────────────────────────────────

// DashboardData bundles every dataset the HTML dashboard needs.
type DashboardData struct {
	// GeneratedAt is stamped in the report header.
	GeneratedAt time.Time
	// ToolVersion is the container-compare version string.
	ToolVersion string
	// ArchRows holds the architectural comparison matrix.
	ArchRows []comparator.ArchRow
	// FeatureRows holds the feature comparison matrix.
	FeatureRows []comparator.FeatureRow
	// CLIRows holds the CLI command mapping.
	CLIRows []comparator.CLIMapping
	// CompRows holds live operation results (may be nil/empty).
	CompRows []comparator.ComparisonRow
	// Annotations holds the educational annotations.
	Annotations []comparator.AnnotatedOperation
	// Benchmark holds timing results (may be nil when not run).
	Benchmark *comparator.BenchmarkSummary
	// AppleAvailable indicates whether Apple Container was reachable.
	AppleAvailable bool
	// PodmanAvailable indicates whether Podman was reachable.
	PodmanAvailable bool
}

// WriteHTMLDashboard renders the DashboardData to a timestamped HTML file in
// ./output/ and returns the path of the created file.
func WriteHTMLDashboard(data DashboardData) (string, error) {
	if err := ensureOutputDir(); err != nil {
		return "", err
	}

	path := timestampedFilename("dashboard", "html")
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer f.Close()

	vm := buildViewModel(data)

	tmpl, err := template.New("dashboard").Funcs(dashFuncs()).Parse(dashboardTemplate)
	if err != nil {
		return "", fmt.Errorf("parse dashboard template: %w", err)
	}
	if err := tmpl.Execute(f, vm); err != nil {
		return "", fmt.Errorf("execute dashboard template: %w", err)
	}

	return path, nil
}

// ── Internal view models ──────────────────────────────────────────────────────

// dashboardVM is the sole data structure passed to the HTML template.
// All fields contain only Go-typed values; the template engine escapes every
// string automatically — no template.HTML conversions are used.
type dashboardVM struct {
	Data           DashboardData
	GeneratedStr   string
	CompStatusRows []compStatusRow
	BenchRows      []benchRow
}

// compStatusRow is a flattened view of a ComparisonRow for table rendering.
type compStatusRow struct {
	Op             string
	AppleCmd       string
	AppleStatus    string // "OK" | "ERROR" | "SKIP"
	AppleBadge     string // CSS class name
	AppleDuration  string
	PodmanCmd      string
	PodmanStatus   string
	PodmanBadge    string
	PodmanDuration string
}

// benchRow carries per-operation benchmark data including pre-computed bar
// widths (0–320 pixels) so the SVG can be assembled by the template without
// any raw-HTML injection.
type benchRow struct {
	OpName       string
	// Apple stats
	AppleMS      float64 // mean in milliseconds
	AppleMinMS   float64
	AppleMaxMS   float64
	AppleIter    int
	AppleErrors  int
	AppleBarW    int    // pixel width of bar (0–320)
	AppleLabel   string // text shown after bar
	// Podman stats
	PodmanMS     float64
	PodmanMinMS  float64
	PodmanMaxMS  float64
	PodmanIter   int
	PodmanErrors int
	PodmanBarW   int
	PodmanLabel  string
	// Chart height so the template can size the SVG element
	SVGHeight int
}

// barMaxPx is the pixel width of the longest bar in the SVG chart.
const barMaxPx = 320

// buildViewModel converts DashboardData into the template-ready dashboardVM.
func buildViewModel(data DashboardData) dashboardVM {
	vm := dashboardVM{
		Data:           data,
		GeneratedStr:   data.GeneratedAt.UTC().Format("2006-01-02 15:04:05 UTC"),
		CompStatusRows: buildCompStatusRows(data.CompRows),
	}
	if data.Benchmark != nil {
		vm.BenchRows = buildBenchRows(data.Benchmark)
	}
	return vm
}

// buildBenchRows converts BenchmarkSummary into pre-computed benchRow values.
func buildBenchRows(bs *comparator.BenchmarkSummary) []benchRow {
	rows := make([]benchRow, 0, len(bs.Results))
	for _, pair := range bs.Results {
		appleMS := msFloat(pair.Apple.Mean)
		podmanMS := msFloat(pair.Podman.Mean)
		maxMS := math.Max(appleMS, podmanMS)
		if maxMS == 0 {
			maxMS = 1
		}
		appleW := int(float64(barMaxPx) * appleMS / maxMS)
		podmanW := int(float64(barMaxPx) * podmanMS / maxMS)

		appleLabel := fmt.Sprintf("%.0fms", appleMS)
		if pair.Apple.Errors > 0 {
			appleLabel = "ERROR"
			appleW = 0
		}
		podmanLabel := fmt.Sprintf("%.0fms", podmanMS)
		if pair.Podman.Errors > 0 {
			podmanLabel = "ERROR"
			podmanW = 0
		}

		rows = append(rows, benchRow{
			OpName:       pair.OperationName,
			AppleMS:      appleMS,
			AppleMinMS:   msFloat(pair.Apple.Min),
			AppleMaxMS:   msFloat(pair.Apple.Max),
			AppleIter:    pair.Apple.Iterations,
			AppleErrors:  pair.Apple.Errors,
			AppleBarW:    appleW,
			AppleLabel:   appleLabel,
			PodmanMS:     podmanMS,
			PodmanMinMS:  msFloat(pair.Podman.Min),
			PodmanMaxMS:  msFloat(pair.Podman.Max),
			PodmanIter:   pair.Podman.Iterations,
			PodmanErrors: pair.Podman.Errors,
			PodmanBarW:   podmanW,
			PodmanLabel:  podmanLabel,
			SVGHeight:    104,
		})
	}
	return rows
}

func msFloat(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// ── Status row builder ────────────────────────────────────────────────────────

func buildCompStatusRows(rows []comparator.ComparisonRow) []compStatusRow {
	out := make([]compStatusRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, compStatusRow{
			Op:             r.OperationName,
			AppleCmd:       r.Apple.Command,
			AppleStatus:    resultStatus(r.Apple),
			AppleBadge:     resultBadge(r.Apple),
			AppleDuration:  resultDuration(r.Apple),
			PodmanCmd:      r.Podman.Command,
			PodmanStatus:   resultStatus(r.Podman),
			PodmanBadge:    resultBadge(r.Podman),
			PodmanDuration: resultDuration(r.Podman),
		})
	}
	return out
}

func resultStatus(r comparator.OperationResult) string {
	if r.Err != nil {
		return "ERROR"
	}
	if r.Duration == 0 && r.Output == "" {
		return "SKIP"
	}
	return "OK"
}

func resultBadge(r comparator.OperationResult) string {
	if r.Err != nil {
		return "badge-err"
	}
	if r.Duration == 0 && r.Output == "" {
		return "badge-skip"
	}
	return "badge-ok"
}

func resultDuration(r comparator.OperationResult) string {
	if r.Err != nil {
		return r.Err.Error()
	}
	if r.Duration == 0 {
		return "—"
	}
	return r.Duration.Round(time.Millisecond).String()
}

// ── Template functions ────────────────────────────────────────────────────────

func dashFuncs() template.FuncMap {
	return template.FuncMap{
		// add1 returns i+1 for 1-based list counters.
		"add1": func(i int) int { return i + 1 },
		// hasData returns true when n > 0.
		"hasData": func(n int) bool { return n > 0 },
		// available returns a check or cross symbol.
		"available": func(ok bool) string {
			if ok {
				return "✅"
			}
			return "❌"
		},
		// gt compares two ints so the template can write {{gt .X 0}}.
		"gt": func(a, b int) bool { return a > b },
		// fmtMS formats a float64 millisecond value as "NNNms".
		"fmtMS": func(ms float64) string { return fmt.Sprintf("%.0fms", ms) },
		// barX computes the text x-position after a bar: base + width + 4px gap.
		"barX": func(base, w int) int { return base + w + 4 },
		// max returns the larger of two float64 values (used for axis label).
		"max": maxF,
	}
}

// ── HTML template ─────────────────────────────────────────────────────────────
//
// The template uses pure-CSS tab switching via the :checked pseudo-class
// on hidden radio inputs.  No JavaScript is required.
// All dynamic string values are auto-escaped by html/template.
// SVG bar charts are constructed entirely from integer/float fields —
// no template.HTML conversions are used anywhere.
const dashboardTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Apple Container vs Podman — Dashboard</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,"Segoe UI",system-ui,sans-serif;font-size:14px;line-height:1.6;background:#1e1e2e;color:#cdd6f4;min-height:100vh}
a{color:#89b4fa}
code,pre{font-family:"SF Mono",ui-monospace,monospace;font-size:12px}
.header{background:linear-gradient(135deg,#181825 0%,#313244 100%);border-bottom:2px solid #45475a;padding:20px 32px}
.header h1{font-size:22px;font-weight:700;color:#cba6f7;margin-bottom:4px}
.header .sub{color:#6c7086;font-size:12px}
.header .badges{margin-top:10px;display:flex;gap:10px;flex-wrap:wrap}
.rt-apple{background:#313244;color:#89dceb;border:1px solid #89dceb;border-radius:12px;padding:2px 10px;font-size:12px}
.rt-podman{background:#313244;color:#fab387;border:1px solid #fab387;border-radius:12px;padding:2px 10px;font-size:12px}
.tabs input[type=radio]{display:none}
.tabs{background:#181825;padding:0 24px;border-bottom:1px solid #313244}
.tab-labels{display:flex;flex-wrap:wrap;gap:0}
.tab-labels label{padding:12px 18px;cursor:pointer;color:#6c7086;font-size:13px;font-weight:500;border-bottom:3px solid transparent;transition:all .15s;white-space:nowrap}
.tab-labels label:hover{color:#cdd6f4}
#tab-overview:checked ~ .tab-content #panel-overview,
#tab-arch:checked     ~ .tab-content #panel-arch,
#tab-cli:checked      ~ .tab-content #panel-cli,
#tab-features:checked ~ .tab-content #panel-features,
#tab-live:checked     ~ .tab-content #panel-live,
#tab-bench:checked    ~ .tab-content #panel-bench,
#tab-lessons:checked  ~ .tab-content #panel-lessons{display:block}
#tab-overview:checked ~ .tab-labels label[for=tab-overview],
#tab-arch:checked     ~ .tab-labels label[for=tab-arch],
#tab-cli:checked      ~ .tab-labels label[for=tab-cli],
#tab-features:checked ~ .tab-labels label[for=tab-features],
#tab-live:checked     ~ .tab-labels label[for=tab-live],
#tab-bench:checked    ~ .tab-labels label[for=tab-bench],
#tab-lessons:checked  ~ .tab-labels label[for=tab-lessons]{color:#cba6f7;border-bottom-color:#cba6f7}
.tab-content .panel{display:none;padding:28px 32px}
.tbl{width:100%;border-collapse:collapse;font-size:13px}
.tbl th{background:#313244;color:#a6adc8;text-align:left;padding:10px 14px;border-bottom:2px solid #45475a;font-weight:600;text-transform:uppercase;font-size:11px;letter-spacing:.05em}
.tbl td{padding:9px 14px;border-bottom:1px solid #313244;vertical-align:top}
.tbl tr:hover td{background:#313244}
.tbl td code{background:#181825;padding:2px 6px;border-radius:4px;color:#a6e3a1;border:1px solid #313244}
.badge{display:inline-block;padding:2px 9px;border-radius:10px;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.04em}
.badge-ok  {background:#1e3a2e;color:#a6e3a1;border:1px solid #a6e3a1}
.badge-err {background:#3a1e1e;color:#f38ba8;border:1px solid #f38ba8}
.badge-skip{background:#2e2e1e;color:#f9e2af;border:1px solid #f9e2af}
.card-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin-bottom:24px}
.card{background:#313244;border:1px solid #45475a;border-radius:10px;padding:18px 20px}
.card-title{color:#a6adc8;font-size:11px;text-transform:uppercase;letter-spacing:.06em;margin-bottom:6px}
.card-value{font-size:24px;font-weight:700;color:#cdd6f4}
.card-sub{color:#6c7086;font-size:11px;margin-top:4px}
.sec-title{font-size:16px;font-weight:700;color:#cba6f7;margin-bottom:16px;padding-bottom:8px;border-bottom:1px solid #313244}
.sec-sub{color:#6c7086;font-size:13px;margin-bottom:20px;line-height:1.7}
.avail-row{display:flex;gap:24px;margin-bottom:20px;flex-wrap:wrap}
.avail-item{background:#313244;border:1px solid #45475a;border-radius:8px;padding:12px 18px;flex:1;min-width:200px}
.avail-item .icon{font-size:22px;margin-bottom:4px}
.avail-item .name{font-size:13px;font-weight:600;color:#cdd6f4}
.avail-item .status{font-size:12px;color:#6c7086;margin-top:2px}
.callout{background:#2a2a3e;border-left:3px solid #cba6f7;border-radius:0 8px 8px 0;padding:12px 16px;margin-bottom:20px;color:#a6adc8;font-size:13px;line-height:1.7}
.callout strong{color:#cdd6f4}
.bench-block{margin-bottom:32px}
.bench-op-title{font-size:13px;font-weight:600;color:#89b4fa;margin-bottom:8px}
.anno-card{background:#313244;border:1px solid #45475a;border-radius:10px;padding:20px 24px;margin-bottom:16px}
.anno-num{display:inline-block;background:#cba6f7;color:#1e1e2e;font-weight:700;font-size:11px;border-radius:10px;padding:1px 8px;margin-right:8px}
.anno-title{font-size:15px;font-weight:700;color:#cdd6f4;margin-bottom:10px}
.anno-lesson{color:#a6adc8;font-size:13px;line-height:1.75;white-space:pre-wrap;margin-bottom:14px}
.anno-cli{background:#181825;border-radius:6px;padding:10px 14px;margin-top:6px}
.anno-cli-label{color:#6c7086;font-size:11px;text-transform:uppercase;margin-bottom:4px}
.anno-cli code{color:#a6e3a1;font-size:12px;white-space:pre-wrap}
.footer{text-align:center;padding:20px;color:#45475a;font-size:11px;border-top:1px solid #313244;margin-top:32px}
</style>
</head>
<body>

<div class="header">
  <h1>🐳 Apple Container vs Podman — Visual Dashboard</h1>
  <div class="sub">Generated {{.GeneratedStr}} · container-compare v{{.Data.ToolVersion}}</div>
  <div class="badges">
    <span class="rt-apple">🍎 Apple Container v1.3.1</span>
    <span class="rt-podman">🦭 Podman</span>
    {{if .Data.AppleAvailable}}<span class="badge badge-ok">Apple: Available</span>{{else}}<span class="badge badge-skip">Apple: Not Available</span>{{end}}
    {{if .Data.PodmanAvailable}}<span class="badge badge-ok">Podman: Available</span>{{else}}<span class="badge badge-skip">Podman: Not Available</span>{{end}}
  </div>
</div>

<div class="tabs">
  <input type="radio" name="tabs" id="tab-overview" checked>
  <input type="radio" name="tabs" id="tab-arch">
  <input type="radio" name="tabs" id="tab-cli">
  <input type="radio" name="tabs" id="tab-features">
  <input type="radio" name="tabs" id="tab-live">
  <input type="radio" name="tabs" id="tab-bench">
  <input type="radio" name="tabs" id="tab-lessons">

  <div class="tab-labels">
    <label for="tab-overview">🏠 Overview</label>
    <label for="tab-arch">🏗 Architecture</label>
    <label for="tab-cli">⌨️ CLI Mapping</label>
    <label for="tab-features">🔍 Features</label>
    <label for="tab-live">⚡ Live Results</label>
    <label for="tab-bench">🚀 Benchmarks</label>
    <label for="tab-lessons">📚 Lessons</label>
  </div>

  <div class="tab-content">

    <!-- TAB 1: OVERVIEW -->
    <div id="panel-overview" class="panel">
      <div class="sec-title">Runtime Overview</div>
      <div class="sec-sub">
        Comparing <strong>Apple Container</strong> (VM-per-container isolation via
        Apple Virtualization.framework) and <strong>Podman</strong> (daemonless OCI
        runtime using Linux namespaces inside Podman Machine on macOS).
      </div>
      <div class="avail-row">
        <div class="avail-item">
          <div class="icon">🍎</div>
          <div class="name">Apple Container</div>
          <div class="status">{{available .Data.AppleAvailable}} {{if .Data.AppleAvailable}}Detected on PATH{{else}}Not found on PATH{{end}}</div>
          <div class="status">Requires: macOS 26+, Apple Silicon</div>
          <div class="status">API: com.apple.container.apiserver (launchd)</div>
        </div>
        <div class="avail-item">
          <div class="icon">🦭</div>
          <div class="name">Podman</div>
          <div class="status">{{available .Data.PodmanAvailable}} {{if .Data.PodmanAvailable}}Detected on PATH{{else}}Not found on PATH{{end}}</div>
          <div class="status">Daemonless; Podman Machine VM required on macOS</div>
          <div class="status">Homepage: <a href="https://podman.io" target="_blank" rel="noopener">podman.io</a></div>
        </div>
      </div>
      <div class="card-grid">
        <div class="card">
          <div class="card-title">Architecture Rows</div>
          <div class="card-value">{{len .Data.ArchRows}}</div>
          <div class="card-sub">Isolation dimensions compared</div>
        </div>
        <div class="card">
          <div class="card-title">Feature Rows</div>
          <div class="card-value">{{len .Data.FeatureRows}}</div>
          <div class="card-sub">Capability comparisons</div>
        </div>
        <div class="card">
          <div class="card-title">CLI Mappings</div>
          <div class="card-value">{{len .Data.CLIRows}}</div>
          <div class="card-sub">Command translations</div>
        </div>
        <div class="card">
          <div class="card-title">Live Operations</div>
          <div class="card-value">{{len .Data.CompRows}}</div>
          <div class="card-sub">Side-by-side results</div>
        </div>
        <div class="card">
          <div class="card-title">Educational Lessons</div>
          <div class="card-value">{{len .Data.Annotations}}</div>
          <div class="card-sub">Annotated operations</div>
        </div>
        {{if .Data.Benchmark}}
        <div class="card">
          <div class="card-title">Benchmark Operations</div>
          <div class="card-value">{{len .Data.Benchmark.Results}}</div>
          <div class="card-sub">Timed on both runtimes</div>
        </div>
        {{end}}
      </div>
      <div class="callout">
        <strong>💡 Key insight:</strong> Apple Container boots a <em>separate lightweight VM</em>
        for every container — providing hardware-level isolation but adding VM startup overhead
        (~500ms–2s per container). Podman forks a process inside the shared Podman Machine VM —
        faster per-container startup, but all containers share the same kernel.
      </div>
    </div>

    <!-- TAB 2: ARCHITECTURE -->
    <div id="panel-arch" class="panel">
      <div class="sec-title">Architecture Comparison</div>
      <div class="sec-sub">How each runtime achieves container isolation on macOS Apple Silicon.</div>
      <div style="overflow-x:auto">
      <table class="tbl">
        <thead><tr><th>Dimension</th><th>🍎 Apple Container</th><th>🦭 Podman</th></tr></thead>
        <tbody>
        {{range .Data.ArchRows}}
        <tr><td><strong>{{.Dimension}}</strong></td><td>{{.Apple}}</td><td>{{.Podman}}</td></tr>
        {{end}}
        </tbody>
      </table>
      </div>
    </div>

    <!-- TAB 3: CLI MAPPING -->
    <div id="panel-cli" class="panel">
      <div class="sec-title">CLI Command Mapping</div>
      <div class="sec-sub">
        Equivalent commands. Apple Container uses <code>container &lt;verb&gt;</code>;
        Podman uses <code>podman &lt;verb&gt;</code>.
      </div>
      <div style="overflow-x:auto">
      <table class="tbl">
        <thead><tr><th>Operation</th><th>🍎 Apple Container</th><th>🦭 Podman</th><th>Notes</th></tr></thead>
        <tbody>
        {{range .Data.CLIRows}}
        <tr>
          <td>{{.Operation}}</td>
          <td><code>{{.AppleCLI}}</code></td>
          <td><code>{{.PodmanCLI}}</code></td>
          <td><span style="color:#6c7086;font-size:12px">{{.Notes}}</span></td>
        </tr>
        {{end}}
        </tbody>
      </table>
      </div>
    </div>

    <!-- TAB 4: FEATURES -->
    <div id="panel-features" class="panel">
      <div class="sec-title">Feature Comparison Matrix</div>
      <div class="sec-sub">Capabilities, platform requirements, and behavioural differences.</div>
      <div style="overflow-x:auto">
      <table class="tbl">
        <thead><tr><th>Feature</th><th>🍎 Apple Container</th><th>🦭 Podman</th><th>Notes</th></tr></thead>
        <tbody>
        {{range .Data.FeatureRows}}
        <tr>
          <td><strong>{{.Feature}}</strong></td>
          <td>{{.Apple}}</td>
          <td>{{.Podman}}</td>
          <td><span style="color:#6c7086;font-size:12px">{{.Notes}}</span></td>
        </tr>
        {{end}}
        </tbody>
      </table>
      </div>
    </div>

    <!-- TAB 5: LIVE RESULTS -->
    <div id="panel-live" class="panel">
      <div class="sec-title">Live Operation Results</div>
      {{if hasData (len .CompStatusRows)}}
      <div class="sec-sub">Identical operations executed against both runtimes in sequence.</div>
      <div style="overflow-x:auto">
      <table class="tbl">
        <thead><tr>
          <th>Operation</th>
          <th>🍎 Command</th><th>🍎 Status</th><th>🍎 Duration</th>
          <th>🦭 Command</th><th>🦭 Status</th><th>🦭 Duration</th>
        </tr></thead>
        <tbody>
        {{range .CompStatusRows}}
        <tr>
          <td><strong>{{.Op}}</strong></td>
          <td><code>{{.AppleCmd}}</code></td>
          <td><span class="badge {{.AppleBadge}}">{{.AppleStatus}}</span></td>
          <td>{{.AppleDuration}}</td>
          <td><code>{{.PodmanCmd}}</code></td>
          <td><span class="badge {{.PodmanBadge}}">{{.PodmanStatus}}</span></td>
          <td>{{.PodmanDuration}}</td>
        </tr>
        {{end}}
        </tbody>
      </table>
      </div>
      {{else}}
      <div class="callout">
        No live operations were executed — neither runtime was reachable.<br>
        Install Apple Container and/or Podman, then re-run:
        <code>container-compare dashboard</code>
      </div>
      {{end}}
    </div>

    <!-- TAB 6: BENCHMARKS -->
    <div id="panel-bench" class="panel">
      <div class="sec-title">Benchmark Results</div>
      {{if .Data.Benchmark}}
      <div class="sec-sub">
        Mean, min, and max timings per operation.
        Image: <code>{{.Data.Benchmark.Image}}</code>
      </div>
      {{range .BenchRows}}
      <div class="bench-block">
        <div class="bench-op-title">{{.OpName}}</div>
        <!-- SVG horizontal bar chart — built entirely from integer fields, no raw HTML injection -->
        <svg xmlns="http://www.w3.org/2000/svg" width="480" height="{{.SVGHeight}}"
             role="img" aria-label="Bar chart for {{.OpName}}">
          <rect width="480" height="{{.SVGHeight}}" fill="#1e1e2e" rx="6"/>
          <!-- Apple bar label -->
          <text x="88" y="30" fill="#cdd6f4" font-size="12"
                font-family="monospace" text-anchor="end">Apple</text>
          <!-- Apple bar -->
          {{if gt .AppleBarW 0}}
          <rect x="94" y="13" width="{{.AppleBarW}}" height="22" fill="#5b9bd5" rx="3"/>
          {{end}}
          <text x="{{barX 94 .AppleBarW}}" y="30" fill="#cdd6f4"
                font-size="11" font-family="monospace">{{.AppleLabel}}</text>
          <!-- Podman bar label -->
          <text x="88" y="70" fill="#cdd6f4" font-size="12"
                font-family="monospace" text-anchor="end">Podman</text>
          <!-- Podman bar -->
          {{if gt .PodmanBarW 0}}
          <rect x="94" y="53" width="{{.PodmanBarW}}" height="22" fill="#f38ba8" rx="3"/>
          {{end}}
          <text x="{{barX 94 .PodmanBarW}}" y="70" fill="#cdd6f4"
                font-size="11" font-family="monospace">{{.PodmanLabel}}</text>
          <!-- Axis labels -->
          <text x="94" y="94" fill="#6c7086" font-size="10"
                font-family="monospace">0ms</text>
          <text x="414" y="94" fill="#6c7086" font-size="10"
                font-family="monospace" text-anchor="end">{{fmtMS (max .AppleMS .PodmanMS)}}</text>
        </svg>
      </div>
      {{end}}
      <div style="overflow-x:auto;margin-top:16px">
      <table class="tbl">
        <thead><tr>
          <th>Operation</th><th>Runtime</th>
          <th>Iterations</th><th>Mean</th><th>Min</th><th>Max</th><th>Errors</th>
        </tr></thead>
        <tbody>
        {{range .BenchRows}}
        <tr>
          <td rowspan="2"><strong>{{.OpName}}</strong></td>
          <td><span class="rt-apple">🍎 Apple</span></td>
          <td>{{.AppleIter}}</td>
          <td>{{fmtMS .AppleMS}}</td>
          <td>{{fmtMS .AppleMinMS}}</td>
          <td>{{fmtMS .AppleMaxMS}}</td>
          <td>{{if gt .AppleErrors 0}}<span class="badge badge-err">{{.AppleErrors}}</span>{{else}}—{{end}}</td>
        </tr>
        <tr>
          <td><span class="rt-podman">🦭 Podman</span></td>
          <td>{{.PodmanIter}}</td>
          <td>{{fmtMS .PodmanMS}}</td>
          <td>{{fmtMS .PodmanMinMS}}</td>
          <td>{{fmtMS .PodmanMaxMS}}</td>
          <td>{{if gt .PodmanErrors 0}}<span class="badge badge-err">{{.PodmanErrors}}</span>{{else}}—{{end}}</td>
        </tr>
        {{end}}
        </tbody>
      </table>
      </div>
      {{else}}
      <div class="callout">
        No benchmark data available. Run <code>container-compare dashboard</code>
        with at least one runtime installed to collect timing data.
      </div>
      {{end}}
    </div>

    <!-- TAB 7: LESSONS -->
    <div id="panel-lessons" class="panel">
      <div class="sec-title">Educational Annotations</div>
      <div class="sec-sub">Key conceptual differences explained for each major container operation.</div>
      {{range $i, $a := .Data.Annotations}}
      <div class="anno-card">
        <div class="anno-title">
          <span class="anno-num">{{add1 $i}}</span>{{$a.Title}}
        </div>
        <div class="anno-lesson">{{$a.Lesson}}</div>
        <div class="anno-cli">
          <div class="anno-cli-label">🍎 Apple Container</div>
          <code>{{$a.AppleCLI}}</code>
        </div>
        <div class="anno-cli" style="margin-top:8px">
          <div class="anno-cli-label">🦭 Podman</div>
          <code>{{$a.PodmanCLI}}</code>
        </div>
      </div>
      {{end}}
    </div>

  </div><!-- tab-content -->
</div><!-- tabs -->

<div class="footer">
  Generated by container-compare v{{.Data.ToolVersion}} · Apple Container v1.3.1 ·
  <a href="https://github.com/apple/container" target="_blank" rel="noopener">github.com/apple/container</a>
  &nbsp;·&nbsp; Made with IBM Bob
</div>
</body>
</html>`

// max returns the larger of two float64 values.
// (Named differently from math.Max to avoid import just for the template func.)
func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

