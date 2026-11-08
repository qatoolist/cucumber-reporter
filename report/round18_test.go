package report

import (
	"strings"
	"testing"
)

// K5-M1: the floor re-derives "not applied" known issues from the raw RunConfig
// issue policies, not from the PolicyReason that applyIssuePolicies writes.
func TestRound18FloorRederivesIssuePolicies(t *testing.T) {
	for name, cfg := range map[string]RunConfig{
		"environment": {Environment: "prod", Issues: []IssuePolicy{{Ref: "K", Environments: []string{"staging"}}}},
		"expired":     {RecordedAt: "2026-03-02T10:00:00Z", Issues: []IssuePolicy{{Ref: "K", Expires: "2026-03-01"}}},
		"signature":   {Issues: []IssuePolicy{{Ref: "K", ErrorPattern: "^not this error$"}}},
	} {
		for _, file := range []string{"run.json", "run.ndjson"} {
			in := round14Input(t, "one", "../testdata/round16/F3/cap/k25-known-z-only/"+file)
			opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "a-a8fde43a2452-L9", Kind: KindKnown, Ref: "K", Title: "t", Step: 1, StepText: "step z"}}}
			p, err := preparePublicRun([]runInput{in}, opt, cfg)
			if err != nil {
				t.Fatalf("%s %s: %v", name, file, err)
			}
			if p.Summary.Computed.Tone != ToneFail {
				t.Fatalf("%s %s: policy-excluded known issue still applied: %+v", name, file, p.Summary.Computed)
			}
			// Simulate a production bug that loses the policy decision.
			lost := p.Options
			lost.Annotations = append([]Annotation(nil), lost.Annotations...)
			for i := range lost.Annotations {
				lost.Annotations[i].PolicyReason = ""
			}
			lost.Analysis = nil
			page, err := buildPage(p.Features, lost, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = checkScenarioOutcomes(p.Features, page, &cfg, nil); err == nil || !strings.Contains(err.Error(), "evidence floor") {
				t.Fatalf("%s %s: lost issue policy accepted: %v", name, file, err)
			}
		}
	}
}
