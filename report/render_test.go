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
	"slices"
	"strings"
	"testing"
	"time"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
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

func mustRender(t testing.TB, features []cucumber.Feature, opt renderOptions) string {
	t.Helper()
	out, err := renderPublicModel(features, opt)
	if err != nil {
		t.Fatalf("renderPublicModel: %v", err)
	}
	return string(out)
}

// sampleOptions mirrors how a test framework would call the library: run facts, caller sections with
// roles, and one widget and one navigation section hidden by default. All values are invented.
func sampleOptions() renderOptions {
	return renderOptions{
		Version: OptionsVersion,
		Title:   "Example suite",
		Name:    "Example suite execution results",
		RunFacts: []Fact{
			{Label: "Started", Value: "2026-01-02T03:04:05Z"},
			{Label: "Revision", Value: "0123456789ab"},
			{Label: "Go version", Value: "go1.27.0"},
		},
		Sections: []Section{
			{
				ID: "known-issues", Title: "Known issues", Tone: ToneWarn, Role: RoleKnownIssues,
				Summary: []Fact{{Label: "Run status", Value: "completed"}, {Label: "Known failures", Value: "2"}, {Label: "Unexpected failures", Value: "0"}},
				Rows: []Fact{
					{Label: "EX-101 Order processing / Reject an order with quantity 0", Value: "known failure"},
					{Label: "EX-101 Order processing / Reject an order with quantity -1", Value: "known failure"},
				},
				Note: "Known failures are listed apart from raw step status.",
			},
			{
				ID: "skipped", Title: "Skipped scenarios", Role: RoleSkipped,
				Rows: []Fact{{Label: "Billing / Invoice is pending implementation", Value: "Scenario skipped by Godog"}},
			},
		},
		Defaults: ViewDefaults{Widgets: map[string]bool{WidgetDuration: false}, Nav: map[string]bool{NavTimeline: false}},
	}
}

// loadShowcase reads the synthetic showcase run and its options file (the command's -options format).
func loadShowcase(t testing.TB, suffix string) ([]cucumber.Feature, renderOptions) {
	t.Helper()
	features := loadFixture(t, filepath.Join("showcase", "cucumber"+suffix+".json"))
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "synthetic", "showcase", "options"+suffix+".json"))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var opt renderOptions
	if err := dec.Decode(&opt); err != nil {
		t.Fatalf("options%s.json: %v", suffix, err)
	}
	return features, opt
}

