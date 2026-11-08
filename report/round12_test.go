package report

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRound12NeverGreen(t *testing.T) {
	for _, name := range []string{"orphan.json", "orphan_only_feature.json", "longs-duplicate-status.json"} {
		raw, err := os.ReadFile("../testdata/round12/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, enhanced := range []bool{false, true} {
			f, err := cucumber.ParseEnhanced(bytes.NewReader(raw), cucumber.Limits{})
			if err != nil {
				continue
			}
			opt := renderOptions{Version: 2}
			if enhanced {
				p, e := preparePublicRun([]runInput{{Name: "input", Features: f}}, opt, RunConfig{})
				if e != nil {
					continue
				}
				f, opt = p.Features, p.Options
			}
			html, err := renderPublicModel(f, opt)
			if err != nil {
				continue
			}
			if strings.Contains(string(html), `"label":"Passed`) {
				t.Errorf("%s enhanced=%v became green", name, enhanced)
			}
		}
	}
}

func TestRound12CapturedRunnerFailures(t *testing.T) {
	dirs, err := filepath.Glob("../testdata/captured/ruby-9.2.1-r12/*/run.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) < 16 {
		t.Fatalf("missing captures: %d", len(dirs))
	}
	for _, file := range dirs {
		t.Run(filepath.Base(filepath.Dir(file)), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			run, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			p, err := preparePublicRun([]runInput{{Name: "capture", Attempts: run.Attempts}}, renderOptions{Version: 2}, RunConfig{})
			if err != nil {
				t.Fatal(err)
			}
			html, err := renderPublicModel(p.Features, p.Options)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte(`"success":false`)) && strings.Contains(string(html), `"label":"Passed`) {
				t.Fatal("runner failure became green")
			}
		})
	}
}

func TestRound12CapturedFormatAgreement(t *testing.T) {
	expected := map[string]map[string]int{
		"attach": {"failed": 1, "passed": 1}, "background": {"failed": 2}, "hooks": {"failed": 3, "passed": 1}, "shapes": {"failed": 2, "passed": 4},
		"flakyonly": {"passed": 2}, "flakyonly-strict": {"passed": 2}, "retryfinal": {"failed": 2, "passed": 1},
		"pendonly": {"pending": 1, "passed": 1}, "pendonly-strict": {"pending": 1, "passed": 1}, "skippass": {"skipped": 1, "passed": 1},
	}
	for _, name := range []string{"pendpass-default", "pendpass-nostrict", "pendpass-strict"} {
		expected[name] = map[string]int{"undefined": 1, "pending": 1, "passed": 1}
	}
	for _, name := range []string{"statuses", "statuses-nostrict", "statuses-strict"} {
		expected[name] = map[string]int{"failed": 2, "undefined": 1, "pending": 1, "passed": 1}
	}
	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			var summaries []RunSummary
			for _, format := range []string{"json", "ndjson"} {
				raw, err := os.ReadFile("../testdata/captured/ruby-9.2.1-r12/" + name + "/run." + format)
				if err != nil {
					t.Fatal(err)
				}
				in := runInput{Name: "capture"}
				if format == "json" {
					in.Features, err = cucumber.ParseEnhanced(bytes.NewReader(raw), cucumber.Limits{})
					if err != nil {
						t.Fatal(err)
					}
					var plain []cucumber.Feature
					plain, err = cucumber.Parse(bytes.NewReader(raw), cucumber.Limits{})
					if err != nil {
						t.Fatal(err)
					}
					opt, _ := normalize(renderOptions{Version: 2})
					var page *pageData
					page, err = buildPage(plain, opt, nil)
					if err != nil {
						t.Fatal(err)
					}
					got := map[string]int{}
					for _, f := range page.Features {
						for _, s := range f.Elements {
							got[s.Status]++
						}
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("plain: %v want %v", got, want)
					}
				} else {
					m, e := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
					if e != nil {
						t.Fatal(e)
					}
					in.Attempts = m.Attempts
				}
				p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{})
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]int{}
				for status, n := range p.Summary.Counts {
					if n > 0 && status != "flaky" {
						got[status] = n
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: %v want %v", format, got, want)
				}
				summaries = append(summaries, p.Summary)
			}
			a, b := summaries[0], summaries[1]
			if len(a.Scenarios) != len(b.Scenarios) {
				t.Fatal("format scenario counts differ")
			}
			byIdentity := map[string]ScenarioSummary{}
			for _, s := range b.Scenarios {
				byIdentity[s.Identity] = s
			}
			for _, s := range a.Scenarios {
				other, ok := byIdentity[s.Identity]
				if !ok || s.Status != other.Status {
					t.Fatalf("identity/outcome differs: %s", s.Name)
				}
			}
		})
	}
}
