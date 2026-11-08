package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRound11SteplessBackground(t *testing.T) {
	for _, name := range []string{"stepless.json", "stepless-hook.json"} {
		b, err := os.ReadFile(filepath.Join("../testdata/plain-compat/bgh", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, enhanced := range []bool{false, true} {
			parse := cucumber.Parse
			if enhanced {
				parse = cucumber.ParseEnhanced
			}
			f, err := parse(bytes.NewReader(b), cucumber.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if enhanced {
				p, err := preparePublicRun([]runInput{{Name: "input", Features: f}}, renderOptions{Version: 2}, RunConfig{})
				if err != nil {
					t.Fatal(err)
				}
				f = p.Features
			}
			if got := f[0].Elements[0].Status(); got != cucumber.StatusUndefined {
				t.Errorf("%s enhanced=%v: %s, want undefined", name, enhanced, got)
			}
		}
	}
}

func TestRound11InlineAnnotationStable(t *testing.T) {
	files, err := filepath.Glob("../testdata/plain-compat/js/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) == 0 || b[0] != '[' {
			continue
		}
		f, err := cucumber.Parse(bytes.NewReader(b), cucumber.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "f-L5", Kind: "known", Ref: "EX-1", Title: "t", Step: 2, StepText: "x"}}}
		if _, err := renderPublicModel(f, opt); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
}

func TestRound11OrphanBackgroundDiagnostic(t *testing.T) {
	raw := `[{"uri":"orphan.feature","elements":[{"type":"background","steps":[{"result":{"status":"passed"}}]}]}]`
	f, err := cucumber.ParseEnhanced(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := preparePublicRun([]runInput{{Name: "input", Features: f}}, renderOptions{Version: 2}, RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.ObservedScenarios != 0 || len(p.Summary.Diagnostics) == 0 {
		t.Fatalf("missing diagnostic: %+v", p.Summary)
	}
	data, err := buildPage(p.Features, p.Options, nil)
	if err != nil || data.Verdict.Label == "Passed" {
		t.Fatalf("orphan became passed: %+v %v", data, err)
	}
	if _, err = renderPublicModel(p.Features, p.Options); err != nil {
		t.Fatal(err)
	}
}

func TestRound11BackgroundSlotsAndHiddenPassingHooks(t *testing.T) {
	raw := `[{"uri":"a.feature","elements":[{"type":"background","steps":[{"result":{"status":"passed"}}]},{"name":"s1","steps":[{"name":"own","result":{"status":"passed"}}],"after":[{"result":{"status":"passed","duration":98765},"embeddings":[{"mime_type":"image/png","data":"aGk="}]}]},{"name":"s2","steps":[{"result":{"status":"passed"}}]}]}]`
	f, err := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	opt, _ := normalize(renderOptions{Version: 2})
	data, err := buildPage(f, opt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if data.Features[0].Elements[0].ID != "a-L0-2" || data.Features[0].Elements[1].ID != "a-L0-3" {
		t.Fatalf("shifted IDs: %+v", data.Features[0].Elements)
	}
	sc := data.Features[0].Elements[0]
	if len(sc.Steps) != 2 || sc.Duration != 0 {
		t.Fatalf("passing hook visible: %+v", sc)
	}
	opt.Annotations = []Annotation{{Scenario: "a-L0", Kind: "known", Ref: "EX-1"}}
	if _, err := renderPublicModel(f, opt); err == nil || !strings.Contains(err.Error(), "annotate a-L0-2") {
		t.Fatalf("missing redirect: %v", err)
	}
}

func TestLargeFallbackRetainsProblemsAndCompleteModel(t *testing.T) {
	features := []featureData{{Elements: make([]scenarioData, 1200)}}
	for i := range features[0].Elements {
		features[0].Elements[i].Status = "passed"
	}
	features[0].Elements[1199].Status = "failed"
	reduced := fallbackFeatures(features)
	if len(reduced[0].Elements) != 1001 || reduced[0].Elements[1000].Status != "failed" {
		t.Fatal("problem detail dropped or fallback unbounded")
	}
	if len(features[0].Elements) != 1200 {
		t.Fatal("full interactive model mutated")
	}
}
