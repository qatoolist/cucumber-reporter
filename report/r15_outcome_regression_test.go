package report

import (
	"testing"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRound15KnownFailureCannotExplainAnotherPendingCase(t *testing.T) {
	for _, status := range []input.Status{input.StatusPending, input.StatusUndefined} {
		t.Run(string(status), func(t *testing.T) {
			in := round14Input(t, "one", "../testdata/round15/runner-signal/r1.ndjson")
			for i := range in.Attempts {
				if in.Attempts[i].Scenario.Name == "Pass first try" {
					in.Attempts[i].Scenario.Steps[0].Result.Status = status
				}
			}
			p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "r-d23e6f3ab2b8-L3", Kind: KindKnown, Ref: "K", Title: "Known first failure", Step: 1, StepText: "step ffp"}}}, RunConfig{Gates: GatePolicy{Unexpected: true}})
			if err != nil {
				t.Fatal(err)
			}
			if p.Summary.Computed.Tone != ToneFail || !p.Summary.RunnerFailed || len(p.Summary.GateFailures) == 0 {
				t.Fatalf("runner failure hidden by unrelated known annotation: %+v", p.Summary)
			}
		})
	}
}

func TestRound15KnownOnlyFailureRemainsKnown(t *testing.T) {
	in := round14Input(t, "one", "../testdata/round16/F3/cap/k25-known-z-only/run.ndjson")
	p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "a-a8fde43a2452-L9", Kind: KindKnown, Ref: "K", Title: "Known first failure", Step: 1, StepText: "step z"}}}, RunConfig{Gates: GatePolicy{Unexpected: true}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Computed.Tone != ToneWarn || p.Summary.RunnerFailed || len(p.Summary.GateFailures) != 0 {
		t.Fatalf("known-only behavior changed: %+v", p.Summary)
	}
}

func TestRound15RetryFinalEffectiveOutcomeReachesOracle(t *testing.T) {
	for _, ext := range []string{"json", "ndjson"} {
		t.Run(ext, func(t *testing.T) {
			in := round14Input(t, "one", "../testdata/captured/ruby-9.2.1-r12/retryfinal/run."+ext)
			p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{Gates: GatePolicy{Unexpected: true}})
			if err != nil {
				t.Fatal(err)
			}
			if p.Summary.Computed.Tone != ToneFail || len(p.Summary.GateFailures) == 0 {
				t.Fatalf("unrescued retries became green: %+v", p.Summary)
			}
			for _, s := range p.Summary.Scenarios {
				if s.Name == "Fails then pending" || s.Name == "Fails then skipped" {
					if s.Status != "failed" {
						t.Errorf("%s: %s", s.Name, s.Status)
					}
				}
			}
		})
	}
}

func TestRound15PendingPriorAttemptIsNotCoveredByKnownFailure(t *testing.T) {
	in := round14Input(t, "one", "../testdata/round15/known/z11_prior_pending_then_hook.json")
	p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "a-L3", Kind: KindKnown, Ref: "K", Title: "Known x", Step: 1, StepText: "x"}}}, RunConfig{Gates: GatePolicy{Unexpected: true}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Computed.Tone != ToneFail || len(p.Summary.GateFailures) == 0 {
		t.Fatalf("pending prior attempt hidden by final known failure: %+v", p.Summary)
	}
}
