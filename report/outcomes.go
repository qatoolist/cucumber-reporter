package report

import (
	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// Scenario outcomes returned by ScenarioOutcomes: the six cucumber statuses, as strings, and
// OutcomeKnown and OutcomeFolded.
const (
	OutcomePassed    = string(cucumber.StatusPassed)
	OutcomeFailed    = string(cucumber.StatusFailed)
	OutcomeKnown     = "known"
	OutcomeAmbiguous = string(cucumber.StatusAmbiguous)
	OutcomeUndefined = string(cucumber.StatusUndefined)
	OutcomePending   = string(cucumber.StatusPending)
	OutcomeSkipped   = string(cucumber.StatusSkipped)
)

// scenarioOutcomes returns each scenario's effective outcome after the known-issue rules documented on
// Annotation, indexed like scenarioIDs: OutcomeKnown for a known failure, otherwise the scenario's
// cucumber status. It is the classification the report itself shows, so callers that need the numbers
// (for example the count of unexpected failures, the OutcomeFailed entries) never re-implement the
// rules. The annotations are validated as renderPublicModel validates them.
func scenarioOutcomes(features []cucumber.Feature, annotations []Annotation) ([][]string, error) {
	opt, err := normalize(renderOptions{Version: OptionsVersion, Annotations: annotations})
	if err != nil {
		return nil, err
	}
	if err = validateRenderFeatures(features); err != nil {
		return nil, err
	}
	page, err := buildPage(features, opt, nil)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, f := range page.Features {
		for _, s := range f.Elements {
			values[s.ID] = s.Status
		}
	}
	ids := scenarioIDs(features)
	out := make([][]string, len(features))
	for i, row := range ids {
		out[i] = make([]string, len(row))
		for j, id := range row {
			out[i][j] = values[id]
			if out[i][j] == "" {
				out[i][j] = OutcomeFolded
			}
		}
	}
	return out, nil
}

// OutcomeFolded denotes a source Background or retry slot folded into another
// scenario, or source-only evidence with no executed destination. Exclude these
// slots when counting ScenarioOutcomes results.
const OutcomeFolded = "folded"
