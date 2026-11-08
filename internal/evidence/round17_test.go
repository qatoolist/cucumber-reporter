package evidence

import (
	"errors"
	"testing"
)

// F4-N2: each floor rule alone is pinned by a direct unit test, so weakening
// the floor is caught even when production still decides correctly.
func TestRound17FloorRulesAlone(t *testing.T) {
	refused := func(name string, err error) {
		t.Helper()
		var floor *FloorError
		if !errors.As(err, &floor) {
			t.Fatalf("%s: floor accepted: %v", name, err)
		}
	}
	// CheckRunner: a rescued retry is a strict-flaky cause that a hard failure
	// elsewhere cannot explain.
	flaky := RunnerAudit{Failed: true, Cases: [][]Execution{{{Results: []string{"failed"}}, {Results: []string{"passed"}}}, {{Results: []string{"failed"}}}}}
	refused("strict-flaky cause", CheckRunner(flaky.Encode(), false))
	if err := CheckRunner(flaky.Encode(), true); err != nil {
		t.Fatal(err)
	}
	// CheckRunner: a runner failure with no hard failure anywhere is unaccounted.
	refused("uncovered runner failure", CheckRunner(RunnerAudit{Failed: true, Cases: [][]Execution{{{Results: []string{"passed"}}}}}.Encode(), false))
	// CheckCases: an execution without own steps or results cannot become green.
	for _, final := range []Execution{{Results: []string{"passed"}, NoOwnSteps: true}, {Results: []string{"passed"}, MissingResults: true}} {
		audit := RunnerAudit{Keys: []string{"k"}, Cases: [][]Execution{{final}}}.Encode()
		for _, status := range []string{"passed", "known"} {
			refused("incomplete "+status, CheckCases(audit, map[string]Final{"k": {Status: status}}))
		}
		if err := CheckCases(audit, map[string]Final{"k": {Status: "undefined"}}); err != nil {
			t.Fatal(err)
		}
	}
	// Messages.Check: the raw runner failure signal must survive parsing.
	var m Messages
	for _, event := range []string{
		`{"testCaseStarted":{"id":"one","testCaseId":"c"}}`,
		`{"testStepFinished":{"testCaseStartedId":"one","testStepResult":{"status":"PASSED"}}}`,
		`{"testRunFinished":{"success":false}}`,
	} {
		if err := m.Observe([]byte(event)); err != nil {
			t.Fatal(err)
		}
	}
	shown := map[string]Execution{"one": {Results: []string{"passed"}}}
	refused("lost runner signal", m.Check(shown, false))
	if err := m.Check(shown, true); err != nil {
		t.Fatal(err)
	}
}
