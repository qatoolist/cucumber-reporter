package cucumbermodel

import (
	"testing"

	"github.com/qatoolist/cucumber-reporter/internal/evidence"
)

func execution(name string, line int, status Status) Scenario {
	return Scenario{Name: name, Line: line, Steps: []Step{{Name: "step", Match: Match{Location: "steps.rb:1"}, Result: Result{Status: status}}}}
}

func TestRetryOutcomesAndOriginalSlots(t *testing.T) {
	for _, final := range []Status{StatusPassed, StatusPending, StatusSkipped, StatusUndefined, StatusFailed} {
		t.Run(string(final), func(t *testing.T) {
			f := []Feature{{Elements: []Scenario{execution("retry", 3, StatusFailed), execution("retry", 3, final), execution("other", 3, StatusPassed)}}}
			out, err := PlainFeatures(f)
			if err != nil {
				t.Fatal(err)
			}
			want := StatusFailed
			if final == StatusPassed {
				want = StatusPassed
			}
			if got := out[0].Elements[0].Status(); got != want {
				t.Fatalf("got %s want %s", got, want)
			}
			if out[0].Elements[0].scenarioState.NavigationSuffix != "-L3-2" || out[0].Elements[1].scenarioState.NavigationSuffix != "-L3-3" {
				t.Fatal("original ID slots renumbered")
			}
			if out[0].featureState.BackgroundSlots["-L3"] != "-L3-2" {
				t.Fatal("missing retry redirect")
			}
			if err := evidence.Check(Evidence(f), Evidence(out)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUnconfirmedDuplicatesDoNotMerge(t *testing.T) {
	a, b := execution("duplicate", 3, StatusFailed), execution("duplicate", 3, StatusPassed)
	a.Steps[0].Match.Location = "Steps.java:1"
	b.Steps[0].Match.Location = "Steps.java:1"
	bg := execution("background", 1, StatusPassed)
	bg.Type = "background"
	got, err := PlainFeatures([]Feature{{Elements: []Scenario{bg, a, bg, b}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Elements) != 2 || got[0].Status() != StatusFailed {
		t.Fatal("unconfirmed retry rescued a failure")
	}
}

func TestBackgroundHookEvidenceAndOrphan(t *testing.T) {
	bg := execution("setup", 2, StatusPassed)
	bg.Type = "background"
	bg.Before = []Step{{Result: Result{Status: StatusPassed}, Embeddings: []Embedding{{Data: "before"}}}}
	sc := execution("example", 5, StatusPassed)
	got, err := PlainFeatures([]Feature{{Elements: []Scenario{bg, sc}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Elements[0].Steps) != 2 || len(got[0].Elements[0].Steps[0].Embeddings) != 1 {
		t.Fatal("passing hook evidence lost or became a step")
	}
	bg.Steps[0].Result.Status = StatusFailed
	got, err = PlainFeatures([]Feature{{Elements: []Scenario{bg}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Elements) != 1 || got[0].Elements[0].Line != 2 || got[0].Status() != StatusFailed {
		t.Fatal("orphan failure lost")
	}
}

func TestEvidenceInventoryIndependentOfFolding(t *testing.T) {
	sc := execution("s", 2, StatusPassed)
	sc.Before = []Step{{Result: Result{Status: StatusFailed}}}
	sc.Steps[0].After = []Step{{Result: Result{Status: StatusFailed}}}
	raw := Evidence([]Feature{{Elements: []Scenario{sc}}})
	folded, err := FoldScenarioSteps(sc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := evidence.Check(raw, Evidence([]Feature{{Elements: []Scenario{folded}}})); err != nil {
		t.Fatal(err)
	}
	folded.Steps = folded.Steps[1:]
	if evidence.Check(raw, Evidence([]Feature{{Elements: []Scenario{folded}}})) == nil {
		t.Fatal("missing hook not caught")
	}
}
