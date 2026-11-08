package cucumbermodel

import "testing"

func TestKnownOutcomeFloorInspectsSoftPriorAttempts(t *testing.T) {
	for _, status := range []Status{StatusPending, StatusUndefined, StatusAmbiguous, StatusPassed, StatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			prior := Scenario{Name: "known retry", Steps: []Step{{Name: "x", Keyword: "Given ", Result: Result{Status: status}}}}
			current := Scenario{Name: "known retry", Steps: []Step{{Name: "x", Keyword: "Given ", Result: Result{Status: StatusFailed}}}}
			current.scenarioState.PreviousAttempts = []Scenario{prior}
			folded, err := FoldScenarioSteps(current, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = CheckOutcome(folded, "known", map[int]bool{1: true})
			wantError := status == StatusPending || status == StatusUndefined || status == StatusAmbiguous
			if (err != nil) != wantError {
				t.Fatalf("known claim after %s: error %v, wantError %v", status, err, wantError)
			}
		})
	}
}
