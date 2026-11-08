package report

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	model "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"
	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRound13KnownDoesNotAbsorbOtherFailures(t *testing.T) {
	for _, name := range []string{"k5_two_failed", "k6_afterstep_known", "k7_beforestep2"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("../testdata/round13/known/" + name + ".json")
			if e != nil {
				t.Fatal(e)
			}
			f, e := pub.Parse(bytes.NewReader(raw), pub.Limits{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := os.ReadFile("../testdata/round13/known/" + name + ".opt")
			if e != nil {
				t.Fatal(e)
			}
			var opt Options
			if e = json.Unmarshal(b, &opt); e != nil {
				t.Fatal(e)
			}
			html, e := Render(f, opt)
			if e != nil {
				t.Fatal(e)
			}
			if bytes.Contains(html, []byte(`"label":"Passed`)) {
				t.Fatal("known issue absorbed an unrelated failure")
			}
		})
	}
}

func TestRound13APIUsesRenderedOutcome(t *testing.T) {
	for _, name := range []string{"retryfinal", "flakyonly"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("../testdata/captured/ruby-9.2.1-r12/" + name + "/run.json")
			if e != nil {
				t.Fatal(e)
			}
			f, e := pub.Parse(bytes.NewReader(raw), pub.Limits{})
			if e != nil {
				t.Fatal(e)
			}
			opt, _ := normalize(renderOptions{Version: 2})
			page, e := buildPage(internalFeatures(f), opt, nil)
			if e != nil {
				t.Fatal(e)
			}
			outcomes, e := ScenarioOutcomes(f, nil)
			if e != nil {
				t.Fatal(e)
			}
			byID := map[string]string{}
			for _, feature := range page.Features {
				for _, s := range feature.Elements {
					byID[s.ID] = s.Status
				}
			}
			ids := ScenarioIDs(f)
			for i, row := range ids {
				for j, id := range row {
					if want, ok := byID[id]; ok && outcomes[i][j] != want {
						t.Errorf("%s: API %s page %s", id, outcomes[i][j], want)
					}
				}
			}
			want := "passed"
			if name == "retryfinal" {
				want = "failed"
			}
			if got := string(pub.RunStatus(f)); got != want {
				t.Errorf("RunStatus %s want %s", got, want)
			}
		})
	}
}

func TestRound13GateVerdict(t *testing.T) {
	f := []cucumber.Feature{{URI: "a.feature", Elements: []cucumber.Scenario{{Name: "pending", Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusPending}}}}, {Name: "passed", Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusPassed}}}}}}}
	p, e := preparePublicRun([]runInput{{Name: "one", Features: f}}, renderOptions{Version: 2}, RunConfig{Gates: GatePolicy{Pending: true}})
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Summary.GateFailures) == 0 || p.Summary.Computed.Tone != ToneFail {
		t.Fatalf("gate verdict: %+v", p.Summary.Computed)
	}
}

func TestRound13CaptureAcceptance(t *testing.T) {
	dirs, e := os.ReadDir("../testdata/round13/captured-ruby-9.2.1")
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "around-fail" || d.Name() == "rerun-workflow" {
			continue
		}
		for _, format := range []string{"json", "ndjson"} {
			t.Run(d.Name()+"/"+format, func(t *testing.T) {
				raw, e := os.ReadFile("../testdata/round13/captured-ruby-9.2.1/" + d.Name() + "/run." + format)
				if os.IsNotExist(e) && d.Name() == "around-fail-msgonly" {
					return
				}
				if e != nil {
					t.Fatal(e)
				}
				in := runInput{Name: "capture"}
				if format == "json" {
					in.Features, e = cucumber.ParseEnhanced(bytes.NewReader(raw), cucumber.Limits{})
				} else {
					var m cucumber.MessageRun
					m, e = cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
					in.Attempts = m.Attempts
				}
				if e != nil {
					t.Fatal(e)
				}
				p, e := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{})
				if e != nil {
					t.Fatal(e)
				}
				if _, e = renderPublicModel(p.Features, p.Options); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestRound13OrderedRerunsRefused(t *testing.T) {
	for _, format := range []string{"json", "ndjson"} {
		var ins []runInput
		for _, name := range []string{"run1", "run2"} {
			ins = append(ins, round14Input(t, name, "../testdata/round14/captured/a1-rerun-allpass/"+name+"."+format))
		}
		if _, e := preparePublicRun(ins, renderOptions{Version: 2}, RunConfig{}); e == nil || !strings.Contains(e.Error(), "not supported in v0.3.0") {
			t.Fatalf("%s: %v", format, e)
		}
	}
}

func TestRound13RuntimeFloorRuleFaults(t *testing.T) {
	for _, prior := range []cucumber.Status{cucumber.StatusFailed, cucumber.StatusUndefined, cucumber.StatusAmbiguous} {
		t.Run(string(prior), func(t *testing.T) {
			old := cucumber.Scenario{Steps: []cucumber.Step{{Result: cucumber.Result{Status: prior}}}}
			sc := cucumber.Scenario{Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusPending}}}}
			model.ScenarioMetadata(&sc).PreviousAttempts = []cucumber.Scenario{old}
			model.CaptureOutcomeFloor(&sc, nil)
			// Reverting retry promotion produces pending, which can combine with a pass into green.
			if e := model.CheckOutcome(sc, "pending", nil); e == nil {
				t.Fatal("disabled retry rule escaped runtime guard")
			}
		})
	}
	sc := cucumber.Scenario{Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusFailed}}, {Result: cucumber.Result{Status: cucumber.StatusFailed}}}}
	model.CaptureOutcomeFloor(&sc, nil)
	if e := model.CheckOutcome(sc, "known", map[int]bool{1: true}); e == nil {
		t.Fatal("disabled known-isolation rule escaped runtime guard")
	}
	sc = cucumber.Scenario{Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusPassed}}}}
	model.CaptureOutcomeFloor(&sc, []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusFailed}}})
	if e := model.CheckOutcome(sc, "passed", nil); e == nil {
		t.Fatal("disabled Background folding escaped runtime guard")
	}
	if e := checkPolicyFloor(RunSummary{Computed: Verdict{Tone: ToneOK}, Scenarios: []ScenarioSummary{{Status: "pending"}}}, GatePolicy{Pending: true}); e == nil {
		t.Fatal("disabled gate rule escaped runtime guard")
	}
}

