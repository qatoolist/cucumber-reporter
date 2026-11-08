package report_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

func round15Copy[T any](t *testing.T, v T) T {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var copy T
	if err := json.Unmarshal(b, &copy); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v, copy) {
		t.Fatal("JSON copy changed public input values")
	}
	return copy
}

func round15Features() []cucumber.Feature {
	return []cucumber.Feature{{URI: "a.feature", Name: "Original", Elements: []cucumber.Scenario{{Name: "original", Type: "scenario", Line: 1, Steps: []cucumber.Step{{Name: "ok", Result: cucumber.Result{Status: cucumber.StatusPassed}}}}}}}
}

func TestRound15PublicCopiesKeepEvidence(t *testing.T) {
	for _, file := range []string{
		"../testdata/captured/ruby-9.2.1-r12/retryfinal/run.json",
		"../testdata/captured/ruby-9.2.1-r12/retryfinal/run.ndjson",
		"../testdata/round15/captured/g3-same-strict/run.ndjson",
	} {
		t.Run(file, func(t *testing.T) {
			raw, readErr := os.ReadFile(file)
			if readErr != nil {
				t.Fatal(readErr)
			}
			in := report.Input{Name: "one"}
			if filepath.Ext(file) == ".ndjson" {
				r, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				in.Attempts = r.Attempts
			} else {
				var err error
				in.Features, err = cucumber.Parse(bytes.NewReader(raw), cucumber.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				original, renderErr := report.Render(in.Features, report.Options{Version: 2})
				if renderErr != nil {
					t.Fatal(renderErr)
				}
				copied, err := report.Render(round15Copy(t, in.Features), report.Options{Version: 2})
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(original, copied) {
					t.Fatal("equal parsed feature copies rendered differently")
				}
			}
			cfg := report.RunConfig{ID: "value-copy", RecordedAt: "2026-11-07T00:00:00Z"}
			original, err := report.PrepareRun([]report.Input{in}, report.Options{Version: 2}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			copied, err := report.PrepareRun([]report.Input{round15Copy(t, in)}, report.Options{Version: 2}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			outcomes, outcomeErr := report.ScenarioOutcomes(original.Features, nil)
			if outcomeErr != nil {
				t.Fatal(outcomeErr)
			}
			index := 0
			for _, row := range outcomes {
				for _, outcome := range row {
					if index >= len(original.Summary.Scenarios) || outcome != original.Summary.Scenarios[index].Status {
						t.Fatalf("live prepared slot %d became %s", index, outcome)
					}
					index++
				}
			}
			if !original.Equal(*copied) {
				t.Fatal("equal public inputs prepared differently")
			}
			a, err := report.Render(original.Features, original.Options)
			if err != nil {
				t.Fatal(err)
			}
			b, err := report.Render(copied.Features, copied.Options)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a, b) {
				t.Fatal("equal prepared snapshots rendered differently")
			}
		})
	}
}

func TestRound15PreparedDirectoryOptionsRefused(t *testing.T) {
	p, err := report.PrepareRun([]report.Input{{Name: "one", Features: round15Features()}}, report.Options{Version: 2}, report.RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "cucumber.json"), []byte(`[{"uri":"other.feature","name":"OTHER","elements":[{"name":"OTHER scenario","line":9,"type":"scenario","steps":[{"name":"ok","result":{"status":"passed"}}]}]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, render := range []func() (string, error){func() (string, error) { return report.RenderDir(dir, p.Options) }, func() (string, error) { return report.RenderDirTo(dir, t.TempDir(), p.Options) }} {
		out, renderErr := render()
		if out != "" || !errors.Is(renderErr, report.ErrInvalidOptions) {
			t.Fatalf("prepared options accepted by directory API: %q %v", out, renderErr)
		}
	}
	p.Features[0].Elements[0].Name = "changed"
	out, err := report.WriteRunDir(p, t.TempDir())
	if out != "" || !errors.Is(err, report.ErrInvalidOptions) {
		t.Fatalf("modified snapshot published: %q %v", out, err)
	}
}

func TestRound15PublicErrorCategories(t *testing.T) {
	f := round15Features()
	reserved := round15Copy(t, f)
	reserved[0].URI = "__run_hooks__/user.feature"
	for _, tc := range []struct {
		name   string
		inputs []report.Input
		cfg    report.RunConfig
		want   error
	}{
		{"missing name", []report.Input{{Features: f}}, report.RunConfig{}, report.ErrInvalidInput},
		{"reserved URI", []report.Input{{Name: "one", Features: reserved}}, report.RunConfig{}, report.ErrInvalidInput},
		{"repeated inputs", []report.Input{{Name: "one", Features: f}, {Name: "two", Features: f}}, report.RunConfig{}, report.ErrInvalidInput},
		{"duplicate expectations", []report.Input{{Name: "one", Features: f}}, report.RunConfig{ExpectedShards: []string{"one", "one"}}, report.ErrInvalidOptions},
		{"unsupported baseline", []report.Input{{Name: "one", Features: f}}, report.RunConfig{Baseline: &report.RunSummary{Version: report.SummaryVersion + 1}}, report.ErrInvalidOptions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := report.PrepareRun(tc.inputs, report.Options{Version: 2}, tc.cfg)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v does not wrap %v", err, tc.want)
			}
		})
	}
}

func TestRound15OptionsJSONCannotLoadComputedState(t *testing.T) {
	var opt report.Options
	if err := json.Unmarshal([]byte(`{"version":2,"analysis":{"computed":{"tone":"ok","label":"FAKE"}},"bundleAttachments":true,"execution":{"fake":{}},"preparedDigest":"fake"}`), &opt); err != nil {
		t.Fatal(err)
	}
	if opt.Analysis != nil || opt.BundleAttachments || opt.Execution != nil || opt.PreparedDigest != "" {
		t.Fatalf("computed fields loaded from JSON: %+v", opt)
	}
}

func TestRound16PreparedStateCannotBeForged(t *testing.T) {
	f := []cucumber.Feature{{URI: "one.feature", Name: "One", Elements: []cucumber.Scenario{{Name: "one", Line: 1, Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusFailed}}}}}}}
	p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: 2}, report.RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	for _, alter := range []func(*report.Options){
		func(o *report.Options) { o.PreparedDigest = "" },
		func(o *report.Options) {
			o.Analysis = &report.RunSummary{Computed: report.Verdict{Label: "Passed", Tone: report.ToneOK}}
		},
		func(o *report.Options) { o.Execution = nil },
	} {
		o := p.Options
		alter(&o)
		if _, renderErr := report.Render(p.Features, o); !errors.Is(renderErr, report.ErrInvalidOptions) {
			t.Fatalf("forged render: %v", renderErr)
		}
	}
	p.Summary.Computed.Label = "Passed"
	if _, renderErr := report.Render(p.Features, p.Options); !errors.Is(renderErr, report.ErrInvalidOptions) {
		t.Fatalf("edited summary: %v", renderErr)
	}
	p, err = report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: 2}, report.RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	p.Metadata = nil
	if _, renderErr := report.WriteRunDir(p, t.TempDir()); !errors.Is(renderErr, report.ErrInvalidOptions) {
		t.Fatalf("edited metadata: %v", renderErr)
	}
}
