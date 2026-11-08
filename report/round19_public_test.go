package report_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

const round19KnownJSON = `[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","name":"S","line":2,"steps":[{"name":"check secret-original","result":{"status":"failed","error_message":"BOOM SECRET123"}}]}]}]`

func round19Prepare(t *testing.T, in report.Input, stepText string, cfg report.RunConfig) (*report.PreparedRun, error) {
	t.Helper()
	var id string
	if in.Features != nil {
		id = report.RunScenarioIDs(in.Features)[0][0]
	} else {
		id = "a-a8fde43a2452-L9" // testdata/round19/known-redaction.ndjson, scenario at line 9
	}
	opt := report.Options{Version: report.OptionsVersion, Annotations: []report.Annotation{{Scenario: id, Kind: report.KindKnown, Ref: "K", Title: "known", Step: 1, StepText: stepText}}}
	return report.PrepareRun([]report.Input{in}, opt, cfg)
}

func round19Inputs(t *testing.T) map[string]report.Input {
	t.Helper()
	f, err := cucumber.Parse(strings.NewReader(round19KnownJSON), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../testdata/round19/known-redaction.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	mr, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]report.Input{"json": {Name: "one", Features: f}, "messages": {Name: "one", Attempts: mr.Attempts}}
}

// R19-H1: redaction that makes a mismatched stepText equal to the executed step
// must not let the annotation cover the failure, in either input format.
func TestRound19RedactionCannotWidenKnownCoverage(t *testing.T) {
	for format, in := range round19Inputs(t) {
		for _, tc := range []struct {
			stepText string
			redact   bool
			tone     string
		}{
			{"check secret-different", false, report.ToneFail},
			{"check secret-different", true, report.ToneFail},
			{"check secret-original", false, report.ToneWarn},
			{"check secret-original", true, report.ToneWarn},
		} {
			cfg := report.RunConfig{Gates: report.GatePolicy{Unexpected: true}}
			if tc.redact {
				cfg.Redaction = &report.Redaction{Patterns: []string{`secret-\w+`}}
			}
			p, err := round19Prepare(t, in, tc.stepText, cfg)
			if err != nil {
				t.Fatalf("%s %+v: %v", format, tc, err)
			}
			if p.Summary.Computed.Tone != tc.tone || (tc.tone == report.ToneFail) != (len(p.Summary.GateFailures) > 0) {
				t.Fatalf("%s %+v: verdict %+v gates %v", format, tc, p.Summary.Computed, p.Summary.GateFailures)
			}
			html, err := report.Render(p.Features, p.Options)
			if err != nil {
				t.Fatalf("%s %+v: render: %v", format, tc, err)
			}
			if tc.redact && bytes.Contains(html, []byte("secret-")) {
				t.Fatalf("%s %+v: redacted text published", format, tc)
			}
			if tc.tone == report.ToneFail && !strings.Contains(diagnosticsText(p), "does not match its declared step") {
				t.Fatalf("%s %+v: mismatch diagnostic missing: %s", format, tc, diagnosticsText(p))
			}
		}
	}
}

// R19-M1: issue policies are evaluated on the original error both by the decision
// and by the independent floor; masking can neither reject nor admit a failure.
func TestRound19IssuePolicyUsesOriginalEvidence(t *testing.T) {
	f := round19Inputs(t)["json"]
	for _, tc := range []struct {
		pattern string
		redact  bool
		tone    string
	}{
		{`^BOOM SECRET123$`, false, report.ToneWarn},
		{`^BOOM SECRET123$`, true, report.ToneWarn},
		{`^BOOM OTHER$`, true, report.ToneFail},
		{`^BOOM \[REDACTED\]$`, true, report.ToneFail},
	} {
		cfg := report.RunConfig{Gates: report.GatePolicy{Unexpected: true}, Issues: []report.IssuePolicy{{Ref: "K", ErrorPattern: tc.pattern}}}
		if tc.redact {
			cfg.Redaction = &report.Redaction{Patterns: []string{"SECRET123"}}
		}
		p, err := round19Prepare(t, f, "check secret-original", cfg)
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		if p.Summary.Computed.Tone != tc.tone {
			t.Fatalf("%+v: verdict %+v", tc, p.Summary.Computed)
		}
		if _, err = report.Render(p.Features, p.Options); err != nil {
			t.Fatalf("%+v: render: %v", tc, err)
		}
	}
}

