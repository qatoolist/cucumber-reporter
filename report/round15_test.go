package report

import (
	"bytes"
	"strings"
	"testing"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"
)

func TestRound15StrictRunnerSignal(t *testing.T) {
	in := round14Input(t, "strict", "../testdata/round15/captured/g3-same-strict/run.ndjson")
	p, e := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{Gates: GatePolicy{Unexpected: true}})
	if e != nil {
		t.Fatal(e)
	}
	if p.Summary.Computed.Tone != ToneFail || len(p.Summary.GateFailures) == 0 {
		t.Fatalf("strict runner failure lost: %v, gates %v", p.Summary.Computed, p.Summary.GateFailures)
	}
}

func TestRound15RescueCannotExplainUnrelatedRunnerFailure(t *testing.T) {
	var ins []runInput
	for _, n := range []string{"r1", "r2"} {
		ins = append(ins, round14Input(t, n, "../testdata/round15/runner-signal/"+n+".ndjson"))
	}
	_, e := preparePublicRun(ins, renderOptions{Version: 2}, RunConfig{})
	if e == nil || !strings.Contains(e.Error(), "not supported in v0.3.0") {
		t.Fatalf("cross-input repeat was accepted: %v", e)
	}
}

func TestRound15RenderUsesSuppliedFeatures(t *testing.T) {
	f := []pub.Feature{{URI: "a.feature", Name: "Original", Elements: []pub.Scenario{{Name: "original", Line: 1, Steps: []pub.Step{{Name: "ok", Result: pub.Result{Status: pub.StatusPassed}}}}}}}
	p, e := PrepareRun([]Input{{Name: "one", Features: f}}, Options{Version: 2}, RunConfig{})
	if e != nil {
		t.Fatal(e)
	}
	other := []pub.Feature{{URI: "b.feature", Name: "OTHER", Elements: []pub.Scenario{{Name: "OTHER", Line: 1, Steps: []pub.Step{{Result: pub.Result{Status: pub.StatusFailed, Error: "OTHER FAILURE"}}}}}}}
	b, e := Render(other, p.Options)
	if e == nil && !bytes.Contains(b, []byte("OTHER FAILURE")) {
		t.Fatal("Render ignored its features argument")
	}
	if e = Validate(other, p.Options); e == nil {
		t.Fatal("Validate accepted analysis for another run")
	}
	b, e = Render(nil, p.Options)
	if e == nil && bytes.Contains(b, []byte(`"name":"original"`)) {
		t.Fatal("Render(nil) rendered cached features")
	}
}
