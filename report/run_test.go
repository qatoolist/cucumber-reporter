package report

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func runFixture() []cucumber.Feature {
	d := time.Second
	return []cucumber.Feature{{URI: "features/check.feature", Name: "Checks", Elements: []cucumber.Scenario{{Name: "accepts an order", Line: 3, Steps: []cucumber.Step{{Name: "accepted", Result: cucumber.Result{Status: cucumber.StatusPassed, Duration: &d}}}}, {Name: "rejects an invalid order", Line: 8, Steps: []cucumber.Step{{Name: "rejected", Result: cucumber.Result{Status: cucumber.StatusFailed, Error: "expected 400, got 200", Duration: &d}}}}}}}
}

func prepared(t *testing.T, features []cucumber.Feature, cfg RunConfig) *preparedRun {
	t.Helper()
	p, err := preparePublicRun([]runInput{{Name: "shard-a", Features: features}}, renderOptions{Version: 2}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunIdentityComparisonAndOptionalHistory(t *testing.T) {
	base := prepared(t, runFixture(), RunConfig{ID: "base", Environment: "staging", Branch: "main", Metadata: map[string]ScenarioMeta{runScenarioIDs(runFixture())[0][0]: {Identity: "accept-order"}}})
	f := runFixture()
	f[0].Elements[0].Line = 99
	f[0].Elements[0].Steps[0].Result.Status = cucumber.StatusFailed
	f[0].Elements[1].Steps[0].Result.Status = cucumber.StatusPassed
	now := prepared(t, f, RunConfig{ID: "now", Environment: "staging", Branch: "main", Metadata: map[string]ScenarioMeta{runScenarioIDs(f)[0][0]: {Identity: "accept-order"}}, Baseline: &base.Summary, History: []RunSummary{base.Summary}, Gates: GatePolicy{NewFailures: true}})
	if now.Summary.Scenarios[0].Identity != base.Summary.Scenarios[0].Identity {
		t.Fatal("line edit changed historical identity")
	}
	if now.Summary.Scenarios[0].ID == base.Summary.Scenarios[0].ID {
		t.Fatal("navigation IDs unexpectedly changed their contract")
	}
	c := now.Summary.Comparison
	if len(c.NewFailures) != 1 || len(c.Fixed) != 1 || len(now.Summary.GateFailures) != 1 {
		t.Fatalf("comparison: %+v gates %v", c, now.Summary.GateFailures)
	}
	if now.Summary.Scenarios[0].Samples != 2 || now.Summary.Scenarios[0].Failures != 1 {
		t.Fatal("historical frequency")
	}
	if base.Summary.Comparison != nil || len(base.Summary.History) != 0 || base.Summary.Scenarios[0].Samples != 0 {
		t.Fatal("history should be absent by default")
	}
	_, err := preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, RunConfig{Baseline: &base.Summary, Environment: "production"})
	if err == nil {
		t.Fatal("mixed baseline scope accepted")
	}
}

func TestRunCompletenessDuplicatesAndGates(t *testing.T) {
	f := runFixture()
	p := prepared(t, f, RunConfig{ExpectedShards: []string{"shard-a", "shard-b"}, ExpectedScenarios: 3, Gates: GatePolicy{Incomplete: true, NewFailures: true}})
	if p.Summary.Completeness != "incomplete" || len(p.Summary.GateFailures) != 2 {
		t.Fatalf("%+v", p.Summary)
	}
	dir, err := writeRunDir(p, t.TempDir())
	if !errors.Is(err, ErrGateFailed) {
		t.Fatalf("gate result %v", err)
	}
	for _, name := range []string{"index.html", "summary.json", "history.json", "scenarios.csv", "summary.md", "rerun.txt", "attachments.json"} {
		if _, err = os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	_, err = preparePublicRun([]runInput{{Name: "a", Features: f}, {Name: "b", Features: f}}, renderOptions{Version: 2}, RunConfig{})
	if err == nil {
		t.Fatal("cross-input reruns must be refused in v0.3.0")
	}
	p = prepared(t, f, RunConfig{Gates: GatePolicy{NewFailures: true, AllowFirstRun: true}})
	if len(p.Summary.GateFailures) != 0 {
		t.Fatal("explicit first-run policy")
	}
}

func TestRunRedactionAllArtifactsAndCallerIsolation(t *testing.T) {
	f := runFixture()
	secret := "super-secret-123"
	f[0].Elements[1].Steps[0].Result.Error = secret
	f[0].Elements[0].Steps[0].Embeddings = []cucumber.Embedding{{Name: "response", MimeType: "application/json", Data: base64.StdEncoding.EncodeToString([]byte(`{"token":"sensitive","number":9007199254740993,"other":"` + secret + `"}`))}}
	p := prepared(t, f, RunConfig{Redaction: &Redaction{Patterns: []string{secret}, JSONKeys: []string{"token"}}, Bundle: true})
	if f[0].Elements[1].Steps[0].Result.Error != secret {
		t.Fatal("caller mutated")
	}
	raw, err := p.Features[0].Elements[0].Steps[0].Embeddings[0].Decode(10000)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sensitive") || !strings.Contains(string(raw), "9007199254740993") {
		t.Fatalf("JSON redaction changed numeric value or leaked key: %s", raw)
	}
	dir, err := writeRunDir(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == "report.zip" {
			continue
		}
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		if bytes.Contains(b, []byte(secret)) {
			t.Fatalf("secret in %s", e.Name())
		}
	}
	z, err := zip.OpenReader(filepath.Join(dir, "report.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if len(z.File) < 7 {
		t.Fatal("incomplete bundle")
	}
}

func TestRunRetryPoliciesAndBinaryEvidence(t *testing.T) {
	f := runFixture()
	id := runScenarioIDs(f)[0][1]
	failed := f[0].Elements[1].Steps
	p := prepared(t, f, RunConfig{Metadata: map[string]ScenarioMeta{id: {Attempts: []Attempt{{Number: 0, Status: "failed", Steps: publicSteps(failed, 0)}, {Number: 1, Status: "failed", Steps: publicSteps(failed, 0)}}}}})
	if !p.Summary.Scenarios[1].Retried || p.Summary.Scenarios[1].FirstStatus != "failed" {
		t.Fatal("attempt classification")
	}
	f[0].Elements[0].Steps[0].Embeddings = []cucumber.Embedding{{Name: "video", MimeType: "video/mp4", Data: base64.StdEncoding.EncodeToString([]byte("binary evidence"))}, {Name: "duplicate", MimeType: "video/mp4", Data: base64.StdEncoding.EncodeToString([]byte("binary evidence"))}}
	p = prepared(t, f, RunConfig{Bundle: true})
	p.Options.Limits.InlineAttachment = 1
	dir, err := writeRunDir(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "attachment-*.bin"))
	if len(entries) != 1 {
		t.Fatalf("wanted deduplicated binary, got %v", entries)
	}
	opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: id, Kind: KindKnown, Ref: "ISS-1", Step: 1, StepText: "rejected"}}}
	p, err = preparePublicRun([]runInput{{Name: "a", Features: f}}, opt, RunConfig{RecordedAt: "2026-11-06T00:00:00Z", Issues: []IssuePolicy{{Ref: "ISS-1", Expires: "2026-11-01"}}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Counts["failed"] != 1 || len(p.Summary.Diagnostics) != 1 {
		t.Fatal("expired exception hid failure")
	}
}

func TestRenderToMatchesRender(t *testing.T) {
	f := runFixture()
	opt := renderOptions{Version: 2}
	want, err := renderPublicModel(f, opt)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err = renderTo(&b, f, opt); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, b.Bytes()) {
		t.Fatal("writer output differs")
	}
}