// R19-M2: caller diagnostics pass through redaction before any export.
func TestRound19InputDiagnosticsAreRedacted(t *testing.T) {
	in := round19Inputs(t)["json"]
	in.Diagnostics = []report.Diagnostic{{Path: "input[0].SECRET123", Message: "warning SECRET123"}}
	p, err := report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Bundle: true, Redaction: &report.Redaction{Patterns: []string{"SECRET123"}}})
	if err != nil {
		t.Fatal(err)
	}
	if s := diagnosticsText(p); strings.Contains(s, "SECRET123") || !strings.Contains(s, "warning [REDACTED]") {
		t.Fatalf("diagnostics not redacted: %s", s)
	}
	round19AssertNotPublished(t, p, "SECRET123")
}

// round19AssertNotPublished checks the HTML and every file of the run directory,
// including the entries of report.zip, for secret.
func round19AssertNotPublished(t *testing.T, p *report.PreparedRun, secret string) {
	t.Helper()
	html, err := report.Render(p.Features, p.Options)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(html, []byte(secret)) {
		t.Fatal("secret in HTML")
	}
	dir, err := report.WriteRunDir(p, t.TempDir())
	if err != nil && !errors.Is(err, report.ErrGateFailed) {
		t.Fatal(err)
	}
	checked := 0
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		if bytes.Contains(b, []byte(secret)) {
			t.Errorf("secret in %s", filepath.Base(path))
		}
		if strings.HasSuffix(path, ".zip") {
			z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
			if err != nil {
				return err
			}
			for _, zf := range z.File {
				r, err := zf.Open()
				if err != nil {
					return err
				}
				body, err := io.ReadAll(r)
				r.Close()
				if err != nil {
					return err
				}
				checked++
				if bytes.Contains(body, []byte(secret)) {
					t.Errorf("secret in %s/%s", filepath.Base(path), zf.Name)
				}
			}
		}
		return nil
	})
	if err != nil || checked < 3 {
		t.Fatalf("walk: %v checked=%d", err, checked)
	}
}

func diagnosticsText(p *report.PreparedRun) string {
	b, _ := json.Marshal(p.Summary.Diagnostics)
	return string(b)
}

// R19-H1 (environment variant): two different environments that redaction masks to
// the same text must not make an environment-scoped issue policy apply.
func TestRound19RedactionCannotMatchIssueEnvironment(t *testing.T) {
	f := round19Inputs(t)["json"]
	for _, tc := range []struct {
		env    string
		redact bool
		tone   string
	}{
		{"env-SECRET2", false, report.ToneWarn},
		{"env-SECRET2", true, report.ToneWarn},
		{"env-SECRET9", false, report.ToneFail},
		{"env-SECRET9", true, report.ToneFail},
	} {
		cfg := report.RunConfig{Environment: tc.env, Gates: report.GatePolicy{Unexpected: true}, Issues: []report.IssuePolicy{{Ref: "K", Environments: []string{"env-SECRET2"}}}}
		if tc.redact {
			cfg.Redaction = &report.Redaction{Patterns: []string{`SECRET\d`}}
		}
		p, err := round19Prepare(t, f, "check secret-original", cfg)
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		if p.Summary.Computed.Tone != tc.tone {
			t.Fatalf("%+v: verdict %+v", tc, p.Summary.Computed)
		}
		if b, _ := json.Marshal(p.Summary); tc.redact && bytes.Contains(b, []byte("SECRET")) {
			t.Fatalf("%+v: environment secret published", tc)
		}
		if _, err = report.Render(p.Features, p.Options); err != nil {
			t.Fatalf("%+v: render: %v", tc, err)
		}
	}
}

