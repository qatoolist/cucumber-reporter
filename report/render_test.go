package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the current renderer")

func loadFixture(t testing.TB, name string) []cucumber.Feature {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "testdata", "synthetic", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	features, err := cucumber.Parse(f, cucumber.Limits{})
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return features
}

func mustRender(t testing.TB, features []cucumber.Feature, opt Options) string {
	t.Helper()
	out, err := Render(features, opt)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(out)
}

// sampleOptions mirrors how a test framework would call the library: run facts and caller sections,
// one with a chart, and one chart hidden by default. All values are invented.
func sampleOptions() Options {
	return Options{
		Version: 1,
		Title:   "Example suite",
		Name:    "Example suite execution results",
		RunFacts: []Fact{
			{"Started", "2026-01-02T03:04:05Z"},
			{"Revision", "0123456789ab"},
			{"Go version", "go1.27.0"},
		},
		Sections: []Section{
			{
				ID: "known-issues", Title: "Known issues", Tone: ToneWarn,
				Summary: []Fact{{"Run status", "completed"}, {"Known failures", "2"}, {"Unexpected failures", "0"}},
				Rows: []Fact{
					{"EX-101 Order processing / Reject an order with quantity 0", "known failure"},
					{"EX-101 Order processing / Reject an order with quantity -1", "known failure"},
				},
				Note:  "Known failures are listed apart from raw step status.",
				Chart: &ChartSeries{Title: "Known failures compared with failures", Segments: []Segment{{"Known failures", 2, ToneWarn}, {"Unexpected failures", 0, ToneFail}}},
			},
			{
				ID: "skipped", Title: "Skipped scenarios", Tone: ToneInfo,
				Rows: []Fact{{"Billing / Invoice is pending implementation", "Scenario skipped by Godog"}},
			},
		},
		Defaults: ViewDefaults{Charts: map[ChartID]bool{ChartSlowestScenarios: false}},
	}
}

func TestGolden(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		opt     Options
	}{
		{"godog-v0.16.0", "godog-v0.16.0.json", sampleOptions()},
		{"minimal", "minimal.json", Options{Version: 1}},
		{"minimal-defaults", "minimal.json", Options{Version: 1, Title: "Defaults", Defaults: ViewDefaults{
			Charts: map[ChartID]bool{ChartStepStatus: false, ChartFeatureDuration: false},
			Theme:  ThemeDark, Density: DensityCompact, Expand: ExpandAll, PageSize: PageSizeAll,
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustRender(t, loadFixture(t, tc.fixture), tc.opt)
			path := filepath.Join("..", "testdata", "golden", tc.name+".html")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run go test ./report -run TestGolden -update to create it)", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s; inspect the diff, then run go test ./report -run TestGolden -update", path)
			}
		})
	}
}

func TestDeterministic(t *testing.T) {
	features := loadFixture(t, "godog-v0.16.0.json")
	a := mustRender(t, features, sampleOptions())
	b := mustRender(t, features, sampleOptions())
	if a != b {
		t.Fatal("two renders of the same input differ")
	}
}

var cspPattern = regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]*)">`)

func inlineElement(t *testing.T, doc, tag string) string {
	t.Helper()
	open, end := "<"+tag+">", "</"+tag+">"
	if strings.Count(doc, "<"+tag) != 1 || strings.Count(doc, end) != 1 {
		t.Fatalf("want exactly one <%s> element", tag)
	}
	start := strings.Index(doc, open)
	if start < 0 {
		t.Fatalf("<%s> has attributes; want a bare element", tag)
	}
	start += len(open)
	return doc[start : start+strings.Index(doc[start:], end)]
}

