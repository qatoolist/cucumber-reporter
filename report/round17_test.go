package report

import (
	"reflect"
	"strings"
	"testing"
	"time"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func durationRun(ms int64) []cucumber.Feature {
	d := time.Duration(ms) * time.Millisecond
	return []cucumber.Feature{{URI: "features/a.feature", Name: "F", Line: 1, Elements: []cucumber.Scenario{{Name: "s", Line: 3, Type: "scenario", Steps: []cucumber.Step{{Keyword: "Given ", Name: "ok", Line: 4, Result: cucumber.Result{Status: cucumber.StatusPassed, Duration: &d}}}}}}}
}

// G4-N1: the gate and the policy floor share one exact predicate, so a run that
// sits exactly on the percentage boundary is neither a regression nor refused.
func TestRound17DurationRegressionBoundaryIsExact(t *testing.T) {
	for _, tc := range []struct {
		base, now int64
		pct       float64
		regressed bool
	}{
		{1000, 1167, 16.6, true},
		{1000, 1167, 16.7, false},
		{1000, 1167, 16.8, false},
		{1000, 1189, 18.9, false},
		{1000, 1190, 18.9, true},
		{2000, 2334, 16.7, false},
		{10000, 11890, 18.9, false},
	} {
		base := prepared(t, durationRun(tc.base), RunConfig{ID: "base"})
		now, err := preparePublicRun([]runInput{{Name: "cur", Features: durationRun(tc.now)}}, renderOptions{Version: 2}, RunConfig{ID: "cur", Baseline: &base.Summary, Gates: GatePolicy{MaxRegressionPercent: tc.pct}})
		if err != nil {
			t.Fatalf("%d->%d at %v%%: %v", tc.base, tc.now, tc.pct, err)
		}
		got := len(now.Summary.GateFailures) == 1 && now.Summary.GateFailures[0] == "duration regression"
		if got != tc.regressed || (!tc.regressed && len(now.Summary.GateFailures) != 0) {
			t.Fatalf("%d->%d at %v%%: gate failures %v, want regression=%v", tc.base, tc.now, tc.pct, now.Summary.GateFailures, tc.regressed)
		}
	}
}

func TestRound17DurationRegressionPredicateMatchesDecimalMath(t *testing.T) {
	// Exhaustive version of fixtures/r17/G4/fp4.py: every tenth of a percent on
	// common baselines, compared with exact decimal arithmetic.
	for p10 := int64(1); p10 < 1000; p10++ {
		pct := float64(p10) / 10
		for _, baseMs := range []int64{1000, 2000, 4000, 5000, 10000} {
			base := baseMs * 1_000_000
			for _, delta := range []int64{base * p10 / 1000, base*p10/1000 + 1, base*p10/1000 - 1} {
				if delta < 100_000_000 {
					continue
				}
				want := delta*1000 > p10*base
				if got := durationRegressed(delta, base, GatePolicy{MaxRegressionPercent: pct}); got != want {
					t.Fatalf("pct %v base %d delta %d: got %v want %v", pct, base, delta, got, want)
				}
			}
		}
	}
}

// F4-N1: the floor re-derives known-issue coverage from the annotations and the
// raw step text; a page that marks a step known without a matching annotation
// (mutant K1: stepText check skipped) is refused at runtime.
func TestRound17FloorRederivesKnownCoverage(t *testing.T) {
	for _, file := range []string{"run.json", "run.ndjson"} {
		in := round14Input(t, "one", "../testdata/round16/F3/cap/k25-known-z-only/"+file)
		opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "a-a8fde43a2452-L9", Kind: KindKnown, Ref: "K", Title: "t", Step: 1, StepText: "step z"}}}
		p, err := preparePublicRun([]runInput{in}, opt, RunConfig{})
		if err != nil {
			t.Fatal(err)
		}
		page, err := buildPage(p.Features, p.Options, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = checkScenarioOutcomes(p.Features, page, nil, nil); err != nil {
			t.Fatalf("%s: matching annotation refused: %v", file, err)
		}
		known := false
		for _, f := range page.Features {
			for _, sc := range f.Elements {
				known = known || sc.Status == "known"
			}
		}
		if !known {
			t.Fatalf("%s: fixture no longer produces a known scenario", file)
		}
		for i := range page.Annotations {
			page.Annotations[i].StepText = "step WRONG"
		}
		if err = checkScenarioOutcomes(p.Features, page, nil, nil); err == nil || !strings.Contains(err.Error(), "evidence floor") {
			t.Fatalf("%s: known status without a matching annotation accepted: %v", file, err)
		}
	}
}

// F4-N6: the former runner overlay on summary.Computed (G5) and the "runner
// reported failure" gate append (G6) were dead: every scenario carries the
// runner signal, so the classified page is already failed (a runner failure
// without scenarios is refused by the parser), and evaluateGates replaced the
// appended entry. This pins the resulting verdict and gate list.
func TestRound17RunnerFailureGate(t *testing.T) {
	in := round14Input(t, "x", "../testdata/round14/messages/success_false_allpass.ndjson")
	for _, unexpected := range []bool{false, true} {
		p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{Gates: GatePolicy{Unexpected: unexpected}})
		if err != nil {
			t.Fatal(err)
		}
		if !p.Summary.RunnerFailed || p.Summary.Computed != (Verdict{Label: "Runner reported failure", Tone: ToneFail}) {
			t.Fatalf("runner failure verdict %+v", p.Summary.Computed)
		}
		want := []string(nil)
		if unexpected {
			want = []string{"unexpected failures"}
		}
		if !reflect.DeepEqual(p.Summary.GateFailures, want) {
			t.Fatalf("unexpected=%v: gate failures %v, want %v", unexpected, p.Summary.GateFailures, want)
		}
	}
}