// R19 review H: a secret split by SGR coloring is masked, although Display
// output deletes the coloring and would rejoin it.
func TestRound19SGRSplitSecretIsRedacted(t *testing.T) {
	raw := `[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","name":"S TOK\u001b[1m123","line":2,"steps":[{"name":"step TOK\u001b[0m123","result":{"status":"failed","error_message":"BOOM TOK\u001b[31m123\u001b[0m"}}]}]}]`
	f, err := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Bundle: true, Redaction: &report.Redaction{Patterns: []string{"TOK123"}}})
	if err != nil {
		t.Fatal(err)
	}
	round19AssertNotPublished(t, p, "TOK123")
	if b, _ := json.Marshal(p.Summary); !bytes.Contains(b, []byte(`BOOM [REDACTED]`)) {
		t.Fatalf("error not masked: %s", b)
	}
}

// R19 review M: masking must not narrow coverage either. The step-text decision
// is taken on unmasked values and carried; the display-escape case (a BEL control
// against a literal \u0007) is independent of the SGR rule.
func TestRound19RedactionCannotNarrowKnownCoverage(t *testing.T) {
	for _, tc := range []struct{ name, stepText, pattern string }{
		// name and stepText are JSON string bodies.
		{`check secret-orig\u001b[0minal`, `check secret-original`, `secret-\w+`},
		{`check secret-original`, `check secret-orig\u001b[0minal`, `secret-\w+`},
		{`check a\u0007b`, `check a\\u0007b`, `\\u0007`},
	} {
		raw := `[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","name":"S","line":2,"steps":[{"name":"` + tc.name + `","result":{"status":"failed","error_message":"BOOM"}}]}]}]`
		f, err := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var stepText string
		if err = json.Unmarshal([]byte(`"`+tc.stepText+`"`), &stepText); err != nil {
			t.Fatal(err)
		}
		for _, redact := range []bool{false, true} {
			cfg := report.RunConfig{Gates: report.GatePolicy{Unexpected: true}}
			if redact {
				cfg.Redaction = &report.Redaction{Patterns: []string{tc.pattern}}
			}
			p, err := round19Prepare(t, report.Input{Name: "one", Features: f}, stepText, cfg)
			if err != nil {
				t.Fatalf("%+v redact=%v: %v", tc, redact, err)
			}
			if p.Summary.Computed.Tone != report.ToneWarn {
				t.Fatalf("%+v redact=%v: matching step text lost coverage: %+v", tc, redact, p.Summary.Computed)
			}
			if _, err = report.Render(p.Features, p.Options); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// R19 review L1: a collapsed mismatch keeps the same issue state as without
// redaction, so lifecycle records do not depend on masking.
func TestRound19CollapsedMismatchStaysInvalid(t *testing.T) {
	in := round19Inputs(t)["json"]
	var states []string
	for _, redact := range []bool{false, true} {
		cfg := report.RunConfig{}
		if redact {
			cfg.Redaction = &report.Redaction{Patterns: []string{`secret-\w+`}}
		}
		p, err := round19Prepare(t, in, "check secret-different", cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Summary.IssueRecords) != 1 {
			t.Fatalf("issue records: %+v", p.Summary.IssueRecords)
		}
		states = append(states, p.Summary.IssueRecords[0].State)
	}
	if states[0] != states[1] || states[0] == "" {
		t.Fatalf("issue state depends on redaction: %v", states)
	}
}

// R19 review L2: stored summaries are masked, so scope comparison under redaction
// is on masked values; the run says so.
func TestRound19MaskedScopeComparisonIsDiagnosed(t *testing.T) {
	in := round19Inputs(t)["json"]
	cfg := report.RunConfig{Environment: "env-SECRET1", Redaction: &report.Redaction{Patterns: []string{`SECRET\d`}}}
	first, err := report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, redact := range []bool{true, false} {
		next := cfg
		next.Baseline = &first.Summary
		if !redact {
			next.Redaction, next.Environment = nil, first.Summary.Environment
		}
		p, err := report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, next)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(diagnosticsText(p), "compared on the redacted environment"); got != redact {
			t.Fatalf("redact=%v: scope diagnostic %v: %s", redact, got, diagnosticsText(p))
		}
	}
}

// R19 re-review L3: step keywords are display text and are masked too.
func TestRound19StepKeywordIsRedacted(t *testing.T) {
	raw := `[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","keyword":"Scenario TOK123","name":"S","line":2,"steps":[{"keyword":"Given TOK123 ","name":"step","result":{"status":"passed"}}]}]}]`
	f, err := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Bundle: true, Redaction: &report.Redaction{Patterns: []string{"TOK123"}}})
	if err != nil {
		t.Fatal(err)
	}
	round19AssertNotPublished(t, p, "TOK123")
}