func hashSource(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// TestCSPMatchesInlineAssets: the policy pins exactly the inline bytes, which are exactly the
// hand-written asset files.
func TestCSPMatchesInlineAssets(t *testing.T) {
	doc := mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions())
	m := cspPattern.FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("no CSP meta tag")
	}
	policy := html.UnescapeString(m[1])
	style, script := inlineElement(t, doc, "style"), inlineElement(t, doc, "script")
	want := "default-src 'none'; script-src " + hashSource(script) + "; style-src " + hashSource(style) +
		"; img-src data:; base-uri 'none'; form-action 'none'; object-src 'none'"
	if policy != want {
		t.Errorf("policy = %q\nwant     %q", policy, want)
	}
	for file, inline := range map[string]string{"report.css": style, "report.js": script} {
		disk, err := os.ReadFile(filepath.Join("assets", file))
		if err != nil {
			t.Fatal(err)
		}
		if string(disk) != inline {
			t.Errorf("inline %s differs from assets/%s", file, file)
		}
	}
	if !strings.Contains(doc, `<meta name="referrer" content="no-referrer">`) {
		t.Error("no referrer policy")
	}
	if strings.Index(doc, "Content-Security-Policy") > strings.Index(doc, "<style>") {
		t.Error("CSP must precede the inline style")
	}
	for _, banned := range []string{" style=", "javascript:", "<link", "<iframe", "<object", "<embed", "<img", "<base", "<form"} {
		if strings.Contains(doc, banned) {
			t.Errorf("document contains %q", banned)
		}
	}
}

func TestAssetBudgets(t *testing.T) {
	for file, max := range map[string]int{"report.css": 12 << 10, "report.js": 8 << 10} {
		info, err := os.Stat(filepath.Join("assets", file))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d bytes (target %d)", file, info.Size(), max)
		if info.Size() > int64(max) {
			t.Errorf("%s is %d bytes; target %d", file, info.Size(), max)
		}
	}
	doc := mustRender(t, loadFixture(t, "minimal.json"), Options{Version: 1})
	t.Logf("minimal report: %d bytes (target 25600)", len(doc))
	if len(doc) > 25<<10 {
		t.Errorf("minimal report is %d bytes; static overhead target 25 KiB", len(doc))
	}
}

func TestOptionsValidation(t *testing.T) {
	ok := func(mod func(*Options)) Options { o := sampleOptions(); mod(&o); return o }
	tests := []struct {
		name string
		opt  Options
		want error
	}{
		{"zero version", ok(func(o *Options) { o.Version = 0 }), ErrOptionsVersion},
		{"future version", ok(func(o *Options) { o.Version = 2 }), ErrOptionsVersion},
		{"section id with capitals", ok(func(o *Options) { o.Sections[0].ID = "Known" }), ErrInvalidOptions},
		{"section id with quote", ok(func(o *Options) { o.Sections[0].ID = `a"b` }), ErrInvalidOptions},
		{"section id empty", ok(func(o *Options) { o.Sections[0].ID = "" }), ErrInvalidOptions},
		{"section id too long", ok(func(o *Options) { o.Sections[0].ID = "a" + strings.Repeat("b", 48) }), ErrInvalidOptions},
		{"duplicate section", ok(func(o *Options) { o.Sections[1].ID = o.Sections[0].ID }), ErrInvalidOptions},
		{"untitled section", ok(func(o *Options) { o.Sections[0].Title = "" }), ErrInvalidOptions},
		{"bad tone", ok(func(o *Options) { o.Sections[0].Tone = "red" }), ErrInvalidOptions},
		{"bad segment tone", ok(func(o *Options) { o.Sections[0].Chart.Segments[0].Tone = "x" }), ErrInvalidOptions},
		{"negative segment", ok(func(o *Options) { o.Sections[0].Chart.Segments[0].Value = -1 }), ErrInvalidOptions},
		{"oversize segment", ok(func(o *Options) { o.Sections[0].Chart.Segments[0].Value = 1_000_000_001 }), ErrInvalidOptions},
		{"unknown chart", ok(func(o *Options) { o.Defaults.Charts = map[ChartID]bool{"pie": true} }), ErrInvalidOptions},
		{"chart of a section without one", ok(func(o *Options) { o.Defaults.Charts = map[ChartID]bool{SectionChartID("skipped"): true} }), ErrInvalidOptions},
		{"unknown section", ok(func(o *Options) { o.Defaults.Sections = map[string]bool{"other": false} }), ErrInvalidOptions},
		{"bad theme", ok(func(o *Options) { o.Defaults.Theme = "blue" }), ErrInvalidOptions},
		{"bad density", ok(func(o *Options) { o.Defaults.Density = "tight" }), ErrInvalidOptions},
		{"bad expand", ok(func(o *Options) { o.Defaults.Expand = "some" }), ErrInvalidOptions},
		{"bad page size", ok(func(o *Options) { o.Defaults.PageSize = 10 }), ErrInvalidOptions},
		{"section chart default", ok(func(o *Options) { o.Defaults.Charts = map[ChartID]bool{SectionChartID("known-issues"): false} }), nil},
		{"section default", ok(func(o *Options) { o.Defaults.Sections = map[string]bool{"skipped": false} }), nil},
		{"every page size", ok(func(o *Options) { o.Defaults.PageSize = PageSizeAll }), nil},
	}
	features := loadFixture(t, "minimal.json")
	for _, tt := range tests {
		_, err := Render(features, tt.opt)
		if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
		if err != nil && strings.Contains(err.Error(), `a"b`) {
			t.Errorf("%s: error echoes option content", tt.name)
		}
	}
}

