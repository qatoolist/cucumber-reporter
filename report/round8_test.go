package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRound8FormatStepParity(t *testing.T) {
	legacy, err := os.ReadFile("../testdata/synthetic/round8/jvm-hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cucumber.ParseEnhanced(bytes.NewReader(legacy), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../testdata/synthetic/round8/hooks.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "pay-L10", Kind: KindKnown, Ref: "KNOWN", Step: 1, StepText: "pay", Outcomes: []Outcome{{Step: 1, Outcome: "known", Text: "expected difference"}}}, {Scenario: "pay-L11", Kind: KindKnown, Ref: "KNOWN", Step: 1, StepText: "pay", Outcomes: []Outcome{{Step: 1, Outcome: "known", Text: "expected difference"}}}}}
	var summaries [][]ScenarioSummary
	for _, in := range []runInput{{Name: "legacy", Features: f}, {Name: "messages", Attempts: messages.Attempts}} {
		p, err := preparePublicRun([]runInput{in}, opt, RunConfig{Gates: GatePolicy{Unexpected: true}})
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Summary.GateFailures) > 0 && !p.Summary.RunnerFailed {
			t.Fatalf("%s: %v", in.Name, p.Summary.GateFailures)
		}
		for _, sc := range p.Features[0].Elements {
			var names []string
			for _, st := range sc.Steps {
				names = append(names, st.Name)
			}
			for i, st := range sc.Steps {
				wantOrigin := "hook"
				wantOwn := 0
				if i == 1 {
					wantOrigin = "background"
				}
				if i == 3 {
					wantOrigin = "own"
					wantOwn = 1
				}
				if modelmeta.StepMetadata(&st).Origin != wantOrigin || modelmeta.StepMetadata(&st).OwnIndex != wantOwn {
					t.Fatalf("%s step %d origin/index: %+v", in.Name, i, st)
				}
			}
			if !reflect.DeepEqual(names, []string{"before", "background", "step-before", "pay", "step-after", "after"}) {
				t.Fatalf("%s order %v", in.Name, names)
			}
		}
		for _, a := range p.Options.Annotations {
			if a.Step != 1 || a.Outcomes[0].Step != 1 {
				t.Fatal("declared annotation and outcome numbers changed")
			}
		}
		summaries = append(summaries, p.Summary.Scenarios)
		optBad := opt
		optBad.Annotations = append([]Annotation(nil), opt.Annotations...)
		optBad.Annotations[0].Step = 2
		optBad.Annotations[0].StepText = "after"
		if _, err = preparePublicRun([]runInput{in}, optBad, RunConfig{}); err == nil {
			t.Fatal("annotation targeted hook instead of own step")
		}
	}
	for i := range summaries[0] {
		a, b := summaries[0][i], summaries[1][i]
		if a.Identity != b.Identity || a.Status != b.Status || a.Status != "known" {
			t.Fatalf("format difference: %+v / %+v", a, b)
		}
	}
}

func TestRound8PlainLineMove(t *testing.T) {
	f := runFixture()
	base := prepared(t, f, RunConfig{ID: "first"})
	for i := range f[0].Elements {
		f[0].Elements[i].Line += 9
	}
	next := prepared(t, f, RunConfig{ID: "next", Baseline: &base.Summary, History: base.History, Gates: GatePolicy{NewFailures: true}})
	if len(next.Summary.GateFailures) > 0 || len(next.Summary.Comparison.Added) > 0 || len(next.Summary.Comparison.Missing) > 0 {
		t.Fatalf("line move: %+v", next.Summary.Comparison)
	}
	for _, sc := range next.Summary.Scenarios {
		if sc.Samples != 2 {
			t.Fatal("history reset")
		}
	}
	f[0].Elements[1].Name = f[0].Elements[0].Name
	p := prepared(t, f, RunConfig{})
	if p.Summary.Scenarios[0].Identity == p.Summary.Scenarios[1].Identity || len(p.Summary.Diagnostics) == 0 {
		t.Fatal("colliding names need distinct identities and a diagnostic")
	}
}

