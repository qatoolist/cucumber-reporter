package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRound10PlainHookFailures(t *testing.T) {
	for _, where := range []string{"before", "after", "step-before", "step-after", "js"} {
		t.Run(where, func(t *testing.T) {
			hook := `[{"result":{"status":"failed","error_message":"HOOK BOOM"},"embeddings":[{"mime_type":"text/plain","data":"SE9PSyBFVklERU5DRQ=="}]}]`
			step := `{"name":"pay","keyword":"Given ","result":{"status":"passed"}}`
			extra := ""
			if strings.HasPrefix(where, "step-") {
				step = `{"name":"pay","result":{"status":"passed"},"` + strings.TrimPrefix(where, "step-") + `":` + hook + `}`
			} else if where == "js" {
				step = `{"hidden":true,"keyword":"Before","result":{"status":"failed","error_message":"HOOK BOOM"}},` + step
			} else {
				extra = `,"` + where + `":` + hook
			}
			raw := `[{"uri":"case.feature","elements":[{"name":"case","line":3,"steps":[` + step + `]` + extra + `}]}]`
			f, err := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			opt, _ := normalize(renderOptions{Version: 2})
			page, err := buildPage(f, opt, nil)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(page)
			if f[0].Status() != cucumber.StatusFailed || !bytes.Contains(data, []byte("HOOK BOOM")) {
				t.Fatalf("plain hook lost: %s", data)
			}
			if where != "js" && !bytes.Contains(data, []byte("SE9PSyBFVklERU5DRQ==")) {
				t.Fatal("hook attachment lost")
			}
		})
	}
}

func TestRound10LinearIDs(t *testing.T) {
	f := []cucumber.Feature{{URI: "case.feature", Elements: make([]cucumber.Scenario, 50000)}}
	start := time.Now()
	ids := scenarioIDs(f)
	elapsed := time.Since(start)
	t.Logf("50,000 colliding IDs: %s", elapsed)
	if elapsed > time.Second {
		t.Fatalf("quadratic IDs: %s", elapsed)
	}
	if ids[0][0] != "case-L0" || ids[0][49999] != "case-L0-50000" {
		t.Fatal("IDs changed")
	}
}

func TestRound10GoOnlyOptions(t *testing.T) {
	var opt renderOptions
	if err := json.Unmarshal([]byte(`{"version":2,"analysis":{"computed":{"text":"FORGED"}},"bundleAttachments":true}`), &opt); err != nil {
		t.Fatal(err)
	}
	if opt.Analysis != nil || opt.BundleAttachments {
		t.Fatal("trusted Go-only options decoded from JSON")
	}
}

