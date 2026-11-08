package cucumbermodel

import "github.com/qatoolist/cucumber-reporter/internal/evidence"

// Evidence inventories results without calling Status, folding, or retry selection.
func Evidence(features []Feature) evidence.Counts {
	var out evidence.Counts
	var steps func([]Step, int)
	steps = func(xs []Step, depth int) {
		if depth > 64 {
			out.Add("failed")
			return
		}
		for _, s := range xs {
			out.Add(string(s.Result.Status))
			steps(s.Before, depth+1)
			steps(s.After, depth+1)
		}
	}
	var scenario func(Scenario, int)
	scenario = func(sc Scenario, depth int) {
		if depth > 64 {
			out.Add("failed")
			return
		}
		steps(sc.Before, 0)
		steps(sc.Steps, 0)
		steps(sc.After, 0)
		for _, a := range sc.scenarioState.PreviousAttempts {
			steps(a.scenarioState.AttemptBackground, 0)
			scenario(a, depth+1)
		}
	}
	for _, f := range features {
		for _, sc := range f.Elements {
			scenario(sc, 0)
		}
	}
	return out
}
