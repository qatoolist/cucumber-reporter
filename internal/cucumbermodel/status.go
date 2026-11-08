package cucumbermodel

// Status is a step outcome as Godog writes it.
type Status string

// The six step statuses Godog emits. Any other value is rejected by Parse.
const (
	StatusPassed    Status = "passed"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
	StatusPending   Status = "pending"
	StatusUndefined Status = "undefined"
	StatusAmbiguous Status = "ambiguous"
)

// Statuses lists every valid status in display order.
var Statuses = []Status{StatusPassed, StatusFailed, StatusSkipped, StatusPending, StatusUndefined, StatusAmbiguous}

// rank orders statuses by severity: a scenario or feature takes the most severe status among its parts.
func (s Status) rank() int {
	switch s {
	case StatusFailed:
		return 6
	case StatusAmbiguous:
		return 5
	case StatusUndefined:
		return 4
	case StatusPending:
		return 3
	case StatusSkipped:
		return 2
	case StatusPassed:
		return 1
	}
	return 0
}

// Valid reports whether s is one of the six statuses.
func (s Status) Valid() bool { return s.rank() > 0 }

// worse returns the more severe of a and b.
func worse(a, b Status) Status {
	if b.rank() > a.rank() {
		return b
	}
	return a
}

// Status is the scenario's overall outcome: the most severe step status, in the order failed,
// ambiguous, undefined, pending, skipped, passed. A failed step followed by skipped steps is a failure.
// A scenario without steps is undefined, as Godog counts it.
func executionStatus(s Scenario) Status {
	hasOwnSteps := s.Type == "orphan_background" || s.Type == "run_hook"
	status := StatusPassed
	var scan func([]Step, int)
	scan = func(steps []Step, depth int) {
		if depth > 64 {
			status = StatusFailed
			return
		}
		for _, st := range steps {
			status = worse(status, st.Result.Status)
			if len(st.Before) > 0 {
				scan(st.Before, depth+1)
			}
			if len(st.After) > 0 {
				scan(st.After, depth+1)
			}
		}
	}
	for _, st := range s.Steps {
		if st.stepState.Origin == "" || st.stepState.Origin == "own" {
			hasOwnSteps = true
			break
		}
	}
	scan(s.Steps, 0)
	scan(s.Before, 0)
	scan(s.After, 0)
	if !hasOwnSteps {
		status = worse(status, StatusUndefined)
	}

	return status
}

// Combine merges scenario statuses by multiple-cucumber-html-reporter's feature rule: failed,
// ambiguous or undefined when any scenario has that status (in that order); pending only when every
// scenario is pending; skipped only when every scenario is skipped or pending; otherwise passed. So a
// skipped or pending scenario beside passed ones leaves the whole passed. No statuses is passed.
func Combine(statuses ...Status) Status {
	values := make([]string, len(statuses))
	for i, s := range statuses {
		values[i] = string(s)
	}
	return Status(CombineOutcomes(values))
}

// CombineOutcomes is the shared feature/run aggregation rule, including known issues.
func CombineOutcomes(statuses []string) string {
	seen := map[string]bool{}
	pending, skippedOrPending := 0, 0
	for _, s := range statuses {
		seen[s] = true
		switch s {
		case "pending":
			pending++
			skippedOrPending++
		case "skipped":
			skippedOrPending++
		}
	}
	for _, s := range []string{"failed", "ambiguous", "undefined", "known"} {
		if seen[s] {
			return s
		}
	}
	switch {
	case len(statuses) > 0 && pending == len(statuses):
		return "pending"
	case len(statuses) > 0 && skippedOrPending == len(statuses):
		return "skipped"
	}
	return "passed"
}

// Status is the feature's overall outcome: Combine over its scenarios.
func (f Feature) Status() Status {
	statuses := make([]Status, len(f.Elements))
	for i, sc := range f.Elements {
		statuses[i] = sc.Status()
	}
	if len(statuses) == 0 {
		return StatusUndefined
	}
	return Combine(statuses...)
}

// RunStatus is the outcome of a whole run: Combine over every scenario of every feature.
func RunStatus(features []Feature) Status {
	if RunnerFailed(features) {
		return StatusFailed
	}
	var err error
	features, err = PlainFeatures(features)
	if err != nil {
		return StatusFailed
	}
	var statuses []Status
	for _, f := range features {
		for _, sc := range f.Elements {
			statuses = append(statuses, sc.Status())
		}
	}
	if len(statuses) == 0 {
		return StatusUndefined
	}
	return Combine(statuses...)
}

