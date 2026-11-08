package report

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func r9Input(t *testing.T, name, format, keyword string) runInput {
	t.Helper()
	raw, err := os.ReadFile("../testdata/synthetic/round9/" + name + "." + format)
	if err != nil {
		t.Fatal(err)
	}
	in := runInput{Name: "input"}
	if format == "json" {
		raw = bytes.ReplaceAll(raw, []byte("Scenario Outline"), []byte(keyword))
		in.Features, err = cucumber.ParseEnhanced(bytes.NewReader(raw), cucumber.Limits{})
	} else {
		var m cucumber.MessageRun
		m, err = cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
		in.Attempts = m.Attempts
	}
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func r9Prepare(t *testing.T, in runInput, opt renderOptions, cfg RunConfig) *preparedRun {
	t.Helper()
	p, err := preparePublicRun([]runInput{in}, opt, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRound9IdentityMatrix(t *testing.T) {
	for _, names := range []string{"distinct", "collision"} {
		for _, keyword := range []string{"Scenario Outline", "Scenario", "Example", "Szenario"} {
			t.Run(names+"/"+keyword, func(t *testing.T) {
				legacy := r9Input(t, names, "json", keyword)
				messages := r9Input(t, names, "ndjson", "")
				a := r9Prepare(t, legacy, renderOptions{Version: 2}, RunConfig{})
				b := r9Prepare(t, messages, renderOptions{Version: 2}, RunConfig{})
				for i, s := range a.Summary.Scenarios {
					if s.Identity != b.Summary.Scenarios[i].Identity {
						t.Fatalf("format identity mismatch %s / %s", s.Identity, b.Summary.Scenarios[i].Identity)
					}
				}
				for _, p := range []*preparedRun{a, b} {
					collision := false
					for _, d := range p.Summary.Diagnostics {
						collision = collision || strings.Contains(d.Message, "identity follows the line")
					}
					if collision != (names == "collision") {
						t.Fatalf("wrong collision warning: %+v", p.Summary.Diagnostics)
					}
				}
				subset := r9Prepare(t, r9Input(t, names+"-subset", "ndjson", ""), renderOptions{Version: 2}, RunConfig{})
				if subset.Summary.Scenarios[0].Identity != b.Summary.Scenarios[1].Identity {
					t.Fatal("Messages subset changed identity")
				}
				for i := range legacy.Features[0].Elements {
					legacy.Features[0].Elements[i].Line += 20
				}
				shifted := r9Prepare(t, legacy, renderOptions{Version: 2}, RunConfig{})
				for i, s := range shifted.Summary.Scenarios {
					if (s.Identity == a.Summary.Scenarios[i].Identity) != (names == "distinct") {
						t.Fatal("wrong moved-row identity")
					}
				}
			})
		}
	}
}

func TestRound9ReservedHookURI(t *testing.T) {
	for _, uri := range []string{"__run_hooks__/sk_live_SECRET.feature", "./__run_hooks__/sk_live_SECRET.feature", "dir/../__run_hooks__/sk_live_SECRET.feature"} {
		f := runFixture()
		f[0].URI = uri
		if _, err := preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, RunConfig{Redaction: &Redaction{Patterns: []string{`sk_live_\w+`}}}); err == nil {
			t.Errorf("reserved user URI accepted: %s", uri)
		}
	}
	raw := `{"hook":{"id":"h","name":"setup"}}
{"testRunHookStarted":{"id":"s","hookId":"h","timestamp":{"seconds":"1","nanos":0}}}
{"testRunHookFinished":{"testRunHookStartedId":"s","timestamp":{"seconds":"2","nanos":0},"result":{"status":"PASSED","duration":{"seconds":"0","nanos":0}}}}
{"testRunFinished":{"success":true}}`
	m, err := cucumber.ParseMessages(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	in := runInput{Name: "hooks", Attempts: m.Attempts}
	base := r9Prepare(t, in, renderOptions{Version: 2}, RunConfig{})
	for _, pat := range []string{`_hook`, `__run_hooks__`, `test-\w+`, `[0-9a-f]{32}`} {
		p := r9Prepare(t, in, renderOptions{Version: 2}, RunConfig{Redaction: &Redaction{Patterns: []string{pat}}})
		if p.Summary.Scenarios[0].ID != base.Summary.Scenarios[0].ID || p.Summary.Scenarios[0].Identity != base.Summary.Scenarios[0].Identity {
			t.Fatal("real hook identity changed")
		}
	}
}

func r9NoLeak(t *testing.T, p *preparedRun, secret string) {
	t.Helper()
	dir, err := writeRunDir(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "report.zip" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err == nil && bytes.Contains(raw, []byte(secret)) {
			t.Errorf("leak in %s", entry.Name())
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(filepath.Join(dir, "report.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		raw, e := io.ReadAll(r)
		r.Close()
		if e != nil || bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("ZIP leak in %s: %v", f.Name, e)
		}
	}
}

func TestRound9MixedPolicyIdentities(t *testing.T) {
	for _, chain := range [][]bool{{false, true}, {true, false, true}} {
		f := runFixture()
		f[0].Elements = f[0].Elements[1:]
		id := runScenarioIDs(f)[0][0]
		var prev *preparedRun
		for i, on := range chain {
			cfg := RunConfig{ID: fmt.Sprintf("run%d", i), Bundle: true, Metadata: map[string]ScenarioMeta{id: {Identity: "acct-sk_live_IDENT"}}}
			if prev != nil {
				cfg.Baseline = &prev.Summary
				cfg.History = prev.History
				cfg.Gates.NewFailures = true
			}
			if on {
				cfg.Redaction = &Redaction{Patterns: []string{`sk_live_\w+`}}
			}
			p := r9Prepare(t, runInput{Name: "a", Features: f}, renderOptions{Version: 2}, cfg)
			if on {
				if len(p.Summary.GateFailures) > 0 {
					t.Fatal("policy created a new failure")
				}
				if i > 0 && p.Summary.Scenarios[0].Samples != i+1 {
					t.Fatal("samples lost")
				}
				r9NoLeak(t, p, "sk_live_IDENT")
			}
			prev = p
		}
	}
}

func TestRound9JSAnnotationsAndDeclaredNumbers(t *testing.T) {
	opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "js-L9", Kind: KindKnown, Ref: "KNOWN", Step: 2, StepText: "pay"}}}
	for _, format := range []string{"json", "ndjson"} {
		t.Run(format, func(t *testing.T) {
			in := r9Input(t, "js", format, "")
			p := r9Prepare(t, in, opt, RunConfig{Gates: GatePolicy{Unexpected: true}})
			if p.Summary.Scenarios[0].Status != "known" || (len(p.Summary.GateFailures) > 0 && !p.Summary.RunnerFailed) {
				t.Fatal("inline hooks shifted annotation")
			}
			page, err := buildPage(p.Features, p.Options, nil)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(page.Annotations[0])
			var a map[string]any
			_ = json.Unmarshal(raw, &a)
			if a["step"] != float64(2) || a["executionStep"] != float64(7) {
				t.Fatalf("display/anchor mixed: %s", raw)
			}
			p.Features[0].Elements[0].Steps[0].Result.Status = cucumber.StatusFailed
			p.Features[0].Elements[0].Steps[6].Result.Status = cucumber.StatusPassed
			page, err = buildPage(p.Features, p.Options, nil)
			if err != nil {
				t.Fatal(err)
			}
			reason := page.Annotations[0].Reason
			if !strings.Contains(reason, "hook before step 1") || !strings.Contains(reason, "declared step 2") {
				t.Fatalf("misleading reason: %s", reason)
			}
		})
	}
}

func TestRound9HistoryBudgetAnalysisAndScopePriority(t *testing.T) {
	f := runFixture()
	first := prepared(t, f, RunConfig{ID: "first", Environment: "staging"})
	second := prepared(t, f, RunConfig{ID: "second", Environment: "staging", History: first.History})
	cfg := RunConfig{ID: "third", Environment: "staging", History: second.History}
	full := prepared(t, f, cfg)
	archive, _ := json.Marshal([]RunSummary{archiveSummary(full.Summary)})
	for _, budget := range []int{1, len(archive) + 10} {
		p, err := prepareRunWithBudget([]runInput{{Name: "shard-a", Features: f}}, renderOptions{Version: 2}, cfg, budget)
		if err != nil {
			t.Fatal(err)
		}
		if p.Summary.Scenarios[0].Samples != 3 {
			t.Fatalf("budget changed analysis: %d", p.Summary.Scenarios[0].Samples)
		}
	}
	huge := first.Summary
	huge.Environment = "prod"
	huge.ID = "huge"
	huge.Scenarios = append([]ScenarioSummary(nil), huge.Scenarios...)
	huge.Scenarios[0].Identity = strings.Repeat("x", 10000)
	huge.Scenarios[0].GeneratedIdentity = false
	cfg.History = []RunSummary{huge}
	p, err := prepareRunWithBudget([]runInput{{Name: "shard-a", Features: f}}, renderOptions{Version: 2}, cfg, len(archive)+100)
	if err != nil {
		t.Fatal(err)
	}
	if p.OmitHistory || len(p.History) != 1 || p.History[0].Environment != "staging" {
		t.Fatalf("other scope blocked current archive: %+v", p.History)
	}
	if !strings.Contains(fmt.Sprint(p.Summary.Diagnostics), "prod") {
		t.Fatal("dropped scope unnamed")
	}
}

func TestRound9CompletenessConfiguration(t *testing.T) {
	f := runFixture()
	cfg := RunConfig{ExpectedScenarios: 3, Metadata: map[string]ScenarioMeta{"missing-L7": {Identity: "missing"}}, Gates: GatePolicy{Incomplete: true}}
	p, err := preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, cfg)
	if err != nil || p.Summary.Completeness != "incomplete" {
		t.Fatalf("count expectation: %v", err)
	}
	cfg.ExpectedShards = []string{"typo"}
	if _, err = preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, cfg); err == nil {
		t.Fatal("unmatched shard expectation hid metadata typo")
	}
}

