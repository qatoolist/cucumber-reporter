package cucumber

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
func (s Scenario) Status() Status {
	if len(s.Steps) == 0 {
		return StatusUndefined
	}
	status := StatusPassed
	for _, st := range s.Steps {
		status = worse(status, st.Result.Status)
	}
	return status
}

// Combine merges scenario statuses by multiple-cucumber-html-reporter's feature rule: failed,
// ambiguous or undefined when any scenario has that status (in that order); pending only when every
// scenario is pending; skipped only when every scenario is skipped or pending; otherwise passed. So a
// skipped or pending scenario beside passed ones leaves the whole passed. No statuses is passed.
func Combine(statuses ...Status) Status {
	var failed, ambiguous, undefined bool
	pending, skippedOrPending := 0, 0
	for _, s := range statuses {
		switch s {
		case StatusFailed:
			failed = true
		case StatusAmbiguous:
			ambiguous = true
		case StatusUndefined:
			undefined = true
		case StatusPending:
			pending++
			skippedOrPending++
		case StatusSkipped:
			skippedOrPending++
		}
	}
	switch {
	case failed:
		return StatusFailed
	case ambiguous:
		return StatusAmbiguous
	case undefined:
		return StatusUndefined
	case len(statuses) > 0 && pending == len(statuses):
		return StatusPending
	case len(statuses) > 0 && skippedOrPending == len(statuses):
		return StatusSkipped
	}
	return StatusPassed
}

// Status is the feature's overall outcome: Combine over its scenarios.
func (f Feature) Status() Status {
	statuses := make([]Status, len(f.Elements))
	for i, sc := range f.Elements {
		statuses[i] = sc.Status()
	}
	return Combine(statuses...)
}

// RunStatus is the outcome of a whole run: Combine over every scenario of every feature.
func RunStatus(features []Feature) Status {
	var statuses []Status
	for _, f := range features {
		for _, sc := range f.Elements {
			statuses = append(statuses, sc.Status())
		}
	}
	return Combine(statuses...)
}