func TestRound8ComputedIDsSurviveRedaction(t *testing.T) {
	f := runFixture()
	// Real synthetic Messages hooks are exercised by TestRound9ReservedHookURI.
	for _, pattern := range []string{`[0-9a-f]{32}`, `test-\w+`, `__run_hooks__`} {
		cfg := RunConfig{ID: "first", Redaction: &Redaction{Patterns: []string{pattern}}}
		first := prepared(t, f, cfg)
		cfg.ID = "second"
		cfg.Baseline = &first.Summary
		cfg.History = first.History
		cfg.Gates.NewFailures = true
		second := prepared(t, f, cfg)
		if len(second.Summary.GateFailures) > 0 {
			t.Fatal(second.Summary.GateFailures)
		}
		plain := prepared(t, f, RunConfig{})
		for i, sc := range second.Summary.Scenarios {
			if sc.Identity != plain.Summary.Scenarios[i].Identity || sc.ID != plain.Summary.Scenarios[i].ID || sc.Samples != 2 {
				t.Fatalf("computed identifiers changed: %+v", sc)
			}
		}
	}
}

func TestRound8DocStringsAlwaysText(t *testing.T) {
	for _, mt := range []string{"application/octet-stream", "image/png", "application/pdf", "image/svg+xml"} {
		f := runFixture()
		f[0].Elements[0].Steps[0].DocString = &cucumber.DocString{ContentType: mt, Value: `{"password":"SECRET"}`}
		p := prepared(t, f, RunConfig{Redaction: &Redaction{JSONKeys: []string{"password"}}})
		if strings.Contains(p.Features[0].Elements[0].Steps[0].DocString.Value, "SECRET") {
			t.Errorf("doc string skipped: %s", mt)
		}
	}
}

func TestRound8HeaderPlacement(t *testing.T) {
	r, _ := newRedactor(&Redaction{Headers: []string{"Authorization"}})
	for _, raw := range []string{"Authorization: SECRET", "curl -H 'Authorization: SECRET'", "[req] Authorization: SECRET"} {
		got := r.text(raw)
		if strings.Contains(got, "SECRET") || !strings.Contains(got, "Authorization:") {
			t.Errorf("unmasked header: %s", got)
		}
	}
	for _, raw := range []string{"X-Authorization: keep", "Authorization:\nNext: keep"} {
		if r.text(raw) != raw {
			t.Errorf("bad boundary: %q", r.text(raw))
		}
	}
}

