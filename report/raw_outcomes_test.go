package report

import (
	"testing"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	model "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
	"github.com/qatoolist/cucumber-reporter/internal/evidence"
)

// Missing final/attempt entries are defensive library invariants: the parser
// cannot produce them, but removing these checks must fail an ordinary Go test.
func TestRound16RunnerAccountingGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stream evidence.Stream
		final  map[string]evidence.Final
		want   bool
	}{
		{"no-signal", evidence.Stream{}, nil, false},
		{"unexplained", evidence.Stream{Failed: true, Unexplained: []string{"hook"}}, nil, true},
		{"missing-final", evidence.Stream{Failed: true, Cases: map[string][]evidence.Execution{"a": {{Results: []string{"failed"}}}, "b": {{Results: []string{"passed"}}}}}, map[string]evidence.Final{"a": {Status: "known"}}, true},
		{"missing-attempt", evidence.Stream{Failed: true, Cases: map[string][]evidence.Execution{"a": nil}}, map[string]evidence.Final{"a": {Status: "known"}}, true},
		{"known-explains", evidence.Stream{Failed: true, Cases: map[string][]evidence.Execution{"a": {{Results: []string{"failed"}}}}}, map[string]evidence.Final{"a": {Status: "known"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := unresolvedStream(tc.stream, tc.final); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	for _, status := range []string{"pending", "undefined", "unknown"} {
		stream := evidence.Stream{Failed: true, Cases: map[string][]evidence.Execution{"a": {{Results: []string{"failed"}}}, "b": {{Results: []string{status}}}}}
		if !unresolvedStream(stream, map[string]evidence.Final{"a": {Status: "known"}, "b": {Status: status}}) {
			t.Fatalf("soft status %s hidden", status)
		}
	}
	sc := input.Scenario{Name: "one", Line: 1, Steps: []input.Step{{Result: input.Result{Status: input.StatusPassed}}}}
	f := input.Feature{URI: "one.feature", Elements: []input.Scenario{sc}}
	key := sourceLocation(f.URI, sc)
	if got := finalEvidence([]input.Feature{f}, map[string]string{key: "pending"})[key].Status; got != "pending" {
		t.Fatalf("final claim changed: %s", got)
	}
	for _, reason := range []string{"run-hook", "inferred", "missing", "feature-signal"} {
		copy := sc
		state := model.ScenarioMetadata(&copy)
		switch reason {
		case "run-hook":
			copy.Type = "run_hook"
			copy.Steps[0].Result.Status = input.StatusFailed
		case "inferred":
			state.InferredRetryFailure = true
		case "missing":
			state.MissingStepResults = true
		case "feature-signal":
			state.RunnerFailed = true
		}
		in := runInput{Attempts: []input.MessageAttempt{{URI: f.URI, Scenario: copy, RunnerFailed: true}}}
		if reason == "feature-signal" {
			in = runInput{Features: []input.Feature{{URI: f.URI, Elements: []input.Scenario{copy}}}}
		}
		streams := rawStreams([]runInput{in})
		if !streams[0].Failed || (reason != "feature-signal" && len(streams[0].Unexplained) == 0) {
			t.Fatalf("lost %s: %+v", reason, streams[0])
		}
	}
}

func TestRound16ComparisonGateRules(t *testing.T) {
	base := prepared(t, runFixture(), RunConfig{})
	for _, tc := range []struct {
		name   string
		config RunConfig
	}{
		{"incomplete", RunConfig{ExpectedScenarios: 3, Gates: GatePolicy{Incomplete: true}}},
		{"baseline", RunConfig{Gates: GatePolicy{NewFailures: true}}},
		{"duration", RunConfig{Baseline: &base.Summary, Gates: GatePolicy{MaxRegressionPercent: 20}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := runFixture()
			if tc.name == "duration" {
				*f[0].Elements[0].Steps[0].Result.Duration *= 4
			}
			now := prepared(t, f, tc.config)
			if len(now.Summary.GateFailures) == 0 {
				t.Fatal("policy failed to reject")
			}
		})
	}
}

func TestRound16HookPolicyFloor(t *testing.T) {
	for _, status := range []string{"failed", "ambiguous", "undefined"} {
		summary := RunSummary{Computed: Verdict{Tone: ToneFail}, Scenarios: []ScenarioSummary{{Status: status, RunHook: true}}}
		if checkPolicyFloor(summary, GatePolicy{Unexpected: true}) == nil {
			t.Fatalf("missing hook gate accepted: %s", status)
		}
	}
}