func TestRenderRejectsInvalidModel(t *testing.T) {
	if _, err := Render(nil, Options{Version: 1}); !errors.Is(err, cucumber.ErrNoResults) {
		t.Errorf("nil features: %v", err)
	}
	bad := []cucumber.Feature{{Elements: []cucumber.Scenario{{Steps: []cucumber.Step{{Result: cucumber.Result{Status: "exploded"}}}}}}}
	if _, err := Render(bad, Options{Version: 1}); !errors.Is(err, cucumber.ErrInvalid) {
		t.Errorf("invalid status: %v", err)
	}
	empty := []cucumber.Feature{{Name: "F"}}
	if _, err := Render(empty, Options{Version: 1}); !errors.Is(err, cucumber.ErrIncomplete) {
		t.Errorf("feature without scenarios: %v", err)
	}
}

func countOpen(doc string) (open, total int) {
	for _, m := range regexp.MustCompile(`<details class="scenario[^"]*"[^>]*>`).FindAllString(doc, -1) {
		total++
		if strings.Contains(m, " open") {
			open++
		}
	}
	return open, total
}

func TestViewDefaultsReachThePage(t *testing.T) {
	features := loadFixture(t, "godog-v0.16.0.json")
	doc := mustRender(t, features, sampleOptions())
	for _, want := range []string{
		`<html lang="en" data-theme="auto">`,
		`data-theme="auto" data-density="comfortable" data-expand="failed" data-page-size="50"`,
		`data-hidden-charts="slowest-scenarios" data-hidden-sections=""`,
		`<figure class="chart" id="chart-slowest-scenarios" data-chart="slowest-scenarios" hidden>`,
		`<figure class="chart" id="chart-section-known-issues" data-chart="section-known-issues">`,
		`<section class="card tone-warn" id="sec-known-issues" data-section="known-issues" aria-labelledby="hd-sec-known-issues">`,
		`data-toggle="chart:slowest-scenarios"`,
		`data-toggle="section:skipped" checked`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("missing %s", want)
		}
	}
	if open, total := countOpen(doc); open != 2 || total != 6 {
		t.Errorf("expand failed: %d of %d open, want 2 of 6", open, total)
	}

	opt := sampleOptions()
	opt.Defaults = ViewDefaults{Theme: ThemeDark, Density: DensityCompact, Expand: ExpandAll, PageSize: 25,
		Sections: map[string]bool{"known-issues": false}, Charts: map[ChartID]bool{ChartStepStatus: false, ChartScenarioStatus: true}}
	doc = mustRender(t, features, opt)
	for _, want := range []string{
		`<html lang="en" data-theme="dark">`,
		`<body class="compact"`,
		`data-theme="dark" data-density="compact" data-expand="all" data-page-size="25"`,
		`data-hidden-charts="step-status" data-hidden-sections="known-issues"`,
		`id="sec-known-issues" data-section="known-issues" aria-labelledby="hd-sec-known-issues" hidden>`,
		`data-toggle="section:known-issues">`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("missing %s", want)
		}
	}
	if open, total := countOpen(doc); open != total {
		t.Errorf("expand all: %d of %d open", open, total)
	}
	opt.Defaults.Expand, opt.Defaults.PageSize = ExpandNone, PageSizeAll
	doc = mustRender(t, features, opt)
	if open, _ := countOpen(doc); open != 0 {
		t.Errorf("expand none: %d open", open)
	}
	if !strings.Contains(doc, `data-page-size="0"`) {
		t.Error("page size all should reach the page as 0")
	}
}

