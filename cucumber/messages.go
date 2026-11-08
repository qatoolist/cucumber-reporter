package cucumber

import (
	"io"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	model "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// MessageAttempt is one runner execution. Scenario uses the same public model as JSON.
// Number is the zero-based arrival order for this test case, independent of runner counters.
type MessageAttempt struct {
	// IdentityCatalog preserves full-source name counts for stable subset navigation.
	IdentityCatalog                 map[string]int `json:",omitempty"`
	URI                             string
	FeatureName, FeatureDescription string
	FeatureTags                     []Tag
	Scenario                        Scenario
	Number                          int
	Started, Finished, Worker       string
	// RunnerFailed preserves testRunFinished.success=false, including strict-flaky runs.
	RunnerFailed bool
}

// MessageRun retains attempts and runner-reported provenance. Git values are facts,
// not automatic baseline/history scope settings.
type MessageRun struct {
	Attempts                 []MessageAttempt
	Started, Finished        string
	Revision, Branch, Remote string
}

// ParseMessages reads offline Cucumber Messages, retaining hooks and retries.
func ParseMessages(r io.Reader, lim Limits) (MessageRun, error) {
	raw, err := input.ParseMessages(r, model.Limits(lim))
	if err != nil {
		return MessageRun{}, publicError(err)
	}
	out := MessageRun{Started: raw.Started, Finished: raw.Finished, Revision: raw.Revision, Branch: raw.Branch, Remote: raw.Remote, Attempts: make([]MessageAttempt, len(raw.Attempts))}
	for i, a := range raw.Attempts {
		out.Attempts[i] = MessageAttempt{URI: a.URI, FeatureName: a.FeatureName, FeatureDescription: a.FeatureDescription, FeatureTags: publicTags(a.FeatureTags), Scenario: publicScenario(a.Scenario), Number: a.Number, Started: a.Started, Finished: a.Finished, Worker: a.Worker, RunnerFailed: a.RunnerFailed}
		out.Attempts[i].IdentityCatalog = model.AttemptCatalog(&a)
	}
	return out, nil
}
