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
func (s Scenario) Status() Status {
	status := StatusPassed
	for _, st := range s.Steps {
		status = worse(status, st.Result.Status)
	}
	return status
}

// Status is the feature's overall outcome: the most severe scenario status.
func (f Feature) Status() Status {
	status := StatusPassed
	for _, sc := range f.Elements {
		status = worse(status, sc.Status())
	}
	return status
}
