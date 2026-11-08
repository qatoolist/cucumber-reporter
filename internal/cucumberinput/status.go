package cucumber

import modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

// Status is a step outcome.
type Status = modelmeta.Status

// The six supported step outcomes.
const (
	StatusPassed    = modelmeta.StatusPassed
	StatusFailed    = modelmeta.StatusFailed
	StatusSkipped   = modelmeta.StatusSkipped
	StatusPending   = modelmeta.StatusPending
	StatusUndefined = modelmeta.StatusUndefined
	StatusAmbiguous = modelmeta.StatusAmbiguous
)

// Statuses lists valid statuses in display order; callers should not modify it.
var Statuses = modelmeta.Statuses

// Combine merges scenario outcomes using the feature verdict rule.
func Combine(statuses ...Status) Status { return modelmeta.Combine(statuses...) }

// RunStatus returns the overall outcome.
func RunStatus(features []Feature) Status { return modelmeta.RunStatus(features) }