func TestSummaryCounts(t *testing.T) {
	doc := mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions())
	for _, want := range []string{
		`<h2 class="verdict st-failed"><span class="glyph" aria-hidden="true">✘</span> Failed</h2>`,
		`<div class="tile" data-kind="features"><h3>Features</h3><p class="total">2</p>`,
		`<div class="tile" data-kind="scenarios"><h3>Scenarios</h3><p class="total">6</p>`,
		`<div class="tile" data-kind="steps"><h3>Steps</h3><p class="total">15</p>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func step(name string, status cucumber.Status, ns int64, emb ...cucumber.Embedding) cucumber.Step {
	s := cucumber.Step{Keyword: "Given ", Name: name, Result: cucumber.Result{Status: status}, Embeddings: emb}
	if ns >= 0 {
		d := time.Duration(ns)
		s.Result.Duration = &d
	}
	return s
}

func one(steps ...cucumber.Step) []cucumber.Feature {
	return []cucumber.Feature{{Name: "F", Keyword: "Feature", URI: "f.feature", Elements: []cucumber.Scenario{{Name: "S", Keyword: "Scenario", Type: "scenario", Steps: steps}}}}
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestAttachments(t *testing.T) {
	features := one(step("s", cucumber.StatusPassed, 1,
		cucumber.Embedding{Name: "note.txt", MimeType: "text/plain", Data: b64("line 1\n  line <2> & \"3\"\n")},
		cucumber.Embedding{Name: "data.json", MimeType: "application/json", Data: b64(`{"b":[1,2],"a":"<x>"}`)},
		cucumber.Embedding{Name: "page.html", MimeType: "text/html; charset=utf-8", Data: b64(`<b onclick="x()">bold</b>`)},
		cucumber.Embedding{Name: "broken.json", MimeType: "application/json", Data: b64(`{"a":`)},
		cucumber.Embedding{Name: "pixel.png", MimeType: "image/png", Data: b64("\x89PNG\x00\x01")},
		cucumber.Embedding{Name: "bad.txt", MimeType: "text/plain", Data: "%%%"},
		cucumber.Embedding{Name: "latin1.txt", MimeType: "text/plain", Data: b64("caf\xe9")},
	))
	doc := mustRender(t, features, Options{Version: 1})
	text := html.UnescapeString(doc)
	for _, want := range []string{
		"line 1\n  line <2> & \"3\"\n",
		"{\n  \"b\": [\n    1,\n    2\n  ],\n  \"a\": \"<x>\"\n}",
		`<b onclick="x()">bold</b>`,
		`{"a":`,
		"Not valid JSON; shown as text.",
		"pixel.png",
		"image/png · 6 B · content not shown",
		"Attachment data is not valid base64; content not shown.",
		"text/plain · 4 B · content not shown",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(doc, `<b onclick`) || strings.Contains(doc, "\x89PNG") {
		t.Error("attachment content reached the page unescaped or binary content was shown")
	}
	if n := strings.Count(doc, `data-copy=`); n != 4 {
		t.Errorf("copy buttons = %d, want 4 (one per shown text attachment)", n)
	}
}

func TestOversizeAttachmentLeftOutByRender(t *testing.T) {
	big := strings.Repeat("x", 2048)
	features := one(step("s", cucumber.StatusPassed, 1, cucumber.Embedding{Name: "big.txt", MimeType: "text/plain", Data: b64(big)}))
	doc := mustRender(t, features, Options{Version: 1, Limits: cucumber.Limits{InlineAttachment: 1024}})
	if strings.Contains(doc, big) {
		t.Error("oversize attachment was inlined")
	}
	if !strings.Contains(doc, "Larger than the 1.0 KiB inline limit; content not included.") {
		t.Error("no notice for the oversize attachment")
	}
}

// syntheticRun builds features × scenarios × steps with failures, skips, tables and small attachments.
func syntheticRun(features, scenarios, steps int) []cucumber.Feature {
	out := make([]cucumber.Feature, features)
	for f := range out {
		out[f] = cucumber.Feature{URI: fmt.Sprintf("features/area-%02d.feature", f), Keyword: "Feature", Name: fmt.Sprintf("Area %02d", f), Line: 1}
		for s := 0; s < scenarios; s++ {
			sc := cucumber.Scenario{ID: fmt.Sprintf("area-%02d;case-%02d", f, s), Keyword: "Scenario", Name: fmt.Sprintf("Case %02d checks a synthetic behaviour", s), Type: "scenario", Line: 3 + s*12, Tags: []cucumber.Tag{{Name: "@synthetic", Line: 2}}}
			for k := 0; k < steps; k++ {
				st := step(fmt.Sprintf("step %d of case %d sends request %d", k, s, f*1000+s*10+k), cucumber.StatusPassed, int64((f*7+s*13+k*17)%997)*1_000_000)
				switch {
				case s == 3 && k == 5:
					st.Result.Status, st.Result.Error = cucumber.StatusFailed, "expected status 200, got 503"
				case s == 3 && k > 5:
					st.Result.Status, st.Result.Duration = cucumber.StatusSkipped, nil
				case k == 1:
					st.Rows = []cucumber.Row{{Cells: []string{"field", "value"}}, {Cells: []string{"id", fmt.Sprint(k)}}}
				case k == 2:
					st.Embeddings = []cucumber.Embedding{{Name: "environment.txt", MimeType: "text/plain", Data: b64("target: synthetic\n")}}
				}
				sc.Steps = append(sc.Steps, st)
			}
			out[f].Elements = append(out[f].Elements, sc)
		}
	}
	return out
}

// TestLargeRun: 5,000 steps stay within a size bound. Render time is measured with testing.Benchmark
// and logged, not asserted, so the test does not depend on the machine's speed.
func TestLargeRun(t *testing.T) {
	features := syntheticRun(50, 10, 10)
	doc := mustRender(t, features, Options{Version: 1})
	const maxBytes = 6 << 20
	t.Logf("5,000 steps: %d bytes (%.1f bytes per step)", len(doc), float64(len(doc))/5000)
	if len(doc) > maxBytes {
		t.Errorf("5,000-step report is %d bytes; bound %d", len(doc), maxBytes)
	}
	if testing.Short() {
		return
	}
	r := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := Render(features, Options{Version: 1}); err != nil {
				b.Fatal(err)
			}
		}
	})
	t.Logf("5,000 steps: %s per render, %d B allocated per render", time.Duration(r.NsPerOp()), r.AllocedBytesPerOp())
}

// TestAttachmentHeavyRun approximates the largest known real run's shape with invented content:
// ten 175 KB JSON attachments (about 233 KB of base64 each).
func TestAttachmentHeavyRun(t *testing.T) {
	var entries []map[string]any
	for i := 0; len(entries) < 1800; i++ {
		entries = append(entries, map[string]any{"id": fmt.Sprintf("res-%05d", i), "kind": "synthetic-resource", "state": "retained", "size": i * 17, "labels": []string{"alpha", "beta"}})
	}
	payload, _ := json.MarshalIndent(map[string]any{"resources": entries}, "", "  ")
	var steps []cucumber.Step
	decoded := 0
	for i := 0; i < 10; i++ {
		steps = append(steps, step(fmt.Sprintf("evidence %d is retained", i), cucumber.StatusPassed, 1000,
			cucumber.Embedding{Name: "retained-evidence.json", MimeType: "application/json", Data: base64.StdEncoding.EncodeToString(payload)}))
		decoded += len(payload)
	}
	doc := mustRender(t, one(steps...), Options{Version: 1})
	ratio := float64(len(doc)) / float64(decoded)
	t.Logf("attachment-heavy run: %d decoded bytes, %d base64 bytes, %d HTML bytes, ratio %.2f to decoded, %.2f to base64",
		decoded, decoded*4/3, len(doc), ratio, float64(len(doc))/float64(decoded*4/3))
	if ratio > 2.5 {
		t.Errorf("HTML is %.2f× the decoded attachments; bound 2.5", ratio)
	}
	if !bytes.Contains([]byte(html.UnescapeString(doc)), payload) {
		t.Error("pretty-printed attachment not shown exactly")
	}
}
