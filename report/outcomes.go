package report

import (
	"math"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// Scenario outcomes returned by ScenarioOutcomes: the six cucumber statuses, as strings, and
// OutcomeKnown.
const (
	OutcomePassed    = string(cucumber.StatusPassed)
	OutcomeFailed    = string(cucumber.StatusFailed)
	OutcomeKnown     = "known"
	OutcomeAmbiguous = string(cucumber.StatusAmbiguous)
	OutcomeUndefined = string(cucumber.StatusUndefined)
	OutcomePending   = string(cucumber.StatusPending)
	OutcomeSkipped   = string(cucumber.StatusSkipped)
)

// ScenarioOutcomes returns each scenario's effective outcome after the known-issue rules documented on
// Annotation, indexed like ScenarioIDs: OutcomeKnown for a known failure, otherwise the scenario's
// cucumber status. It is the classification the report itself shows, so callers that need the numbers
// (for example the count of unexpected failures, the OutcomeFailed entries) never re-implement the
// rules. The annotations are validated as Render validates them.
func ScenarioOutcomes(features []cucumber.Feature, annotations []Annotation) ([][]string, error) {
	opt, err := normalize(Options{Version: OptionsVersion, Annotations: annotations})
	if err != nil {
		return nil, err
	}
	if err = cucumber.Validate(features); err != nil {
		return nil, err
	}
	sids := ScenarioIDs(features)
	steps := map[string]int{}
	anns := map[string][]*annotationData{}
	for i, f := range features {
		for j, sc := range f.Elements {
			steps[sids[i][j]] = len(sc.Steps)
		}
	}
	if err = checkAnnotations(opt.Annotations, steps); err != nil {
		return nil, err
	}
	data := make([]annotationData, len(opt.Annotations))
	for i, a := range opt.Annotations {
		data[i] = annotationData{Annotation: cleanAnnotation(a)}
		anns[a.Scenario] = append(anns[a.Scenario], &data[i])
	}
	out := make([][]string, len(features))
	for i, f := range features {
		out[i] = make([]string, len(f.Elements))
		for j, sc := range f.Elements {
			// Attachments play no part in the outcome: an inline limit below zero keeps none.
			sd, err := buildScenario(sc, sids[i][j], anns[sids[i][j]], i, j, cucumber.Limits{Embedding: math.MaxInt64, InlineAttachment: -1}, nil)
			if err != nil {
				return nil, err
			}
			out[i][j] = sd.Status
		}
	}
	return out, nil
}