// Decision is the sole scenario outcome authority. Known contains execution
// positions whose exact annotation matched; observations apply only to a clean pass.
type Decision struct {
	Status, Raw Status
	Known       map[int]bool
	Flaky       bool
	Reason      string
}

// Decide accounts for every result in the final execution and every earlier
// attempt. A retry rescues failure only when the complete final execution passed.
func Decide(s Scenario, declared, observed map[int]bool) Decision {
	raw := executionStatus(s)
	d := Decision{Raw: raw, Status: raw}
	priorFailure, priorNonpass := false, false
	for _, status := range s.scenarioState.ReportedAttempts {
		priorFailure = priorFailure || status == StatusFailed || status == StatusUndefined || status == StatusAmbiguous
		priorNonpass = priorNonpass || status != StatusPassed
	}
	for _, p := range s.scenarioState.PreviousAttempts {
		status := executionStatus(p)
		for _, st := range p.scenarioState.AttemptBackground {
			status = worse(status, st.Result.Status)
		}
		priorFailure = priorFailure || status == StatusFailed || status == StatusAmbiguous || status == StatusUndefined
		priorNonpass = priorNonpass || status != StatusPassed
	}
	if raw != StatusPassed && priorFailure {
		d.Status = StatusFailed
		d.Reason = "earlier failure was not rescued by a passing retry"
	}
	if raw == StatusPassed {
		d.Flaky = priorNonpass
		if priorNonpass {
			d.Reason = "retry rescued by a passing final execution"
		}
	}
	if raw == StatusFailed {
		allowed, matched := true, false
		for i, st := range s.Steps {
			switch st.Result.Status {
			case StatusPassed, StatusSkipped:
			case StatusFailed:
				if declared[i+1] {
					matched = true
				} else {
					allowed = false
				}
			default:
				allowed = false
			}
			if len(st.Before) > 0 || len(st.After) > 0 { // caller models need not already be folded
				for _, hook := range append(append([]Step(nil), st.Before...), st.After...) {
					if hook.Result.Status != StatusPassed && hook.Result.Status != StatusSkipped {
						allowed = false
					}
				}
			}
		}
		for _, hook := range append(append([]Step(nil), s.Before...), s.After...) {
			if hook.Result.Status != StatusPassed && hook.Result.Status != StatusSkipped {
				allowed = false
			}
		}
		for _, prior := range s.scenarioState.PreviousAttempts {
			flat, err := FoldScenarioSteps(prior, prior.scenarioState.AttemptBackground)
			if err != nil {
				allowed = false
				continue
			}
			positions := matchingKnownPositions(s, flat, declared)
			for i, st := range flat.Steps {
				if st.Result.Status != StatusPassed && st.Result.Status != StatusSkipped && (st.Result.Status != StatusFailed || !positions[i+1]) {
					allowed = false
				}
			}
		}
		// Status-only metadata cannot prove where an earlier failure occurred.
		for _, status := range s.scenarioState.ReportedAttempts {
			if status != StatusPassed && status != StatusSkipped {
				allowed = false
			}
		}
		if matched && allowed {
			d.Status = "known"
			d.Known = declared
			d.Reason = "annotated known failure"
		} else if matched {
			d.Reason = "known issue plus unexpected failure"
		}
	}
	if raw == StatusPassed && len(observed) > 0 {
		d.Status = "known"
		d.Known = observed
		d.Reason = "observed known issue"
	}
	return d
}

// Status returns the same effective execution outcome used by rendering and gates.
func (s Scenario) Status() Status { return Decide(s, nil, nil).Status }

// RunnerFailed preserves the run-level runner signal even in a public snapshot.
func RunnerFailed(features []Feature) bool {
	for _, f := range features {
		for _, sc := range f.Elements {
			if sc.scenarioState.RunnerFailed {
				return true
			}
		}
	}
	return false
}

// matchingKnownPositions maps declared step failures through retry execution layouts.
func matchingKnownPositions(current, prior Scenario, known map[int]bool) map[int]bool {
	out := map[int]bool{}
	background := 0
	for i, st := range current.Steps {
		if st.stepState.Origin == "background" {
			background++
		}
		if !known[i+1] {
			continue
		}
		priorBackground := 0
		for j, before := range prior.Steps {
			if before.stepState.Origin == "background" {
				priorBackground++
			}
			if st.stepState.Origin == "background" && background != priorBackground {
				continue
			}
			if st.stepState.Origin == before.stepState.Origin && st.stepState.OwnIndex == before.stepState.OwnIndex && st.Name == before.Name && st.Keyword == before.Keyword {
				out[j+1] = true
			}
		}
	}
	return out
}