func TestDiagnosticsDoNotLeakUnknownNames(t *testing.T) {
	raw := []byte(`[{"uri":"f","elements":[{"name":"x","steps":[{"result":{"status":"passed"}}],"output":[{"secret-marker":"secret"}]}]}]`)
	ds, err := validateInput(raw, cucumber.Limits{})
	if err != nil || len(ds) != 1 {
		t.Fatalf("%v %v", ds, err)
	}
	b, _ := json.Marshal(ds)
	if bytes.Contains(b, []byte("secret")) {
		t.Fatal("diagnostics expose content")
	}
}

func BenchmarkPrepareRun(b *testing.B) {
	f := runFixture()
	for b.Loop() {
		if _, err := preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, RunConfig{}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRedactionRemapsNavigationWithoutLosingAnnotations(t *testing.T) {
	f := runFixture()
	f[0].URI = "private-secret/check.feature"
	id := runScenarioIDs(f)[0][1]
	opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: id, Kind: KindKnown, Ref: "ISS-1", Step: 1, StepText: "rejected"}}}
	p, err := preparePublicRun([]runInput{{Name: "a", Features: f}}, opt, RunConfig{Redaction: &Redaction{Patterns: []string{"check", "private-secret"}}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Counts["known"] != 1 {
		t.Fatal("redaction changed classification")
	}
	if _, err = renderPublicModel(p.Features, p.Options); err != nil {
		t.Fatal(err)
	}
}

func TestRedactionDoesNotMutateHistoricalConfiguration(t *testing.T) {
	base := prepared(t, runFixture(), RunConfig{ID: "base"})
	base.Summary.Scenarios[0].Name = "secret-name"
	cfg := RunConfig{ID: "next", History: []RunSummary{base.Summary}, Issues: []IssuePolicy{{Ref: "ISS-1", Owner: "secret-owner"}}, Phases: []Phase{{Name: "secret-phase", Started: "2026-11-06T00:00:00Z", Finished: "2026-11-06T00:00:01Z"}}, Redaction: &Redaction{Patterns: []string{"secret-[a-z]+"}}}
	before, _ := json.Marshal(cfg)
	p := prepared(t, runFixture(), cfg)
	after, _ := json.Marshal(cfg)
	if !bytes.Equal(before, after) {
		t.Fatal("caller configuration mutated")
	}
	encoded, _ := json.Marshal(p.History)
	if bytes.Contains(encoded, []byte("secret-")) {
		t.Fatal("history or policy display text was not redacted")
	}
}

func TestRunManifestMatchesTextSidecars(t *testing.T) {
	f := runFixture()
	f[0].Elements[0].Steps[0].Embeddings = []cucumber.Embedding{{Name: "text", MimeType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte("long evidence"))}}
	p := prepared(t, f, RunConfig{})
	p.Options.Limits.InlineAttachment = 1
	dir, err := writeRunDir(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "attachments.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest []AttachmentManifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest) != 1 || manifest[0].State != "sidecar" || manifest[0].File == "" {
		t.Fatalf("incorrect manifest: %+v", manifest)
	}
	if _, err = os.Stat(filepath.Join(dir, manifest[0].File)); err != nil {
		t.Fatal(err)
	}
}