// R19 re-review L1: a baseline or history entry that matches the current scope
// only after masking is refused, as an unmasked scope mismatch is.
func TestRound19MaskedOnlyScopeMatchIsRefused(t *testing.T) {
	in := round19Inputs(t)["json"]
	stored, err := report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Environment: "env-SECRET1"})
	if err != nil {
		t.Fatal(err)
	}
	red := &report.Redaction{Patterns: []string{`SECRET\d`}}
	for name, cfg := range map[string]report.RunConfig{
		"baseline": {Environment: "env-SECRET2", Baseline: &stored.Summary, Redaction: red},
		"history":  {Environment: "env-SECRET2", History: []report.RunSummary{stored.Summary}, Redaction: red},
	} {
		if _, err = report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, cfg); !errors.Is(err, report.ErrInvalidOptions) {
			t.Fatalf("%s: masked-only scope match accepted: %v", name, err)
		}
		cfg.Environment = "env-SECRET1" // same raw scope stays accepted
		if _, err = report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, cfg); err != nil {
			t.Fatalf("%s: same scope refused: %v", name, err)
		}
	}
}

// R19 re-review L-a: a zero-width character cannot split a configured JSON key.
func TestRound19ZeroWidthJSONKeyIsRedacted(t *testing.T) {
	raw := `[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","name":"S","line":2,"steps":[{"name":"step","doc_string":{"content_type":"application/json","value":"{\"pass\u200bword\":\"hunter2\"}"},"result":{"status":"passed"}}]}]}]`
	f, err := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Bundle: true, Redaction: &report.Redaction{JSONKeys: []string{"password"}}})
	if err != nil {
		t.Fatal(err)
	}
	round19AssertNotPublished(t, p, "hunter2")
}

// R19 re-review L-c: two different other-scope history entries must not merge
// into one masked scope.
func TestRound19MaskedHistoryScopesDoNotMerge(t *testing.T) {
	in := round19Inputs(t)["json"]
	var history []report.RunSummary
	for _, env := range []string{"env-SECRET1", "env-SECRET2"} {
		p, err := report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Environment: env})
		if err != nil {
			t.Fatal(err)
		}
		history = append(history, p.Summary)
	}
	red := &report.Redaction{Patterns: []string{`SECRET\d`}}
	if _, err := report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Environment: "prod", History: history, Redaction: red}); !errors.Is(err, report.ErrInvalidOptions) {
		t.Fatalf("merged masked scopes accepted: %v", err)
	}
	if _, err := report.PrepareRun([]report.Input{in}, report.Options{Version: report.OptionsVersion}, report.RunConfig{Environment: "prod", History: history[:1], Redaction: red}); err != nil {
		t.Fatalf("single other scope refused: %v", err)
	}
}
