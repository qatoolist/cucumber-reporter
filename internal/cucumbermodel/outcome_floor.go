package cucumbermodel

import "github.com/qatoolist/cucumber-reporter/internal/evidence"

// ExecutionEvidence is an immutable, result-only record taken before folding.
// It deliberately does not call Status, Decide, or any renderer classification.
type ExecutionEvidence struct {
	Results []Status
	Own     bool
}

func scanExecution(sc Scenario, background []Step) ExecutionEvidence {
	e := ExecutionEvidence{Own: sc.Type == "orphan_background" || sc.Type == "run_hook"}
	var scan func([]Step, int)
	scan = func(steps []Step, depth int) {
		if depth > 64 {
			e.Results = append(e.Results, StatusFailed)
			return
		}
		for _, s := range steps {
			e.Results = append(e.Results, s.Result.Status)
			scan(s.Before, depth+1)
			scan(s.After, depth+1)
		}
	}
	for _, s := range sc.Steps {
		if s.stepState.Origin == "" || s.stepState.Origin == "own" {
			e.Own = true
		}
	}
	scan(background, 0)
	scan(sc.Before, 0)
	scan(sc.Steps, 0)
	scan(sc.After, 0)
	return e
}

// CaptureOutcomeFloor records the original per-execution evidence before any
// folding, hook hiding, retry selection or known-issue classification.
func CaptureOutcomeFloor(sc *Scenario, background []Step) {
	if sc.scenarioState.OutcomeFloor != nil {
		return
	}
	for _, a := range sc.scenarioState.PreviousAttempts {
		sc.scenarioState.OutcomeFloor = append(sc.scenarioState.OutcomeFloor, scanExecution(a, a.scenarioState.AttemptBackground))
	}
	sc.scenarioState.OutcomeFloor = append(sc.scenarioState.OutcomeFloor, scanExecution(*sc, background))
}

// CheckOutcome independently enforces the failure floor for this scenario.
// Matching counts in another scenario or an earlier attempt never rescues it.
func CheckOutcome(sc Scenario, status string, known map[int]bool) error {
	floor := sc.scenarioState.OutcomeFloor
	if len(floor) == 0 {
		CaptureOutcomeFloor(&sc, nil)
		floor = sc.scenarioState.OutcomeFloor
	}
	last := floor[len(floor)-1]
	strictPass := last.Own && len(last.Results) > 0
	earlierFailure := false
	for i, a := range floor {
		for _, s := range a.Results {
			if i == len(floor)-1 {
				strictPass = strictPass && s == StatusPassed
			} else {
				earlierFailure = earlierFailure || s == StatusFailed || s == StatusAmbiguous || s == StatusUndefined
			}
		}
	}
	bad := false
	if earlierFailure && !strictPass {
		bad = status != "failed" && status != "known"
	}
	if status == "passed" && !strictPass {
		bad = true
	}
	if status == "known" {
		// Known-issue coverage must prove every previous non-passing execution,
		// including pending attempts that are not classified as hard failures.
		if len(floor) > 1 && !strictPass {
			if len(sc.scenarioState.PreviousAttempts) != len(floor)-1 {
				bad = true
			}
			for n, prior := range sc.scenarioState.PreviousAttempts {
				flat, err := FoldScenarioSteps(prior, prior.scenarioState.AttemptBackground)
				if err != nil {
					bad = true
					continue
				}
				positions := independentlyCovered(sc, flat, known)
				represented := 0
				for i, st := range flat.Steps {
					if st.Result.Status == StatusFailed && positions[i+1] {
						represented++
						continue
					}
					if st.Result.Status != StatusPassed && st.Result.Status != StatusSkipped {
						bad = true
					}
				}
				required := 0
				if n >= len(floor)-1 {
					bad = true
					continue
				}
				for _, result := range floor[n].Results {
					if result == StatusFailed {
						required++
					}
					if result == StatusUndefined || result == StatusAmbiguous || result == StatusPending {
						bad = true
					}
				}
				if represented < required {
					bad = true
				}
			}
		}
		matched := false
		for i, s := range sc.Steps {
			if s.Result.Status == StatusFailed && known[i+1] {
				matched = true
				continue
			}
			if s.Result.Status != StatusPassed && s.Result.Status != StatusSkipped {
				bad = true
			}
		}
		if !strictPass && !matched {
			bad = true
		}
	}
	// Every hard failure in the final execution needs either its exact known
	// position or a hard outcome; evidence displayed in attempts is insufficient.
	hard := false
	for _, s := range last.Results {
		hard = hard || s == StatusFailed || s == StatusUndefined || s == StatusAmbiguous
	}
	if hard && status != "known" && status != "failed" && status != "undefined" && status != "ambiguous" {
		bad = true
	}
	if bad {
		return evidence.Floorf("evidence floor: scenario outcome contradicts its execution evidence; this is a reporter bug")
	}
	return nil
}

// independentlyCovered is deliberately separate from matchingKnownPositions.
// A signature is origin + declared ordinal + exact keyword/text; duplicate text
// at a different position never inherits a known annotation.
func independentlyCovered(current, prior Scenario, known map[int]bool) map[int]bool {
	type signature struct {
		origin        string
		ordinal       int
		keyword, text string
	}
	enumerate := func(s Scenario) []signature {
		out := make([]signature, len(s.Steps))
		background := 0
		for i, st := range s.Steps {
			ordinal := st.stepState.OwnIndex
			if st.stepState.Origin == "background" {
				background++
				ordinal = background
			}
			out[i] = signature{st.stepState.Origin, ordinal, st.Keyword, st.Name}
		}
		return out
	}
	expected := map[signature]bool{}
	for i, sig := range enumerate(current) {
		if known[i+1] {
			expected[sig] = true
		}
	}
	out := map[int]bool{}
	for i, sig := range enumerate(prior) {
		if expected[sig] {
			out[i+1] = true
		}
	}
	return out
}
