package report_test

import (
	"os"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

// round20Attempts builds two failed attempts of one scenario whose only step has
// the given keyword and name in each attempt.
func round20Attempts(keywords, names [2]string) []report.Input {
	var attempts []cucumber.MessageAttempt
	for i := range names {
		attempts = append(attempts, cucumber.MessageAttempt{URI: "a.feature", FeatureName: "A", Number: i, Scenario: cucumber.Scenario{Type: "scenario", Name: "S", Line: 2, Steps: []cucumber.Step{{Keyword: keywords[i], Name: names[i], Result: cucumber.Result{Status: cucumber.StatusFailed, Error: "BOOM"}}}}})
	}
	return []report.Input{{Name: "one", Attempts: attempts}}
}

// R19 dev review M: masking must not make an earlier attempt's different step
// look like the covered final step. Masked and unmasked runs give the same report.
func TestRound20RedactionKeepsRetryCoverage(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []report.Input
		tone string
	}{
		"names differ":    {round20Attempts([2]string{"Given ", "Given "}, [2]string{"check secret-original", "check secret-different"}), report.ToneFail},
		"keywords differ": {round20Attempts([2]string{"Given secret-a ", "Given secret-b "}, [2]string{"check secret-different", "check secret-different"}), report.ToneFail},
		"same step":       {round20Attempts([2]string{"Given ", "Given "}, [2]string{"check secret-different", "check secret-different"}), report.ToneWarn},
	} {
		first, err := report.PrepareRun(tc.in, report.Options{Version: report.OptionsVersion}, report.RunConfig{})
		if err != nil {
			t.Fatal(err)
		}
		opt := report.Options{Version: report.OptionsVersion, Annotations: []report.Annotation{{Scenario: first.Summary.Scenarios[0].ID, Kind: report.KindKnown, Title: "known", Step: 1, StepText: "check secret-different"}}}
		for _, redact := range []bool{false, true} {
			cfg := report.RunConfig{Gates: report.GatePolicy{Unexpected: true}}
			if redact {
				cfg.Redaction = &report.Redaction{Patterns: []string{`secret-\w+`}}
			}
			p, err := report.PrepareRun(tc.in, opt, cfg)
			if err != nil {
				t.Fatalf("%s redact=%v: %v", name, redact, err)
			}
			if p.Summary.Computed.Tone != tc.tone || (tc.tone == report.ToneFail) != (len(p.Summary.GateFailures) > 0) {
				t.Fatalf("%s redact=%v: verdict %+v gates %v", name, redact, p.Summary.Computed, p.Summary.GateFailures)
			}
			if _, err = report.Render(p.Features, p.Options); err != nil {
				t.Fatalf("%s redact=%v: render: %v", name, redact, err)
			}
			if redact {
				round19AssertNotPublished(t, p, "secret-")
			}
		}
	}
}

// The dev's JSON fixture: one input with an in-input retry whose step text differs.
func TestRound20RedactionKeepsRetryCoverageJSON(t *testing.T) {
	raw, err := os.ReadFile("../testdata/round20/retry-names.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cucumber.Parse(strings.NewReader(string(raw)), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	opt := report.Options{Version: report.OptionsVersion, Annotations: []report.Annotation{{Scenario: "r-d23e6f3ab2b8-L3", Kind: report.KindKnown, Title: "known", Step: 1, StepText: "check secret-different"}}}
	for _, redact := range []bool{false, true} {
		cfg := report.RunConfig{Gates: report.GatePolicy{Unexpected: true}}
		if redact {
			cfg.Redaction = &report.Redaction{Patterns: []string{`secret-\w+`}}
		}
		p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, opt, cfg)
		if err != nil {
			t.Fatalf("redact=%v: %v", redact, err)
		}
		if p.Summary.Computed.Tone != report.ToneFail {
			t.Fatalf("redact=%v: verdict %+v", redact, p.Summary.Computed)
		}
		if _, err = report.Render(p.Features, p.Options); err != nil {
			t.Fatalf("redact=%v: render: %v", redact, err)
		}
	}
}

