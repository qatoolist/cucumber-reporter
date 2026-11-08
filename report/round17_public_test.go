package report_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

const round17JSON = `[{"uri":"features/a.feature","id":"a","keyword":"Feature","name":"A","line":1,"elements":[
{"id":"a;s1","keyword":"Scenario","name":"one","line":3,"type":"scenario","steps":[{"keyword":"Given ","name":"x","line":4,"result":{"status":"passed","duration":1000}}]},
{"id":"a;s2","keyword":"Scenario","name":"two","line":6,"type":"scenario","steps":[{"keyword":"Given ","name":"y","line":7,"result":{"status":"failed","duration":1000,"error_message":"boom"}}]}]}]`

const round17Rerun = `[{"uri":"features/a.feature","id":"a","keyword":"Feature","name":"A","line":1,"elements":[
{"id":"a;s2","keyword":"Scenario","name":"two","line":6,"type":"scenario","steps":[{"keyword":"Given ","name":"y","line":7,"result":{"status":"passed","duration":1000}}]}]}]`

func round17Parse(t *testing.T, s string) []cucumber.Feature {
	t.Helper()
	f, err := cucumber.Parse(strings.NewReader(s), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

var round17Options = report.Options{Version: report.OptionsVersion}

// G4-N5: ErrEvidence is attached where floor errors are created, never by
// matching message text, so a URI cannot make an input error look like one.
func TestRound17EvidenceSentinelIsNotTextMatched(t *testing.T) {
	a := strings.Replace(round17JSON, `"features/a.feature"`, `"evidence floor: a.feature"`, 1)
	b := strings.Replace(round17Rerun, `"features/a.feature"`, `"evidence floor: a.feature"`, 1)
	_, err := report.PrepareRun([]report.Input{{Name: "r1", Features: round17Parse(t, a)}, {Name: "r2", Features: round17Parse(t, b)}}, round17Options, report.RunConfig{})
	if !errors.Is(err, report.ErrInvalidInput) || errors.Is(err, report.ErrEvidence) {
		t.Fatalf("collision with floor text in URI: Is(ErrInvalidInput)=%v Is(ErrEvidence)=%v %v", errors.Is(err, report.ErrInvalidInput), errors.Is(err, report.ErrEvidence), err)
	}
	// A genuine floor refusal still carries the sentinel.
	f := round17Parse(t, round17JSON)
	f[0].Elements[1].Steps[0].Result.Status = cucumber.StatusPassed
	_, err = report.PrepareRun([]report.Input{{Name: "x", Features: f}}, round17Options, report.RunConfig{})
	if !errors.Is(err, report.ErrEvidence) || errors.Is(err, report.ErrInvalidInput) {
		t.Fatalf("floor refusal lost its sentinel: %v", err)
	}
}

// G4-N6: caller edits and metadata conflicts are described as such, with the URI.
func TestRound17CallerEditWording(t *testing.T) {
	f := round17Parse(t, round17JSON)
	f[0].Elements[1].Steps[0].Result.Status = cucumber.StatusPassed
	_, err := report.PrepareRun([]report.Input{{Name: "x", Features: f}}, round17Options, report.RunConfig{})
	if err == nil || !strings.Contains(err.Error(), "features/a.feature:6") || !strings.Contains(err.Error(), "edited after parsing") {
		t.Fatalf("parse-then-edit refusal: %v", err)
	}

	pr, err := report.PrepareRun([]report.Input{{Name: "x", Features: round17Parse(t, round17JSON)}}, round17Options, report.RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	pr.Summary.Computed.Label = "Passed"
	_, err = report.Render(pr.Features, pr.Options)
	if err == nil || !strings.Contains(err.Error(), "edited after PrepareRun") {
		t.Fatalf("prepared-run edit: %v", err)
	}

	renamed := strings.NewReplacer(`"name":"A"`, `"name":"A renamed"`, `"line":6`, `"line":60`, `"line":7`, `"line":61`).Replace(round17Rerun)
	_, err = report.PrepareRun([]report.Input{{Name: "a", Features: round17Parse(t, round17JSON)}, {Name: "b", Features: round17Parse(t, renamed)}}, round17Options, report.RunConfig{})
	if !errors.Is(err, report.ErrInvalidInput) || strings.Contains(err.Error(), "Messages") || !strings.Contains(err.Error(), "features/a.feature") {
		t.Fatalf("JSON-only metadata conflict: %v", err)
	}
}
