package cucumber

import (
	"testing"

	"github.com/qatoolist/cucumber-reporter/internal/evidence"
)

func TestRawFloorRejectsTransferredFailure(t *testing.T) {
	raw, err := evidence.Scan([]byte(`[{"uri":"a.feature","elements":[{"line":3,"steps":[{"result":{"status":"failed"}}]},{"line":6,"steps":[{"result":{"status":"passed"}}]}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	// The same total failure count, falsely attributed to the other scenario.
	f := []Feature{{URI: "a.feature", Elements: []Scenario{{Line: 3, Steps: []Step{{Result: Result{Status: StatusPassed}}}}, {Line: 6, Steps: []Step{{Result: Result{Status: StatusFailed}}}}}}}
	if err = checkRawScenarioEvidence(raw, f); err == nil {
		t.Fatal("failure moved between scenarios escaped raw floor")
	}
}
