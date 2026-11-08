package cucumbermodel

import "testing"

func TestRunAndExecutionStatus(t *testing.T) {
	for _, status := range []Status{StatusPassed, StatusFailed, StatusPending, StatusSkipped, StatusUndefined, StatusAmbiguous} {
		sc := Scenario{Steps: []Step{{Name: "one", Result: Result{Status: status}}}}
		if got := ExecutionStatus(sc); got != status {
			t.Fatalf("raw %s != %s", got, status)
		}
		if got := RunStatus([]Feature{{Elements: []Scenario{sc}}}); got != status {
			t.Fatalf("run %s != %s", got, status)
		}
	}
	if got := RunStatus(nil); got != StatusUndefined {
		t.Fatal(got)
	}
	sc := Scenario{Steps: []Step{{Result: Result{Status: StatusPending}}}}
	sc.scenarioState.PreviousAttempts = []Scenario{{Steps: []Step{{Result: Result{Status: StatusFailed}}}}}
	if ExecutionStatus(sc) != StatusPending || sc.Status() != StatusFailed {
		t.Fatal("raw and effective retry outcomes mixed")
	}
}

func TestKnownBackgroundRetriesKeepDeclaredPosition(t *testing.T) {
	makeScenario := func(failed int) Scenario {
		t.Helper()
		sc, err := FoldScenarioSteps(Scenario{Steps: []Step{{Name: "own", Result: Result{Status: StatusSkipped}}}}, []Step{{Name: "same", Result: Result{Status: StatusPassed}}, {Name: "same", Result: Result{Status: StatusPassed}}})
		if err != nil {
			t.Fatal(err)
		}
		sc.Steps[failed].Result.Status = StatusFailed
		return sc
	}
	sc := makeScenario(0)
	sc.scenarioState.PreviousAttempts = []Scenario{makeScenario(1)}
	if d := Decide(sc, map[int]bool{1: true}, nil); d.Status != "failed" {
		t.Fatalf("another Background failure hidden: %+v", d)
	}
	sc.scenarioState.PreviousAttempts = []Scenario{makeScenario(0)}
	if d := Decide(sc, map[int]bool{1: true}, nil); d.Status != "known" {
		t.Fatalf("same Background failure not covered: %+v", d)
	}
}