func TestRound13PublicTypes(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		if strings.Contains(typ.PkgPath(), "/internal/") {
			t.Errorf("internal exported type: %v", typ)
		}
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(typ.Elem())
		case reflect.Map:
			walk(typ.Key())
			walk(typ.Elem())
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				if f.IsExported() {
					walk(f.Type)
				}
			}
		}
	}
	for _, v := range []any{PreparedRun{}, Options{}, Attempt{}, Input{}, RunConfig{}} {
		walk(reflect.TypeOf(v))
	}
}

func TestRound13InlineBudget(t *testing.T) {
	raw := bytes.Repeat([]byte("evidence"), 128<<10)
	steps := make([]cucumber.Step, 40)
	for i := range steps {
		steps[i] = cucumber.Step{Result: cucumber.Result{Status: cucumber.StatusPassed}, Embeddings: []cucumber.Embedding{{MimeType: "application/octet-stream", Data: base64.StdEncoding.EncodeToString(raw)}}}
	}
	opt, _ := normalize(renderOptions{Version: 2})
	sidecars := 0
	page, e := buildPage([]cucumber.Feature{{Elements: []cucumber.Scenario{{Steps: steps}}}}, opt, func(_ string, b []byte) error {
		sidecars++
		if !bytes.Equal(b, raw) {
			t.Fatal("changed attachment")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	total := 0
	for _, st := range page.Features[0].Elements[0].Steps {
		for _, a := range st.Att {
			if a.Data != nil {
				total += len(*a.Data)
			}
		}
	}
	if total > 32<<20 || sidecars == 0 {
		t.Fatalf("inline bytes %d, sidecars %d", total, sidecars)
	}
}

func TestRound13AfterStepEvidence(t *testing.T) {
	raw, e := os.ReadFile("../testdata/round13/after-step.json")
	if e != nil {
		t.Fatal(e)
	}
	f, e := pub.Parse(bytes.NewReader(raw), pub.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	opt, _ := normalize(renderOptions{Version: 2})
	p, e := buildPage(internalFeatures(f), opt, nil)
	if e != nil {
		t.Fatal(e)
	}
	steps := p.Features[0].Elements[0].Steps
	if len(steps) != 2 || len(steps[0].Att) != 1 || len(steps[1].Att) != 0 || !strings.HasPrefix(steps[0].Att[0].Name, "After hook output") {
		t.Fatalf("misplaced hook evidence: %+v", steps)
	}
	run, e := PrepareRun([]Input{{Name: "input", Features: f}}, Options{Version: 2}, RunConfig{})
	if e != nil {
		t.Fatal(e)
	}
	page, e := buildPage(internalFeatures(run.Features), internalOptions(run.Options), nil)
	if e != nil {
		t.Fatal(e)
	}
	if got := page.Features[0].Elements[0].Steps[1].Label; got != "A hook after step 1" {
		t.Fatalf("hook label: %s", got)
	}
}

// Cucumber's own summary is the independent oracle. Ruby sometimes counts a
// failed-then-pending retry as "flaky"; a report may be stricter, never greener.
func TestRound13CapturedRunnerTruth(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../testdata/captured/ruby-9.2.1-r12/*/summary.txt", "../testdata/round13/captured-ruby-9.2.1/*/summary.txt"} {
		xs, e := filepath.Glob(pattern)
		if e != nil {
			t.Fatal(e)
		}
		files = append(files, xs...)
	}
	totalRE := regexp.MustCompile(`(?m)^(\d+) scenarios?(?: \(([^\n]+)\))?$`)
	failedRE := regexp.MustCompile(`(\d+) failed`)
	checked := 0
	for _, file := range files {
		dir := filepath.Dir(file)
		name := filepath.Base(dir)
		if name == "around-fail" || name == "rerun-workflow" {
			continue
		}
		text, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		matches := totalRE.FindAllStringSubmatch(string(text), -1)
		if len(matches) == 0 {
			t.Fatalf("missing runner summary in %s", file)
		}
		m := matches[len(matches)-1]
		total, _ := strconv.Atoi(m[1])
		failed := 0
		if f := failedRE.FindStringSubmatch(m[2]); len(f) > 0 {
			failed, _ = strconv.Atoi(f[1])
		}
		for _, format := range []string{"json", "ndjson"} {
			t.Run(dir+"/"+format, func(t *testing.T) {
				raw, e := os.ReadFile(filepath.Join(dir, "run."+format))
				if os.IsNotExist(e) && name == "around-fail-msgonly" {
					return
				}
				if e != nil {
					t.Fatal(e)
				}
				in := runInput{Name: "capture"}
				if format == "json" {
					in.Features, e = cucumber.ParseEnhanced(bytes.NewReader(raw), cucumber.Limits{})
				} else {
					var m cucumber.MessageRun
					m, e = cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
					in.Attempts = m.Attempts
				}
				if e != nil {
					t.Fatal(e)
				}
				p, e := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{})
				if e != nil {
					t.Fatal(e)
				}
				if p.Summary.ObservedScenarios != total {
					t.Fatalf("scenarios %d; cucumber %d", p.Summary.ObservedScenarios, total)
				}
				unattributed := format == "ndjson" && strings.Contains(name, "around")
				if p.Summary.Counts["failed"] < failed && !unattributed {
					t.Fatalf("failed %d; cucumber %d", p.Summary.Counts["failed"], failed)
				}
				if failed > 0 && p.Summary.Computed.Tone != ToneFail {
					t.Fatal("cucumber failure became green")
				}
				if total == 0 && p.Summary.Computed.Label != "No scenarios executed" {
					t.Fatal(p.Summary.Computed)
				}
				if format == "json" {
					public, e := pub.Parse(bytes.NewReader(raw), pub.Limits{})
					if e != nil {
						t.Fatal(e)
					}
					html, e := Render(public, Options{Version: 2})
					if e != nil {
						t.Fatal(e)
					}
					var page pageData
					block := regexp.MustCompile(`(?s)id="report-data">(.*?)</script>`).FindSubmatch(html)
					if len(block) != 2 {
						t.Fatal("missing report data")
					}
					if e = json.Unmarshal(block[1], &page); e != nil {
						t.Fatal(e)
					}
					if failed > 0 && page.Verdict.Tone != ToneFail {
						t.Fatal("public plain rendering became green")
					}
					outcomes, e := ScenarioOutcomes(public, nil)
					if e != nil {
						t.Fatal(e)
					}
					ids := ScenarioIDs(public)
					values := map[string]string{}
					var statuses []pub.Status
					for _, f := range page.Features {
						for _, sc := range f.Elements {
							values[sc.ID] = sc.Status
							statuses = append(statuses, pub.Status(sc.Status))
						}
					}
					for i, row := range ids {
						for j, id := range row {
							if target := page.Aliases[id]; target != "" {
								if outcomes[i][j] != OutcomeFolded {
									t.Fatalf("folded slot %s = %s", id, outcomes[i][j])
								}
								continue
							}
							if want, ok := values[id]; ok && outcomes[i][j] != want {
								t.Fatalf("API outcome %s: %s != %s", id, outcomes[i][j], want)
							}
						}
					}
					want := pub.Combine(statuses...)
					if len(statuses) == 0 {
						want = pub.StatusUndefined
					}
					if got := pub.RunStatus(public); got != want {
						t.Fatalf("RunStatus %s != page %s", got, want)
					}
				}
			})
		}
		checked++
	}
	if checked < 45 {
		t.Fatalf("only %d runner captures", checked)
	}
}

func TestRound13PassingRunHooksWithoutScenarios(t *testing.T) {
	sc := cucumber.Scenario{Type: "run_hook", Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusPassed}}}}
	model.StepMetadata(&sc.Steps[0]).Origin = "hook"
	if sc.Status() != cucumber.StatusPassed {
		t.Fatal("passing run hook became undefined")
	}
	f := []cucumber.Feature{{Elements: []cucumber.Scenario{sc}}}
	opt, _ := normalize(renderOptions{Version: 2})
	page, e := buildPage(f, opt, nil)
	if e != nil {
		t.Fatal(e)
	}
	if page.Verdict.Label != "No scenarios executed" {
		t.Fatal(page.Verdict)
	}
}

