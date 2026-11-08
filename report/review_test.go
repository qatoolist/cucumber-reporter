package report

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestEnhancedOutlineRowsAndBackgrounds(t *testing.T) {
	f := runFixture()
	row := f[0].Elements[0]
	row.Name = "same outline name"
	row.Type = "scenario_outline"
	row.Line = 10
	other := row
	other.Line = 11
	background := row
	background.Type = "background"
	background.Name = "setup"
	background.Line = 2
	f[0].Elements = []cucumber.Scenario{background, row, background, other}
	p := prepared(t, f, RunConfig{ExpectedScenarios: 2})
	if len(p.Features[0].Elements) != 2 || p.Summary.Completeness != "complete" {
		t.Fatalf("outline/background handling: %+v", p.Summary)
	}
	if p.Summary.Scenarios[0].Identity == p.Summary.Scenarios[1].Identity {
		t.Fatal("outline rows share an identity")
	}
	// Legacy JSON has no full AST catalog: a filtered singleton loses collision context.
	f[0].Elements = []cucumber.Scenario{other}
	single := prepared(t, f, RunConfig{})
	if single.Summary.Scenarios[0].Identity == p.Summary.Scenarios[1].Identity {
		t.Fatal("legacy subset unexpectedly retained collision context")
	}
}

func TestRepeatedPlainScenarioNamesUseLineFallback(t *testing.T) {
	f := runFixture()
	f[0].Elements[1].Name = f[0].Elements[0].Name
	fallback := prepared(t, f, RunConfig{})
	if fallback.Summary.Scenarios[0].Identity == fallback.Summary.Scenarios[1].Identity || len(fallback.Summary.Diagnostics) == 0 {
		t.Fatal("colliding names need distinct identities and a diagnostic")
	}
	ids := runScenarioIDs(f)
	p := prepared(t, f, RunConfig{Metadata: map[string]ScenarioMeta{ids[0][0]: {Identity: "first"}, ids[0][1]: {Identity: "second"}}})
	if p.Summary.Scenarios[0].Identity == p.Summary.Scenarios[1].Identity {
		t.Fatal("explicit identities collapsed")
	}
}

func TestIdenticalExecutionsGrowHistory(t *testing.T) {
	first := prepared(t, runFixture(), RunConfig{})
	second := prepared(t, runFixture(), RunConfig{History: first.History})
	third := prepared(t, runFixture(), RunConfig{History: second.History})
	if first.Summary.ID == second.Summary.ID || len(third.History) != 3 || third.Summary.Scenarios[0].Samples != 3 {
		t.Fatalf("reruns collapsed: %+v", third.Summary)
	}
}

func TestRunHooksExcludedFromExpectedScenarioCount(t *testing.T) {
	f := runFixture()
	hook := f[0].Elements[1]
	hook.Type = "run_hook"
	hook.Name = "BeforeAll"
	hook.Line = 20
	f[0].Elements = append(f[0].Elements, hook)
	p := prepared(t, f, RunConfig{ExpectedScenarios: 2, Gates: GatePolicy{Unexpected: true, Incomplete: true}})
	if p.Summary.ObservedScenarios != 2 || p.Summary.RunHooks != 1 || p.Summary.Completeness != "complete" || len(p.Summary.GateFailures) != 1 {
		t.Fatalf("hook accounting: %+v", p.Summary)
	}
	if strings.Contains(RerunLocations(p.Summary, nil), ":20") {
		t.Fatal("run hook exported as rerunnable scenario")
	}
}

func TestRedactionXMLAndEscapedJSONWithoutKeyRules(t *testing.T) {
	f := runFixture()
	secret := `https://secret.test/"quoted"`
	payload := `{"url":"https:\/\/secret.test\/\"quoted\"","count":9007199254740993}`
	f[0].Elements[0].Steps[0].Embeddings = []cucumber.Embedding{
		{Name: "xml", MimeType: "application/xml", Data: base64.StdEncoding.EncodeToString([]byte("<token>secret-value</token>"))},
		{Name: "json", MimeType: "application/json", Data: base64.StdEncoding.EncodeToString([]byte(payload))},
	}
	f[0].Elements[0].Steps[0].DocString = &cucumber.DocString{ContentType: "application/json", Value: payload}
	p := prepared(t, f, RunConfig{Redaction: &Redaction{Patterns: []string{regexp.QuoteMeta(secret), "secret-value", "REDACTED"}}})
	st := p.Features[0].Elements[0].Steps[0]
	for _, a := range st.Embeddings {
		raw, err := a.Decode(10000)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "[[") {
			t.Fatalf("redaction failed: %s", raw)
		}
	}
	if !json.Valid([]byte(st.DocString.Value)) || strings.Contains(st.DocString.Value, "secret") || !strings.Contains(st.DocString.Value, "9007199254740993") {
		t.Fatal("JSON doc string escaped redaction or changed number")
	}
}