func TestRound9ImportedCountsAndHeaders(t *testing.T) {
	old := prepared(t, runFixture(), RunConfig{ID: "old"})
	old.Summary.Counts["sk_live_SECRET"] = 1
	if _, err := preparePublicRun([]runInput{{Name: "a", Features: runFixture()}}, renderOptions{Version: 2}, RunConfig{History: []RunSummary{old.Summary}}); err == nil || strings.Contains(err.Error(), "sk_live_SECRET") {
		t.Fatalf("invalid count keys: %v", err)
	}
	r, _ := newRedactor(&Redaction{Headers: []string{"Authorization"}})
	for _, raw := range []string{"Proxy-Authorization: keep", "X-Authorization: keep", "Authorization-Extra: keep", "Authorization:\r\nNext: keep"} {
		if got := r.text(raw); got != raw {
			t.Fatalf("overmatched %q: %q", raw, got)
		}
	}
	for _, raw := range []string{"\tAuthorization:\tSECRET\r\nNext: keep", "{'Authorization': 'SECRET'}", "\"Authorization\": \"SECRET\nNext: keep"} {
		out := r.text(raw)
		if strings.Contains(out, "SECRET") || (strings.Contains(raw, "Next:") && !strings.Contains(out, "Next: keep")) {
			t.Fatalf("bad header masking: %q", out)
		}
	}
}