func TestRound13RunnerFailureOverridesInformationalOutcome(t *testing.T) {
	for _, status := range []cucumber.Status{cucumber.StatusPending, cucumber.StatusSkipped} {
		p, e := preparePublicRun([]runInput{{Name: "stream", Attempts: []cucumber.MessageAttempt{{URI: "a.feature", RunnerFailed: true, Scenario: cucumber.Scenario{ID: "case", Line: 1, Steps: []cucumber.Step{{Result: cucumber.Result{Status: status}}}}}}}}, renderOptions{Version: 2}, RunConfig{})
		if e != nil {
			t.Fatal(e)
		}
		if p.Summary.Computed.Tone != ToneFail {
			t.Fatal(p.Summary.Computed)
		}
	}
}

func TestRound13MetadataRetriesUseSameAuthority(t *testing.T) {
	for _, final := range []cucumber.Status{cucumber.StatusPassed, cucumber.StatusPending, cucumber.StatusSkipped} {
		f := []cucumber.Feature{{URI: "a.feature", Elements: []cucumber.Scenario{{Name: "case", Line: 3, Steps: []cucumber.Step{{Result: cucumber.Result{Status: final}}}}}}}
		id := runScenarioIDs(f)[0][0]
		p, e := preparePublicRun([]runInput{{Name: "one", Features: f}}, renderOptions{Version: 2}, RunConfig{Metadata: map[string]ScenarioMeta{id: {Attempts: []Attempt{{Number: 0, Status: "failed"}, {Number: 1, Status: string(final)}}}}})
		if e != nil {
			t.Fatal(e)
		}
		if raw := model.ExecutionStatus(p.Features[0].Elements[0]); raw != final {
			t.Fatalf("raw execution %s became %s", final, raw)
		}
		want := "failed"
		if final == cucumber.StatusPassed {
			want = "passed"
		}
		if p.Summary.Scenarios[0].Status != want {
			t.Fatalf("%s: metadata retry became %s", final, p.Summary.Scenarios[0].Status)
		}
		if _, e = renderPublicModel(p.Features, p.Options); e != nil {
			t.Fatal(e)
		}
	}
}