func TestRound10PassingHooksAndBackground(t *testing.T) {
	base := `[{"uri":"case.feature","elements":[{"name":"case","line":3,"steps":[{"name":"pay","result":{"status":"passed"}}]}]}]`
	withHook := strings.Replace(base, `"steps":`, `"before":[{"result":{"status":"passed"}}],"after":[{"result":{"status":"passed"}}],"steps":`, 1)
	render := func(raw string) []byte {
		f, e := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := renderPublicModel(f, renderOptions{Version: 2})
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	if !bytes.Equal(render(base), render(withHook)) {
		t.Fatal("passing hooks changed plain output")
	}
	bg := strings.Replace(base, `"elements":[`, `"elements":[{"type":"background","steps":[{"name":"setup","result":{"status":"failed","error_message":"BACKGROUND BOOM"}}]},`, 1)
	f, e := cucumber.Parse(strings.NewReader(bg), cucumber.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(f[0].Elements) != 1 || f[0].Status() != cucumber.StatusFailed || !bytes.Contains(render(bg), []byte("BACKGROUND BOOM")) {
		t.Fatal("Background execution lost or counted separately")
	}
}

func TestRound10PrivateModelFields(t *testing.T) {
	for _, tc := range []struct {
		model  any
		fields []string
	}{
		{cucumber.Feature{}, []string{"ReportID", "SyntheticRunHooks"}},
		{cucumber.Scenario{}, []string{"IdentityNameCount", "BackgroundUnresolved"}},
		{cucumber.Step{}, []string{"Origin", "OwnIndex"}},
	} {
		for _, name := range tc.fields {
			if _, ok := reflect.TypeOf(tc.model).FieldByName(name); ok {
				t.Fatalf("internal field exported: %s", name)
			}
		}
	}
	var forged cucumber.Feature
	if err := json.Unmarshal([]byte(`{"ReportID":"forged","SyntheticRunHooks":true,"Elements":[{"IdentityNameCount":99,"Steps":[{"Origin":"hook","OwnIndex":99}]}]}`), &forged); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(forged)
	for _, field := range []string{"ReportID", "SyntheticRunHooks", "IdentityNameCount", "Origin", "OwnIndex"} {
		if bytes.Contains(raw, []byte(field)) {
			t.Fatalf("private bookkeeping exposed through JSON: %s", raw)
		}
	}
	original := &RunSummary{Version: SummaryVersion, ID: "trusted", Name: "SECRET"}
	p, err := preparePublicRun([]runInput{{Name: "input", Features: runFixture()}}, renderOptions{Version: 2, BundleAttachments: true, Analysis: original}, RunConfig{Redaction: &Redaction{Patterns: []string{"SECRET"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Options.BundleAttachments || p.Options.Analysis == nil || p.Options.Analysis == original || original.Name != "SECRET" {
		t.Fatal("Go-only options lost or caller analysis mutated in redaction clone")
	}
}

func TestRound10RedactionCollisions(t *testing.T) {
	for _, format := range []string{"json", "ndjson"} {
		in := r9Input(t, "distinct", format, "Scenario")
		p := r9Prepare(t, in, renderOptions{Version: 2}, RunConfig{Redaction: &Redaction{Patterns: []string{"alice|bob"}}})
		if p.Summary.Scenarios[0].Identity == p.Summary.Scenarios[1].Identity || len(p.Summary.Diagnostics) == 0 {
			t.Fatal("redacted collisions did not use line fallback")
		}
		if format == "ndjson" {
			in = r9Input(t, "distinct-subset", format, "")
			subset := r9Prepare(t, in, renderOptions{Version: 2}, RunConfig{Redaction: &Redaction{Patterns: []string{"alice|bob"}}})
			if subset.Summary.Scenarios[0].Identity != p.Summary.Scenarios[1].Identity {
				t.Fatal("redacted catalog lost subset identity")
			}
		}
	}
}

func TestRound10HistoryKeepsCurrentScopeFirst(t *testing.T) {
	makeRun := func(id, env string, n int) RunSummary {
		s := RunSummary{Version: SummaryVersion, ID: id, Environment: env}
		for i := 0; i < n; i++ {
			s.Scenarios = append(s.Scenarios, ScenarioSummary{Identity: fmt.Sprint(i), Status: "passed"})
		}
		return s
	}
	current := []RunSummary{makeRun("a", "current", 600), makeRun("b", "current", 600)}
	raw, _ := json.Marshal(current)
	history := append([]RunSummary{makeRun("other", "other", 800)}, current...)
	archive, err := compactHistory(history, 30, len(raw)+100, scopeKey(current[0]))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.entries) != 2 || archive.entries[0].ID != "a" || archive.entries[1].ID != "b" {
		t.Fatal("current scope pruned before other scope", archive.entries)
	}
}

func TestRound10BackgroundHookAndCopyIsolation(t *testing.T) {
	raw := `[{"uri":"case.feature","elements":[{"type":"background","before":[{"result":{"status":"failed","error_message":"BG HOOK BOOM"}}],"steps":[{"result":{"status":"passed"}}]},{"name":"case","line":3,"steps":[{"result":{"status":"passed"}}]}]}]`
	f, err := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := renderPublicModel(f, renderOptions{Version: 2})
	if err != nil || !bytes.Contains(doc, []byte("BG HOOK BOOM")) || f[0].Status() != cucumber.StatusFailed {
		t.Fatal("Background hook lost", err)
	}
	input := runFixture()
	input[0].Elements[0].Steps[0].DocString = &cucumber.DocString{Value: "SECRET"}
	input[0].Elements[0].Steps[0].Rows = []cucumber.Row{{Cells: []string{"SECRET"}}}
	p := prepared(t, input, RunConfig{Redaction: &Redaction{Patterns: []string{"SECRET"}}})
	if input[0].Elements[0].Steps[0].DocString.Value != "SECRET" || input[0].Elements[0].Steps[0].Rows[0].Cells[0] != "SECRET" {
		t.Fatal("redaction mutated caller data")
	}
	if p.Features[0].Elements[0].Steps[0].DocString.Value == "SECRET" {
		t.Fatal("redaction skipped copied evidence")
	}
}

func TestRound10OutputDirectoryError(t *testing.T) {
	_, err := renderFeaturesDirTo(runFixture(), filepath.Join(t.TempDir(), "missing"), renderOptions{Version: 2})
	if !errors.Is(err, ErrNoOutputDir) || errors.Is(err, ErrNoInput) {
		t.Fatalf("wrong output sentinel: %v", err)
	}
}