func TestRound8MissingShardMetadata(t *testing.T) {
	f := runFixture()
	cfg := RunConfig{ExpectedShards: []string{"present", "missing"}, Metadata: map[string]ScenarioMeta{"absent-L3": {Identity: "absent"}}, Gates: GatePolicy{Incomplete: true}}
	p, err := preparePublicRun([]runInput{{Name: "present", Features: f}}, renderOptions{Version: 2}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Completeness != "incomplete" || len(p.Summary.Diagnostics) == 0 || len(p.Summary.GateFailures) != 1 {
		t.Fatalf("not incomplete: %+v", p.Summary)
	}
	cfg.ExpectedShards = []string{"present"}
	if _, err = preparePublicRun([]runInput{{Name: "present", Features: f}}, renderOptions{Version: 2}, cfg); err == nil {
		t.Fatal("typo metadata accepted for complete run")
	}
}

func TestRound8LargeRunOptionalHistory(t *testing.T) {
	p, err := prepareRunWithBudget([]runInput{{Name: "a", Features: runFixture()}}, renderOptions{Version: 2}, RunConfig{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := writeRunDir(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "history.json")); !os.IsNotExist(err) {
		t.Fatal("oversized history was published")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s RunSummary
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Diagnostics) == 0 {
		t.Fatal("omitted history has no diagnostic")
	}
}

func TestRound8XMLAndMalformedHeaderEvidence(t *testing.T) {
	r, _ := newRedactor(&Redaction{Patterns: []string{"SECRET"}, Headers: []string{"Authorization"}})
	for _, tc := range []struct{ mime, body string }{{"image/svg+xml", "<svg>SECRET</svg>"}, {"application/json", `{"Authorization":"Bearer hidden"`}, {"application/json", `{"Authorization":"Bearer hidden`}} {
		out, err := r.attachmentText([]byte(tc.body), tc.mime)
		if err != nil || bytes.Contains(out, []byte("SECRET")) || bytes.Contains(out, []byte("hidden")) {
			t.Errorf("unmasked: %s %v", out, err)
		}
	}
}

func TestRound8ImportedStructuralFields(t *testing.T) {
	base := prepared(t, runFixture(), RunConfig{ID: "base"})
	for _, mutate := range []func(*RunSummary){func(s *RunSummary) { s.RecordedAt = "SECRET" }, func(s *RunSummary) {
		s.IssueRecords = []IssueObservation{{Identity: s.Scenarios[0].Identity, State: "SECRET"}}
	}} {
		bad := base.Summary
		mutate(&bad)
		if _, err := preparePublicRun([]runInput{{Name: "a", Features: runFixture()}}, renderOptions{Version: 2}, RunConfig{History: []RunSummary{bad}}); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("invalid structural fields: %v", err)
		}
	}
}

func TestRound8HistoryPrunesLargestScope(t *testing.T) {
	makeRun := func(scope string, n, rows int) RunSummary {
		s := RunSummary{Version: SummaryVersion, ID: fmt.Sprintf("%s-%d", scope, n), Environment: scope}
		for i := 0; i < rows; i++ {
			s.Scenarios = append(s.Scenarios, ScenarioSummary{Identity: fmt.Sprintf("scenario-%d", i), Status: "passed"})
		}
		return s
	}
	var history []RunSummary
	for i := 0; i < 20; i++ {
		history = append(history, makeRun("staging", i, 1))
	}
	for i := 0; i < 4; i++ {
		history = append(history, makeRun("prod", i, 400))
	}
	expected := append([]RunSummary(nil), history[:20]...)
	expected = append(expected, history[22:]...)
	raw, _ := json.Marshal(expected)
	result, err := compactHistory(history, 30, len(raw)+20, scopeKey(history[0]))
	if err != nil || result.omitted {
		t.Fatalf("archive: %+v %v", result, err)
	}
	counts := map[string]int{}
	for _, s := range result.entries {
		counts[s.Environment]++
	}
	if counts["staging"] != 20 || counts["prod"] != 2 {
		t.Fatalf("wrong scope pruned: %v", counts)
	}
	if len(result.diagnostics) != 1 || !strings.Contains(result.diagnostics[0].Message, "prod") {
		t.Fatal(result.diagnostics)
	}
}

func TestRound8RedactedRunIDCollision(t *testing.T) {
	cfg := RunConfig{ID: "sk_live_RUN1", Redaction: &Redaction{Patterns: []string{`sk_live_\w+`}}}
	first := prepared(t, runFixture(), cfg)
	cfg.ID = "sk_live_RUN2"
	cfg.History = first.History
	if _, err := preparePublicRun([]runInput{{Name: "a", Features: runFixture()}}, renderOptions{Version: 2}, cfg); err == nil {
		t.Fatal("redacted run IDs collapsed history")
	}
}

func TestRound8ConfigurableDurationFloor(t *testing.T) {
	f := runFixture()
	f[0].Elements = f[0].Elements[:1]
	d := time.Millisecond
	f[0].Elements[0].Steps[0].Result.Duration = &d
	base := prepared(t, f, RunConfig{ID: "first"})
	d = 99 * time.Millisecond
	minMs := 0.0
	cfg := RunConfig{ID: "next", Baseline: &base.Summary, Gates: GatePolicy{MaxRegressionPercent: 50, MinRegressionMs: &minMs}}
	next := prepared(t, f, cfg)
	if len(next.Summary.Comparison.Slower) != 1 || len(next.Summary.GateFailures) != 1 {
		t.Fatal("configured low floor ignored")
	}
	minMs = -1
	if _, err := preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, cfg); err == nil {
		t.Fatal("negative floor accepted")
	}
}

func TestRound8GeneratedRunIDsAreStructural(t *testing.T) {
	cfg := RunConfig{Redaction: &Redaction{Patterns: []string{`run-\w+`}}}
	for i := 0; i < 3; i++ {
		p := prepared(t, runFixture(), cfg)
		if !strings.HasPrefix(p.Summary.ID, "run-") || len(p.History) != i+1 {
			t.Fatal("generated run identifier was redacted or history collapsed")
		}
		cfg.History = p.History
	}
}