func TestRound9MessagesAnnotationsAndImportLeaks(t *testing.T) {
	in := r9Input(t, "js", "ndjson", "")
	secret := "sk_live_MESSAGES"
	for i := range in.Attempts {
		a := &in.Attempts[i]
		a.Worker = secret
		for j := range a.Scenario.Steps {
			s := &a.Scenario.Steps[j]
			s.Result.Error = secret
			s.DocString = &cucumber.DocString{ContentType: "json", Value: `{"password":"` + secret + `"}`}
			s.Embeddings = []cucumber.Embedding{{Name: secret, MimeType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte(secret))}}
		}
	}
	opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "js-L9", Kind: KindKnown, Ref: secret, Step: 2, StepText: "pay", Title: secret, Note: secret, Details: []Fact{{Label: secret, Value: secret}}, Outcomes: []Outcome{{Step: 2, Outcome: "known", Text: secret}}}}}
	cfg := RunConfig{ID: "first", Bundle: true, Metadata: map[string]ScenarioMeta{"js-L9": {Identity: secret}}}
	first := r9Prepare(t, in, opt, cfg)
	cfg.ID = "second"
	cfg.History = first.History
	cfg.Baseline = &first.Summary
	cfg.Redaction = &Redaction{Patterns: []string{`sk_live_\w+`}, JSONKeys: []string{"password"}}
	p := r9Prepare(t, in, opt, cfg)
	r9NoLeak(t, p, secret)
	for _, f := range p.Features {
		for _, sc := range f.Elements {
			for _, s := range sc.Steps {
				for _, a := range s.Embeddings {
					raw, err := a.Decode(10000)
					if err != nil || bytes.Contains(raw, []byte(secret)) {
						t.Fatalf("decoded Messages evidence leaked: %v", err)
					}
				}
			}
		}
	}
}

