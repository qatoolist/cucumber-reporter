package report

import (
	"errors"
	"strings"
	"testing"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"
	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRepeatedInputsListEveryLocation(t *testing.T) {
	feature := func(lines ...int) []pub.Feature {
		f := pub.Feature{URI: "features/cells.feature", Name: "Cells"}
		for _, line := range lines {
			f.Elements = append(f.Elements, pub.Scenario{Name: "cell", Line: line, Steps: []pub.Step{{Result: pub.Result{Status: pub.StatusPassed}}}})
		}
		return []pub.Feature{f}
	}
	_, err := PrepareRun([]Input{{Name: "chrome/input.json", Features: feature(4, 12)}, {Name: "firefox/input.json", Features: feature(4, 12)}}, Options{Version: 2}, RunConfig{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("category: %v", err)
	}
	for _, part := range []string{"chrome/input.json", "firefox/input.json", `"features/cells.feature":4`, `"features/cells.feature":12`, "not supported in v0.3.0", "one report per run or cell", "--retry within a single run"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("missing %q in %v", part, err)
		}
	}
	p, err := PrepareRun([]Input{{Name: "chrome", Features: feature(4)}, {Name: "firefox", Features: feature(12)}}, Options{Version: 2}, RunConfig{})
	if err != nil || p.Summary.ObservedScenarios != 2 {
		t.Fatalf("disjoint shards: %v %v", p, err)
	}
}

func TestRound16CollisionDiagnosticBounded(t *testing.T) {
	var f input.Feature
	f.URI = "features/" + strings.Repeat("界", 200) + ".feature"
	for i := 1; i <= 5000; i++ {
		f.Elements = append(f.Elements, input.Scenario{Name: "same", Line: i})
	}
	err := rejectRepeatedInputs([]runInput{{Name: "a.json", Features: []input.Feature{f}}, {Name: "b.json", Features: []input.Feature{f}}})
	if err == nil || len(err.Error()) >= 4096 || !strings.Contains(err.Error(), "and 4980 more") {
		t.Fatalf("unbounded or incomplete diagnostic: %v", err)
	}
}

func TestRound17CollisionListOrderAndTruncation(t *testing.T) {
	f := input.Feature{URI: "features/cells.feature"}
	for _, line := range []int{10000, 100, 9, 1000, 10} {
		f.Elements = append(f.Elements, input.Scenario{Name: "same", Line: line})
	}
	g := f
	g.Elements = append([]input.Scenario(nil), f.Elements...)
	g.Elements = append(g.Elements, input.Scenario{Name: "only b", Line: 7})
	err := rejectRepeatedInputs([]runInput{{Name: "a.json", Features: []input.Feature{f}}, {Name: "b.json", Features: []input.Feature{g}}})
	if err == nil {
		t.Fatal("collision accepted")
	}
	last := -1
	for _, line := range []string{":9 ", ":10 ", ":100 ", ":1000 ", ":10000 "} {
		at := strings.Index(err.Error(), line)
		if at <= last {
			t.Fatalf("not in numeric line order (%s): %v", line, err)
		}
		last = at
	}
	long := input.Feature{URI: "features/" + strings.Repeat("very_long_directory_name_", 8) + "/checkout.feature", Elements: []input.Scenario{{Name: "s", Line: 42}}}
	other := long
	other.Elements = append(other.Elements, input.Scenario{Name: "t", Line: 50})
	err = rejectRepeatedInputs([]runInput{{Name: "a/r.json", Features: []input.Feature{long}}, {Name: "b/r.json", Features: []input.Feature{other}}})
	if err == nil || !strings.Contains(err.Error(), `…":42 in input[0] "a/r.json", input[1] "b/r.json"`) {
		t.Fatalf("long URI truncated away the line or inputs: %v", err)
	}
	err = rejectRepeatedInputs([]runInput{{Name: "a/r.json", Features: []input.Feature{long}}, {Name: "b/r.json", Features: []input.Feature{long}}})
	if err == nil || !strings.Contains(err.Error(), `input[1] "b/r.json" has the same content as input[0] "a/r.json"`) {
		t.Fatalf("identical inputs not reported as such: %v", err)
	}
}