func TestRedactedIdentityAndRerunSafety(t *testing.T) {
	f := runFixture()
	f[0].URI = "private-token/check.feature"
	f[0].Elements[0].Name = "private-token"
	rawID := Identity(f[0].URI, f[0].Elements[0].Name)
	p := prepared(t, f, RunConfig{Redaction: &Redaction{Patterns: []string{"private-token", "REDACTED"}}})
	if p.Summary.Scenarios[0].Identity == rawID {
		t.Fatal("identity still hashes unredacted secret")
	}
	if !p.Summary.Scenarios[0].RerunUnavailable || RerunLocations(p.Summary, nil) != "" {
		t.Fatal("unusable redacted rerun selectors exported")
	}
	b, _ := json.Marshal(p.Summary)
	if strings.Contains(string(b), "[[REDACTED]]") {
		t.Fatal("summary was double-redacted")
	}
}

func TestCSVFormulaGuardUnicodeWhitespace(t *testing.T) {
	for _, s := range []string{"\u00a0=1+1", "\u2003@SUM(A1)", "\ufeff+1", "\tlabel"} {
		if !strings.HasPrefix(csvSafe(s), "'") {
			t.Fatalf("unguarded %q", s)
		}
	}
}

func TestBackgroundEvidenceAndIsolation(t *testing.T) {
	f := runFixture()
	bg := cucumber.Scenario{Type: "background", Steps: []cucumber.Step{{Name: "setup", Result: cucumber.Result{Status: cucumber.StatusFailed, Error: "BG BOOM"}, Embeddings: []cucumber.Embedding{{MimeType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte("setup evidence"))}}}}}
	f[0].Elements = append([]cucumber.Scenario{bg}, f[0].Elements...)
	p := prepared(t, f, RunConfig{Gates: GatePolicy{Unexpected: true}})
	if len(p.Features[0].Elements[0].Steps) != 2 || len(p.Features[0].Elements[1].Steps) != 1 || p.Summary.Scenarios[0].Status != "failed" || p.Features[0].Elements[0].Steps[0].Result.Error != "BG BOOM" {
		t.Fatal("Background evidence missing or reused")
	}
	p.Features[0].Elements[0].Steps[0].Embeddings[0].Data = "changed"
	if f[0].Elements[0].Steps[0].Embeddings[0].Data == "changed" || len(f[0].Elements[1].Steps) != 1 {
		t.Fatal("caller data mutated")
	}
	f[0].Elements = []cucumber.Scenario{bg}
	orphan, err := preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, RunConfig{})
	if err != nil || len(orphan.Summary.Diagnostics) == 0 {
		t.Fatalf("orphan Background lacks diagnostic: %v", err)
	}
}

func TestExplicitExecutionIDSupportsReprocessing(t *testing.T) {
	first := prepared(t, runFixture(), RunConfig{ID: "ci-build-123"})
	again := prepared(t, runFixture(), RunConfig{ID: "ci-build-123", History: first.History})
	if len(again.History) != 1 || again.Summary.Scenarios[0].Samples > 1 {
		t.Fatal("same execution counted twice")
	}
	one, _ := json.Marshal(first.Summary)
	repeat := prepared(t, runFixture(), RunConfig{ID: "ci-build-123"})
	two, _ := json.Marshal(repeat.Summary)
	if string(one) != string(two) {
		t.Fatal("explicit execution ID is not reproducible")
	}
}

func TestHookFeaturesDoNotRenameRealFeature(t *testing.T) {
	features := []cucumber.Feature{
		{URI: "__run_hooks__/stream-1.feature", Elements: []cucumber.Scenario{{Type: "run_hook"}}},
		{URI: "run-hooks.feature", Elements: []cucumber.Scenario{{Type: "scenario"}}},
		{URI: "stream-1.feature", Elements: []cucumber.Scenario{{Type: "scenario"}}},
	}
	ids := featureIDs(features)
	if ids[0] != "run-hooks-2" || ids[1] != "run-hooks" || ids[2] != "stream-1" {
		t.Fatalf("hook changed ordinary feature IDs: %v", ids)
	}
}