func TestRound9MissingBackgroundReferences(t *testing.T) {
	raw, err := os.ReadFile("../testdata/synthetic/round9/js.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte(`"children": [`), []byte(`"children": [{"background":{"id":"bg","name":"setup"}},`))
	raw = bytes.ReplaceAll(raw, []byte(`"astNodeIds": ["first-ast"]`), []byte(`"astNodeIds": []`))
	m, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p := r9Prepare(t, runInput{Name: "messages", Attempts: m.Attempts}, renderOptions{Version: 2}, RunConfig{})
	for _, d := range p.Summary.Diagnostics {
		if strings.Contains(d.Message, "Background") {
			return
		}
	}
	t.Fatal("missing Background mapping was silent")
}

func TestRound9KeywordHooks(t *testing.T) {
	raw := `[{"uri":"hooks.feature","elements":[{"name":"case","line":3,"steps":[{"keyword":"Before","result":{"status":"passed"}},{"keyword":"Given","name":"pay","result":{"status":"failed"}},{"keyword":"After","result":{"status":"passed"}}]}]}]`
	f, err := cucumber.ParseEnhanced(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p := r9Prepare(t, runInput{Name: "legacy", Features: f}, renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "hooks-L3", Kind: KindKnown, Ref: "KNOWN", Step: 1, StepText: "pay"}}}, RunConfig{Gates: GatePolicy{Unexpected: true}})
	if len(p.Summary.GateFailures) != 0 || p.Summary.Scenarios[0].Status != "known" {
		t.Fatal("keyword-only hooks shifted annotation")
	}
}

func TestRound9RuleNameCollisions(t *testing.T) {
	legacy := r9Input(t, "collision", "json", "Scenario")
	for i := range legacy.Features[0].Elements {
		if legacy.Features[0].Elements[i].Type != "background" {
			legacy.Features[0].Elements[i].Type = "scenario"
		}
	}
	a := r9Prepare(t, legacy, renderOptions{Version: 2}, RunConfig{})
	b := r9Prepare(t, r9Input(t, "rules", "ndjson", ""), renderOptions{Version: 2}, RunConfig{})
	for i, s := range a.Summary.Scenarios {
		if s.Identity != b.Summary.Scenarios[i].Identity {
			t.Fatal("Rule collision differs across formats")
		}
	}
	subset := r9Prepare(t, r9Input(t, "rules-subset", "ndjson", ""), renderOptions{Version: 2}, RunConfig{})
	if subset.Summary.Scenarios[0].Identity != b.Summary.Scenarios[1].Identity {
		t.Fatal("Rule collision subset changed identity")
	}
	legacy.Features = a.Features
	ids := runScenarioIDs(legacy.Features)
	cfg := RunConfig{Metadata: map[string]ScenarioMeta{ids[0][0]: {Identity: "first"}, ids[0][1]: {Identity: "second"}}}
	pinned := r9Prepare(t, legacy, renderOptions{Version: 2}, cfg)
	legacy.Features[0].Elements = legacy.Features[0].Elements[1:]
	single := r9Prepare(t, legacy, renderOptions{Version: 2}, RunConfig{Metadata: map[string]ScenarioMeta{ids[0][1]: {Identity: "second"}}})
	if single.Summary.Scenarios[0].Identity != pinned.Summary.Scenarios[1].Identity {
		t.Fatal("explicit identity failed to pin legacy subset")
	}
}

func TestRound9OutlineShardPermutationAndSubset(t *testing.T) {
	for _, names := range []string{"distinct", "collision"} {
		in := r9Input(t, names, "ndjson", "")
		a := runInput{Name: "a", Attempts: in.Attempts[:1]}
		b := runInput{Name: "b", Attempts: in.Attempts[1:]}
		base, err := preparePublicRun([]runInput{a, b}, renderOptions{Version: 2}, RunConfig{})
		if err != nil {
			t.Fatal(err)
		}
		identities := map[int]string{}
		for _, s := range base.Summary.Scenarios {
			identities[s.Line] = s.Identity
		}
		for _, inputs := range [][]runInput{{b, a}, {a}, {b}} {
			p, err := preparePublicRun(inputs, renderOptions{Version: 2}, RunConfig{})
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range p.Summary.Scenarios {
				if identities[s.Line] != s.Identity {
					t.Fatalf("%s identity changed with shard selection", names)
				}
			}
		}
		if names == "distinct" {
			for i := range in.Attempts {
				in.Attempts[i].Scenario.Line += 20
			}
			moved := r9Prepare(t, in, renderOptions{Version: 2}, RunConfig{})
			for _, s := range moved.Summary.Scenarios {
				if identities[s.Line-20] != s.Identity {
					t.Fatal("Messages unique row moved identity")
				}
			}
		}
	}
}