func TestRound13PreparedPublicSnapshotRetainsRedaction(t *testing.T) {
	raw := `[{"uri":"a.feature","elements":[{"id":"case","name":"case","line":3,"steps":[{"match":{"location":"steps.rb:1"},"result":{"status":"failed","error_message":"PRIOR_SECRET"}}]},{"id":"case","name":"case","line":3,"steps":[{"match":{"location":"steps.rb:1"},"result":{"status":"passed"}}]}]}]`
	f, e := pub.Parse(strings.NewReader(raw), pub.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := PrepareRun([]Input{{Name: "one", Features: f}}, Options{Version: 2}, RunConfig{Redaction: &Redaction{Patterns: []string{"PRIOR_SECRET"}}})
	if e != nil {
		t.Fatal(e)
	}
	html, e := Render(p.Features, Options{Version: 2})
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(html, []byte("PRIOR_SECRET")) {
		t.Fatal("public snapshot restored unredacted private retry evidence")
	}
}

func TestRound13PublicSnapshotRetainsRunnerFailure(t *testing.T) {
	p, e := PrepareRun([]Input{{Name: "one", Attempts: []pub.MessageAttempt{{URI: "a.feature", RunnerFailed: true, Scenario: pub.Scenario{ID: "case", Line: 3, Steps: []pub.Step{{Result: pub.Result{Status: pub.StatusPassed}}}}}}}}, Options{Version: 2}, RunConfig{})
	if e != nil {
		t.Fatal(e)
	}
	if got := pub.RunStatus(p.Features); got != pub.StatusFailed {
		t.Fatalf("public snapshot lost runner failure: %s", got)
	}
	html, e := Render(p.Features, Options{Version: 2})
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(html, []byte(`"label":"Passed`)) {
		t.Fatal("rendered snapshot lost runner failure")
	}
}