func TestGolden(t *testing.T) {
	showcase, showcaseOpt := loadShowcase(t, "")
	cases := []struct {
		name     string
		features []cucumber.Feature
		opt      renderOptions
	}{
		{"godog-v0.16.0", loadFixture(t, "godog-v0.16.0.json"), sampleOptions()},
		{"minimal", loadFixture(t, "minimal.json"), renderOptions{Version: OptionsVersion}},
		{"minimal-defaults", loadFixture(t, "minimal.json"), renderOptions{Version: OptionsVersion, Title: "Defaults", Defaults: ViewDefaults{
			Theme: ThemeDark, Density: DensityCompact, Expand: ExpandAll, View: ViewFailures,
			Widgets: map[string]bool{WidgetStatus: false}, Nav: map[string]bool{NavRun: false},
		}}},
		{"showcase", showcase, showcaseOpt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustRender(t, tc.features, tc.opt)
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
			for name, pattern := range map[string]string{"data": `(?s)<script[^>]*id="report-data"[^>]*>(.*?)</script>`, "fallback": `(?s)<noscript>(.*?)</noscript>`} {
				re := regexp.MustCompile(pattern)
				actual, expected := re.FindStringSubmatch(got), re.FindStringSubmatch(string(want))
				if len(actual) != 2 || len(expected) != 2 || actual[1] != expected[1] {
					t.Errorf("%s golden differs for %s", name, tc.name)
				}
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

// bareElement returns the content of the only <tag> element written without attributes.
func bareElement(t *testing.T, doc, tag string) string {
	t.Helper()
	open, end := "<"+tag+">", "</"+tag+">"
	if strings.Count(doc, open) != 1 {
		t.Fatalf("want exactly one bare <%s> element", tag)
	}
	start := strings.Index(doc, open) + len(open)
	return doc[start : start+strings.Index(doc[start:], end)]
}

func hashSource(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// TestCSPMatchesInlineAssets: the policy pins exactly the inline bytes, which are exactly the
// committed minified asset files.
func TestCSPMatchesInlineAssets(t *testing.T) {
	doc := mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions())
	m := cspPattern.FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("no CSP meta tag")
	}
	policy := html.UnescapeString(m[1])
	style, script := bareElement(t, doc, "style"), bareElement(t, doc, "script")
	want := "default-src 'none'; script-src " + hashSource(script) + "; style-src " + hashSource(style) +
		"; img-src data:; base-uri 'none'; form-action 'none'; object-src 'none'"
	if policy != want {
		t.Errorf("policy = %q\nwant     %q", policy, want)
	}
	for file, inline := range map[string]string{"app.min.css": style, "app.min.js": script} {
		disk, err := os.ReadFile(filepath.Join("assets", file))
		if err != nil {
			t.Fatal(err)
		}
		if string(disk) != inline {
			t.Errorf("inline content differs from assets/%s", file)
		}
	}
	if !strings.Contains(doc, `<meta name="referrer" content="no-referrer">`) {
		t.Error("no referrer policy")
	}
	if strings.Index(doc, "Content-Security-Policy") > strings.Index(doc, "<style>") {
		t.Error("CSP must precede the inline style")
	}
	if strings.Count(doc, "<script") != 2 || !strings.Contains(doc, `<script type="application/json" id="report-data">`) {
		t.Error("want the data block and the app script only")
	}
	for _, token := range tokenize(t, doc) {
		if token.kind != 's' {
			continue
		}
		for _, attribute := range token.attrs {
			if attribute.name == "style" {
				t.Error("document contains an inline style attribute")
			}
		}
	}
	for _, banned := range []string{"javascript:", "<link", "<iframe", "<object", "<embed", "<img", "<base", "<form", "unsafe-"} {
		if strings.Contains(doc, banned) {
			t.Errorf("document contains %q", banned)
		}
	}
}

// TestAssetSizes reports the embedded sizes; there is no budget to fail.
func TestAssetSizes(t *testing.T) {
	for _, pair := range [][2]string{{"app.js", "app.min.js"}, {"app.css", "app.min.css"}} {
		src, err := os.ReadFile(filepath.Join("assets", pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d bytes readable, %d bytes embedded (%s)", pair[0], len(src), len(assetText(pair[1])), pair[1])
	}
	doc := mustRender(t, loadFixture(t, "minimal.json"), renderOptions{Version: OptionsVersion})
	t.Logf("minimal report: %d bytes", len(doc))
}

func assetText(name string) string {
	if strings.HasSuffix(name, ".js") {
		return scriptSource
	}
	return styleSource
}

func TestOptionsValidation(t *testing.T) {
	ok := func(mod func(*renderOptions)) renderOptions { o := sampleOptions(); mod(&o); return o }
	tests := []struct {
		name string
		opt  renderOptions
		want error
	}{
		{"zero version", ok(func(o *renderOptions) { o.Version = 0 }), ErrOptionsVersion},
		{"version 1", ok(func(o *renderOptions) { o.Version = 1 }), ErrOptionsVersion},
		{"future version", ok(func(o *renderOptions) { o.Version = 3 }), ErrOptionsVersion},
		{"section id with capitals", ok(func(o *renderOptions) { o.Sections[0].ID = "Known" }), ErrInvalidOptions},
		{"section id with quote", ok(func(o *renderOptions) { o.Sections[0].ID = `a"b` }), ErrInvalidOptions},
		{"section id empty", ok(func(o *renderOptions) { o.Sections[0].ID = "" }), ErrInvalidOptions},
		{"section id too long", ok(func(o *renderOptions) { o.Sections[0].ID = "a" + strings.Repeat("b", 48) }), ErrInvalidOptions},
		{"duplicate section", ok(func(o *renderOptions) { o.Sections[1].ID = o.Sections[0].ID }), ErrInvalidOptions},
		{"untitled section", ok(func(o *renderOptions) { o.Sections[0].Title = "" }), ErrInvalidOptions},
		{"bad tone", ok(func(o *renderOptions) { o.Sections[0].Tone = "red" }), ErrInvalidOptions},
		{"bad role", ok(func(o *renderOptions) { o.Sections[0].Role = "issues" }), ErrInvalidOptions},
		{"two known-issue sections", ok(func(o *renderOptions) { o.Sections[1].Role = RoleKnownIssues }), ErrInvalidOptions},
		{"verdict without label", ok(func(o *renderOptions) { o.Verdict = &Verdict{Tone: ToneOK} }), ErrInvalidOptions},
		{"verdict bad tone", ok(func(o *renderOptions) { o.Verdict = &Verdict{Label: "x", Tone: "green"} }), ErrInvalidOptions},
		{"top fact unknown", ok(func(o *renderOptions) { o.TopFacts = []string{"Branch"} }), ErrInvalidOptions},
		{"four top facts", ok(func(o *renderOptions) { o.TopFacts = []string{"Started", "Revision", "Go version", "Started"} }), ErrInvalidOptions},
		{"unknown widget", ok(func(o *renderOptions) { o.Defaults.Widgets = map[string]bool{"pie": true} }), ErrInvalidOptions},
		{"unknown nav", ok(func(o *renderOptions) { o.Defaults.Nav = map[string]bool{"help": true} }), ErrInvalidOptions},
		{"hidden dashboard", ok(func(o *renderOptions) { o.Defaults.Nav = map[string]bool{NavDashboard: false} }), ErrInvalidOptions},
		{"bad theme", ok(func(o *renderOptions) { o.Defaults.Theme = "blue" }), ErrInvalidOptions},
		{"bad density", ok(func(o *renderOptions) { o.Defaults.Density = "tight" }), ErrInvalidOptions},
		{"bad expand", ok(func(o *renderOptions) { o.Defaults.Expand = "some" }), ErrInvalidOptions},
		{"bad view", ok(func(o *renderOptions) { o.Defaults.View = "timeline" }), ErrInvalidOptions},
		{"every view", ok(func(o *renderOptions) { o.Defaults.View = ViewScenarios }), nil},
		{"dashboard shown explicitly", ok(func(o *renderOptions) { o.Defaults.Nav = map[string]bool{NavDashboard: true} }), nil},
		{"verdict with tone", ok(func(o *renderOptions) { o.Verdict = &Verdict{Label: "Passed", Tone: ToneOK} }), nil},
	}
	features := loadFixture(t, "minimal.json")
	for _, tt := range tests {
		_, err := renderPublicModel(features, tt.opt)
		if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
		if err != nil && strings.Contains(err.Error(), `a"b`) {
			t.Errorf("%s: error echoes option content", tt.name)
		}
	}
}

func TestRenderRejectsInvalidModel(t *testing.T) {
	if _, err := renderPublicModel(nil, renderOptions{Version: OptionsVersion}); err != nil {
		t.Errorf("nil features: %v", err)
	}
	bad := []cucumber.Feature{{Elements: []cucumber.Scenario{{Steps: []cucumber.Step{{Result: cucumber.Result{Status: "exploded"}}}}}}}
	if _, err := renderPublicModel(bad, renderOptions{Version: OptionsVersion}); !errors.Is(err, cucumber.ErrInvalid) {
		t.Errorf("invalid status: %v", err)
	}
	empty := []cucumber.Feature{{Name: "F"}}
	if _, err := renderPublicModel(empty, renderOptions{Version: OptionsVersion}); err != nil {
		t.Errorf("feature without scenarios: %v", err)
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
	return []cucumber.Feature{{Name: "F", Keyword: "Feature", URI: "f.feature", Elements: []cucumber.Scenario{{Name: "S", Keyword: "Scenario", Type: "scenario", Line: 2, Steps: steps}}}}
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// noscript returns the no-JS fallback markup.
func noscript(t testing.TB, doc string) string {
	t.Helper()
	i, j := strings.Index(doc, "<noscript>"), strings.Index(doc, "</noscript>")
	if i < 0 || j < i || strings.Count(doc, "<noscript>") != 1 {
		t.Fatal("want one noscript fallback")
	}
	return doc[i+len("<noscript>") : j]
}

func TestFallback(t *testing.T) {
	features, opt := loadShowcase(t, "")
	fb := noscript(t, mustRender(t, features, opt))
	text := html.UnescapeString(fb)
	for _, want := range []string{
		`<p class="nj-v s-failed">Failed: 3 unexpected failures</p>`,
		"<tr><th>Environment</th><td><bdi>staging-eu-1</bdi></td></tr>",
		`<tr><td class="st s-known">⚑ Known failure</td><td>4</td></tr>`,
		`<tr><td class="st s-failed">✕ Failed</td><td>3</td></tr>`,
		"<tr><th>Total</th><th>40</th></tr>",
		"<h2><bdi>Scope</bdi></h2>",
		"<p>Load and soak scenarios run in a separate suite.</p>",
		"<code>features/orders/order_processing.feature</code> @orders",
		`<details open><summary><span class="st s-failed">✕ Failed</span>`,
		"Attachment: confirm-response.json (application/json, ",
		"Known failure EX-330: Expected failure: no push is sent to that customer",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fallback lacks %q", want)
		}
	}
	// The declared step of the observed expected failure carries the marker; the others keep their status.
	i := strings.Index(text, "Scenario: <bdi>Push notification preferences")
	if i < 0 {
		t.Fatal("no push-preferences scenario")
	}
	sc := text[i : i+strings.Index(text[i:], "</details>")]
	if strings.Count(sc, `class="st s-known"`) != 1 || strings.Count(sc, `class="st s-passed"`) != 2 || !strings.Contains(sc, `s-known">⚑ Known failure</span> <b>Then</b>`) {
		t.Errorf("push-preferences scenario markers wrong: %s", sc)
	}
	for _, banned := range []string{" style=", "<script", "<img"} {
		if strings.Contains(strings.ToLower(fb), banned) {
			t.Errorf("fallback contains %q", banned)
		}
	}
}

// syntheticRun builds features × scenarios × steps with failures, skips, tables and small attachments.
func syntheticRun(features, scenarios, steps int) []cucumber.Feature {
	out := make([]cucumber.Feature, features)
	for f := range out {
		out[f] = cucumber.Feature{URI: fmt.Sprintf("features/area-%02d.feature", f), Keyword: "Feature", Name: fmt.Sprintf("Area %02d", f), Line: 1}
		for s := 0; s < scenarios; s++ {
			sc := cucumber.Scenario{ID: fmt.Sprintf("area-%02d;case-%02d", f, s), Keyword: "Scenario", Name: fmt.Sprintf("Case %02d checks a synthetic behavior", s), Type: "scenario", Line: 3 + s*12, Tags: []cucumber.Tag{{Name: "@synthetic", Line: 2}}}
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

// TestLargeRun: 5,000 steps render and every step reaches the data block. Size and render time are
// logged, not asserted.
func TestLargeRun(t *testing.T) {
	features := syntheticRun(50, 10, 10)
	doc := mustRender(t, features, renderOptions{Version: OptionsVersion})
	d, _ := dataBlock(t, doc)
	n := 0
	for _, f := range d.Features {
		for _, s := range f.Elements {
			n += len(s.Steps)
		}
	}
	if n != 5000 {
		t.Errorf("data block has %d steps, want 5000", n)
	}
	t.Logf("5,000 steps: %d bytes (%.1f bytes per step)", len(doc), float64(len(doc))/5000)
	if testing.Short() {
		return
	}
	r := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := renderPublicModel(features, renderOptions{Version: OptionsVersion}); err != nil {
				b.Fatal(err)
			}
		}
	})
	t.Logf("5,000 steps: %s per render, %d B allocated per render", time.Duration(r.NsPerOp()), r.AllocedBytesPerOp())
}

// TestAttachmentHeavyRun: ten large JSON attachments reach the data block as their exact base64 text.
// The size ratio is logged, not asserted.
func TestAttachmentHeavyRun(t *testing.T) {
	var entries []map[string]any
	for i := 0; len(entries) < 1800; i++ {
		entries = append(entries, map[string]any{"id": fmt.Sprintf("res-%05d", i), "kind": "synthetic-resource", "state": "retained", "size": i * 17, "labels": []string{"alpha", "beta"}})
	}
	payload, _ := json.MarshalIndent(map[string]any{"resources": entries}, "", "  ")
	encoded := base64.StdEncoding.EncodeToString(payload)
	var steps []cucumber.Step
	for i := 0; i < 10; i++ {
		steps = append(steps, step(fmt.Sprintf("evidence %d is retained", i), cucumber.StatusPassed, 1000,
			cucumber.Embedding{Name: "retained-evidence.json", MimeType: "application/json", Data: encoded}))
	}
	doc := mustRender(t, one(steps...), renderOptions{Version: OptionsVersion})
	t.Logf("attachment-heavy run: %d decoded bytes, %d HTML bytes, ratio %.2f to decoded, %.2f to base64",
		10*len(payload), len(doc), float64(len(doc))/float64(10*len(payload)), float64(len(doc))/float64(10*len(encoded)))
	if strings.Count(doc, `"data":"`+encoded+`"`) != 10 {
		t.Error("attachments are not carried as their exact base64 text")
	}
}

func scenarioOf(name string, line int, statuses ...cucumber.Status) cucumber.Scenario {
	sc := cucumber.Scenario{Keyword: "Scenario", Name: name, Type: "scenario", Line: line}
	for _, s := range statuses {
		sc.Steps = append(sc.Steps, step("s", s, -1))
	}
	return sc
}

// TestHeadlineFollowsFeatureRule: a skipped or pending scenario beside passed ones does not turn the
// feature or the run headline skipped or pending; only a run that is all skipped is skipped.
func TestHeadlineFollowsFeatureRule(t *testing.T) {
	tests := []struct {
		name      string
		scenarios []cucumber.Scenario
		verdict   Verdict
	}{
		{"passed and skipped", []cucumber.Scenario{scenarioOf("a", 1, "passed"), scenarioOf("b", 2, "passed", "skipped")}, Verdict{"Passed", ToneOK}},
		{"passed and pending", []cucumber.Scenario{scenarioOf("a", 1, "passed"), scenarioOf("b", 2, "pending")}, Verdict{"Passed", ToneOK}},
		{"all skipped", []cucumber.Scenario{scenarioOf("a", 1, "skipped"), scenarioOf("b", 2, "passed", "skipped")}, Verdict{"Skipped", ToneInfo}},
		{"skipped and pending", []cucumber.Scenario{scenarioOf("a", 1, "skipped"), scenarioOf("b", 2, "pending")}, Verdict{"Skipped", ToneInfo}},
		{"all pending", []cucumber.Scenario{scenarioOf("a", 1, "pending"), scenarioOf("b", 2, "passed", "pending")}, Verdict{"Pending", ToneInfo}},
		{"undefined among passed", []cucumber.Scenario{scenarioOf("a", 1, "passed"), scenarioOf("b", 2, "undefined")}, Verdict{"Undefined", ToneFail}},
		{"ambiguous", []cucumber.Scenario{scenarioOf("a", 1, "ambiguous")}, Verdict{"Ambiguous", ToneFail}},
	}
	for _, tt := range tests {
		features := []cucumber.Feature{{Name: "F", Keyword: "Feature", Elements: tt.scenarios}}
		d, _ := dataBlock(t, mustRender(t, features, renderOptions{Version: OptionsVersion}))
		if d.Verdict != tt.verdict || d.Features[0].Status != strings.ToLower(tt.verdict.Label) {
			t.Errorf("%s: verdict %+v, feature %s; want %+v", tt.name, d.Verdict, d.Features[0].Status, tt.verdict)
		}
	}
}

func TestSteplessScenarioRendersUndefined(t *testing.T) {
	d, _ := dataBlock(t, mustRender(t, loadFixture(t, "godog-v0.16.0-stepless.json"), renderOptions{Version: OptionsVersion}))
	var statuses []string
	for _, s := range d.Features[0].Elements {
		statuses = append(statuses, s.Status)
	}
	if !slices.Contains(statuses, "undefined") {
		t.Errorf("statuses %v; a scenario without steps is undefined", statuses)
	}
}

func TestVerdictOverride(t *testing.T) {
	opt := sampleOptions()
	opt.Verdict = &Verdict{Label: "Passed with known failures", Tone: ToneWarn}
	doc := mustRender(t, loadFixture(t, "godog-v0.16.0.json"), opt)
	d, _ := dataBlock(t, doc)
	if d.Verdict != *opt.Verdict {
		t.Errorf("verdict %+v", d.Verdict)
	}
	if !strings.Contains(noscript(t, doc), `<p class="nj-v s-known">Passed with known failures</p>`) {
		t.Error("fallback headline does not use the caller's verdict and tone")
	}
}