// round20Run prepares two attempts with the given steps, a known issue at the
// declared position, and optional redaction.
func round20Run(t *testing.T, steps [2][]cucumber.Step, known report.Annotation, redact bool) *report.PreparedRun {
	t.Helper()
	var attempts []cucumber.MessageAttempt
	for i := range steps {
		attempts = append(attempts, cucumber.MessageAttempt{URI: "a.feature", FeatureName: "A", Number: i, Scenario: cucumber.Scenario{Type: "scenario", Name: "S", Line: 2, Steps: steps[i]}})
	}
	in := []report.Input{{Name: "one", Attempts: attempts}}
	first, err := report.PrepareRun(in, report.Options{Version: report.OptionsVersion}, report.RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	known.Scenario, known.Kind, known.Title = first.Summary.Scenarios[0].ID, report.KindKnown, "known"
	cfg := report.RunConfig{Gates: report.GatePolicy{Unexpected: true}}
	if redact {
		cfg.Redaction = &report.Redaction{Patterns: []string{`secret-\w+`}}
	}
	p, err := report.PrepareRun(in, report.Options{Version: report.OptionsVersion, Annotations: []report.Annotation{known}}, cfg)
	if err != nil {
		t.Fatalf("redact=%v: %v", redact, err)
	}
	if _, err = report.Render(p.Features, p.Options); err != nil {
		t.Fatalf("redact=%v: render: %v", redact, err)
	}
	return p
}

func round20Step(keyword, name string, status cucumber.Status) cucumber.Step {
	return cucumber.Step{Keyword: keyword, Name: name, Result: cucumber.Result{Status: status, Error: "BOOM"}}
}

// R20 review L2: pairing is by position. An earlier step equal to its own final
// step stays covered, although it differs from another final step that masks alike.
func TestRound20RetryPairingIsPositional(t *testing.T) {
	attempt := []cucumber.Step{round20Step("Given ", "check secret-a", cucumber.StatusFailed), round20Step("Given ", "check secret-b", cucumber.StatusSkipped)}
	for _, redact := range []bool{false, true} {
		p := round20Run(t, [2][]cucumber.Step{attempt, attempt}, report.Annotation{Step: 1, StepText: "check secret-a"}, redact)
		if p.Summary.Computed.Tone != report.ToneWarn {
			t.Fatalf("redact=%v: same-position retry lost coverage: %+v", redact, p.Summary.Computed)
		}
	}
}

// R20 review L1/L5: only a declared step that differed before masking is marked;
// hooks and steps still distinct after masking (keyword) are left unchanged.
func TestRound20RetryMarkerOnlyWhereMaskingMerged(t *testing.T) {
	hook := func(name string) cucumber.Step {
		s := round20Step("", name, cucumber.StatusPassed)
		s.Hidden = true
		return s
	}
	for name, steps := range map[string][2][]cucumber.Step{
		"hooks":   {{hook("Before secret-a"), hook("Before secret-b"), round20Step("Given ", "check x", cucumber.StatusFailed)}, {hook("Before secret-a"), hook("Before secret-b"), round20Step("Given ", "check x", cucumber.StatusFailed)}},
		"keyword": {{round20Step("When ", "check secret-x", cucumber.StatusFailed)}, {round20Step("Given ", "check secret-x", cucumber.StatusFailed)}},
	} {
		text := "check x"
		if name == "keyword" {
			text = "check secret-x"
		}
		p := round20Run(t, steps, report.Annotation{Step: 1, StepText: text}, true)
		html, err := report.Render(p.Features, p.Options)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(html), "differs before redaction") {
			t.Fatalf("%s: marker added where masking merged nothing", name)
		}
	}
}

// R20 review M1: a baseline match on a generated identity of a masked name is
// masked too; the run says so.
func TestRound20MaskedIdentityComparisonIsDiagnosed(t *testing.T) {
	run := func(name string, cfg report.RunConfig) *report.PreparedRun {
		f, err := cucumber.Parse(strings.NewReader(`[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","name":"`+name+`","line":2,"steps":[{"name":"step","result":{"status":"failed","error_message":"BOOM"}}]}]}]`), cucumber.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: report.OptionsVersion}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, redact := range []bool{true, false} {
		cfg := report.RunConfig{}
		if redact {
			cfg.Redaction = &report.Redaction{Patterns: []string{`secret-\w+`}}
		}
		base := run("login secret-old", cfg)
		cfg.Baseline, cfg.Gates = &base.Summary, report.GatePolicy{NewFailures: true}
		p := run("login secret-new", cfg)
		if got := strings.Contains(diagnosticsText(p), "identities were changed by redaction"); got != redact {
			t.Fatalf("redact=%v: identity diagnostic %v: %s", redact, got, diagnosticsText(p))
		}
	}
	// Explicit metadata identities that a pattern matches merge the same way.
	f, err := cucumber.Parse(strings.NewReader(`[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","name":"login","line":2,"steps":[{"name":"step","result":{"status":"failed","error_message":"BOOM"}}]}]}]`), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	id := report.RunScenarioIDs(f)[0][0]
	for _, tc := range []struct {
		base, current string
		diagnosed     bool
	}{{"id-secret-1", "id-secret-2", true}, {"id-1", "id-2", false}} {
		cfg := report.RunConfig{Redaction: &report.Redaction{Patterns: []string{`secret-\w+`}}, Metadata: map[string]report.ScenarioMeta{id: {Identity: tc.base}}}
		base, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: report.OptionsVersion}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Metadata = map[string]report.ScenarioMeta{id: {Identity: tc.current}}
		cfg.Baseline, cfg.Gates = &base.Summary, report.GatePolicy{NewFailures: true}
		p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: report.OptionsVersion}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(diagnosticsText(p), "identities were changed by redaction"); got != tc.diagnosed {
			t.Fatalf("explicit %+v: identity diagnostic %v: %s", tc, got, diagnosticsText(p))
		}
	}
}

// R20 review L2: background steps pair by their background ordinal.
func TestRound20RetryPairingKeepsBackgroundOrdinal(t *testing.T) {
	// A captured Ruby background retry (round13 retry-background, always failing),
	// with a passing first background step added.
	raw, err := os.ReadFile("../testdata/round20/retry-background.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cucumber.Parse(strings.NewReader(string(raw)), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: report.OptionsVersion}, report.RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	opt := report.Options{Version: report.OptionsVersion, Annotations: []report.Annotation{{Scenario: first.Summary.Scenarios[0].ID, Kind: report.KindKnown, Title: "known", BackgroundStep: 2, StepText: "setup secret-b"}}}
	for _, redact := range []bool{false, true} {
		cfg := report.RunConfig{Gates: report.GatePolicy{Unexpected: true}}
		if redact {
			cfg.Redaction = &report.Redaction{Patterns: []string{`secret-\w+`}}
		}
		p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, opt, cfg)
		if err != nil {
			t.Fatalf("redact=%v: %v", redact, err)
		}
		if p.Summary.Computed.Tone != report.ToneWarn {
			t.Fatalf("redact=%v: background retry lost coverage: %+v", redact, p.Summary.Computed)
		}
	}
}
