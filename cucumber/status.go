package cucumber

import modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

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

// Statuses lists every valid status in display order. Treat this compatibility
// variable as read-only; validation does not depend on its mutable contents.
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

// Status is the effective scenario outcome, including hooks and retry evidence.
// A failure is rescued only by a strictly passed final attempt.
// A scenario without steps is undefined, as Godog counts it.
func (s Scenario) Status() Status { return Status(internalScenario(s).Status()) }

// Combine merges scenario statuses by multiple-cucumber-html-reporter's feature rule: failed,
// ambiguous or undefined when any scenario has that status (in that order); pending only when every
// scenario is pending; skipped only when every scenario is skipped or pending; otherwise passed. So a
// skipped or pending scenario beside passed ones leaves the whole passed. No statuses is passed.
func Combine(statuses ...Status) Status {
	values := make([]modelmeta.Status, len(statuses))
	for i, s := range statuses {
		values[i] = modelmeta.Status(s)
	}
	return Status(modelmeta.Combine(values...))
}

// Status is the feature outcome after Background, hook and retry normalization.
func (f Feature) Status() Status { return RunStatus([]Feature{f}) }

// RunStatus returns the effective outcome after Background and retry folding.
// An empty execution has StatusUndefined; its report headline is informational.
func RunStatus(features []Feature) Status {
	return Status(modelmeta.RunStatus(internalFeatures(features)))
}
