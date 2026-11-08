package report

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRootLeakAcrossEvidenceAndMetadata(t *testing.T) {
	f := runFixture()
	secret := "sk_live_ROOTSECRET"
	f[0].Tags = []cucumber.Tag{{Name: "tag"}}
	for si := range f[0].Elements {
		sc := &f[0].Elements[si]
		sc.Tags = []cucumber.Tag{{Name: "tag"}}
		hook := func() cucumber.Step {
			return cucumber.Step{Name: "hook", Result: cucumber.Result{Status: cucumber.StatusPassed}, Embeddings: []cucumber.Embedding{{MimeType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte(secret))}}}
		}
		sc.Before = []cucumber.Step{hook()}
		sc.After = []cucumber.Step{hook()}
		sc.Steps[0].Before = []cucumber.Step{hook()}
		sc.Steps[0].After = []cucumber.Step{hook()}
		sc.Steps[0].DocString = &cucumber.DocString{ContentType: "text/plain", Value: "body"}
		sc.Steps[0].Rows = []cucumber.Row{{Cells: []string{"cell"}}}
	}
	fieldNumber := 0
	var plant func(reflect.Value, string)
	plant = func(v reflect.Value, field string) {
		if oneOf(field, "Status", "Type", "Keyword", "Origin", "ReportID", "MimeType", "ContentType", "Data", "Started", "Finished", "RecordedAt") {
			return
		}
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				plant(v.Elem(), field)
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				plant(v.Field(i), v.Type().Field(i).Name)
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				plant(v.Index(i), field)
			}
		case reflect.String:
			if v.CanSet() {
				fieldNumber++
				v.SetString(fmt.Sprintf("sk_live_FIELD_%d", fieldNumber))
			}
		}
	}
	plant(reflect.ValueOf(f), "")
	leak := regexp.MustCompile(`sk_live_[A-Za-z0-9_]+`)
	for _, mt := range []string{"message/http", "application/csv", "application/x-pem-file", "application/unknown-text", "text/plain; charset=iso-8859-1"} {
		body := []byte("café " + secret)
		if strings.Contains(mt, "iso-") {
			body = append([]byte{'c', 'a', 'f', 0xe9, ' '}, []byte(secret)...)
		}
		f[0].Elements[0].Steps[0].Embeddings = append(f[0].Elements[0].Steps[0].Embeddings, cucumber.Embedding{Name: secret, MimeType: mt, Data: base64.StdEncoding.EncodeToString(body)})
	}
	id := runScenarioIDs(f)[0][0]
	cfg := RunConfig{ID: secret, Environment: secret, Branch: secret, Revision: secret, Bundle: true, Redaction: &Redaction{Patterns: []string{`sk_live_[A-Za-z0-9_]+`}}, Metadata: map[string]ScenarioMeta{id: {Identity: secret, Worker: secret}}, Phases: []Phase{{Name: secret, Worker: secret, Started: "2026-11-06T00:00:00Z", Finished: "2026-11-06T00:00:01Z"}}}
	opt := renderOptions{Version: 2, Title: "sk_live_TITLE", Name: "sk_live_NAME", RunFacts: []Fact{{Label: "sk_live_LABEL", Value: "sk_live_VALUE"}}, WallClock: "sk_live_CLOCK", Sections: []Section{{ID: "details", Title: "sk_live_SECTION", Summary: []Fact{{Label: "sk_live_SUMMARY", Value: "sk_live_FACT"}}, Rows: []Fact{{Label: "sk_live_ROW", Value: "sk_live_CELL"}}, Note: "sk_live_NOTE"}}}
	p, prepareErr := preparePublicRun([]runInput{{Name: "sk_live_SHARD", Features: f}}, opt, cfg)
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}
	for _, sc := range p.Features[0].Elements {
		for _, st := range sc.Steps {
			for _, a := range st.Embeddings {
				raw, err := a.Decode(10000)
				if err != nil || leak.Match(raw) {
					t.Errorf("attachment leak: %s %v", a.MimeType, err)
				}
			}
		}
	}
	dir, err := writeRunDir(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() == "report.zip" {
			return nil
		}
		b, e := os.ReadFile(path)
		if e == nil && leak.Match(b) {
			t.Errorf("secret leaked in %s", d.Name())
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(filepath.Join(dir, "report.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, e := range z.File {
		r, e2 := e.Open()
		if e2 != nil {
			t.Fatal(e2)
		}
		b, e2 := io.ReadAll(r)
		r.Close()
		if e2 != nil {
			t.Fatal(e2)
		}
		if leak.Match(b) {
			t.Errorf("secret leaked in ZIP %s", e.Name)
		}
	}
}

func TestRootIdentityPermutationAndSubset(t *testing.T) {
	a, b := runFixture(), runFixture()
	a[0].URI = "a/login.feature"
	b[0].URI = "b/login.feature"
	a[0].Elements = a[0].Elements[:1]
	b[0].Elements = b[0].Elements[:1]
	inputs := []runInput{{Name: "a", Features: a}, {Name: "b", Features: b}}
	base, err := preparePublicRun(inputs, renderOptions{Version: 2}, RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]ScenarioSummary{}
	for _, s := range base.Summary.Scenarios {
		want[s.URI] = s
	}
	for _, run := range [][]runInput{{inputs[1], inputs[0]}, {inputs[1]}, {inputs[0]}} {
		cfg := RunConfig{Metadata: map[string]ScenarioMeta{}}
		for _, in := range run {
			uri := in.Features[0].URI
			cfg.Metadata[want[uri].ID] = ScenarioMeta{Identity: "explicit:" + uri, Worker: in.Name}
		}
		p, err := preparePublicRun(run, renderOptions{Version: 2}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range p.Summary.Scenarios {
			if s.ID != want[s.URI].ID || s.Identity != "explicit:"+s.URI || s.Worker != string(s.URI[0]) {
				t.Fatalf("order/subset changed binding: %+v; expected %+v", s, want[s.URI])
			}
		}
	}
}

func TestRootFormatParityAndOutlineSubsets(t *testing.T) {
	legacy, err := cucumber.Parse(strings.NewReader(`[{"uri":"o.feature","elements":[{"type":"scenario","keyword":"Scenario Outline","name":"login alice","line":8,"steps":[{"name":"go","result":{"status":"passed"}}]},{"type":"scenario","keyword":"Scenario Outline","name":"login bob","line":9,"steps":[{"name":"go","result":{"status":"passed"}}]}]}]`), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"gherkinDocument":{"uri":"o.feature","feature":{"children":[{"scenario":{"id":"ast","examples":[{"tableBody":[{"id":"r1","location":{"line":8}},{"id":"r2","location":{"line":9}}]}]}}]}}}
{"pickle":{"id":"p","uri":"o.feature","name":"login alice","astNodeIds":["ast","r1"],"steps":[{"id":"ps","text":"go"}]}}
{"testCase":{"id":"tc","pickleId":"p","testSteps":[{"id":"ts","pickleStepId":"ps"}]}}
{"testCaseStarted":{"id":"a","testCaseId":"tc","attempt":0,"timestamp":{"seconds":"1","nanos":0}}}
{"testStepFinished":{"testCaseStartedId":"a","testStepId":"ts","testStepResult":{"status":"PASSED","duration":{"seconds":"0","nanos":0}}}}
{"testCaseFinished":{"testCaseStartedId":"a","timestamp":{"seconds":"2","nanos":0}}}
{"testRunFinished":{"success":true}}`
	mr, err := cucumber.ParseMessages(strings.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	full := prepared(t, legacy, RunConfig{})
	one, err := preparePublicRun([]runInput{{Name: "messages", Attempts: mr.Attempts}}, renderOptions{Version: 2}, RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if full.Summary.Scenarios[0].Identity != one.Summary.Scenarios[0].Identity || full.Summary.Scenarios[0].Status != one.Summary.Scenarios[0].Status {
		t.Fatalf("formats differ: %+v / %+v", full.Summary.Scenarios[0], one.Summary.Scenarios[0])
	}
	legacy[0].Elements = legacy[0].Elements[:1]
	subset := prepared(t, legacy, RunConfig{})
	if subset.Summary.Scenarios[0].Identity != full.Summary.Scenarios[0].Identity {
		t.Fatal("subset identity changed")
	}
}

func TestRootHistoryKeepsOtherScopes(t *testing.T) {
	first := prepared(t, runFixture(), RunConfig{ID: "stg-1", Environment: "staging"})
	second := prepared(t, runFixture(), RunConfig{ID: "prod-1", Environment: "prod", History: first.History})
	third := prepared(t, runFixture(), RunConfig{ID: "stg-2", Environment: "staging", History: second.History})
	if len(third.History) != 3 || third.Summary.Scenarios[0].Samples != 2 {
		b, _ := json.Marshal(third.History)
		t.Fatalf("history scopes lost or mixed: %s", b)
	}
}

func TestRootRedactionTypesHeadersAndStructure(t *testing.T) {
	r, err := newRedactor(&Redaction{Patterns: []string{`sk_live_\w+`}, Headers: []string{"Authorization"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mt := range []string{"text/plain", "message/http", "application/csv", "application/x-pem-file", "application/x-custom", "application/javascript", "text/css", "text/html", "text/xml", "application/xml", "application/graphql", "application/yaml", "text/markdown", "text/log", ""} {
		raw, e := r.attachmentText([]byte("sk_live_TOKEN"), mt)
		if e != nil || string(raw) != "[REDACTED]" {
			t.Errorf("unscanned %s: %q %v", mt, raw, e)
		}
	}
	raw, err := r.attachmentText([]byte(`{"headers":{"AUTHORIZATION":"Bearer secret"}}`), "application/x-json")
	if err != nil || bytes.Contains(raw, []byte("Bearer secret")) {
		t.Errorf("JSON header leak: %s %v", raw, err)
	}
	if got := r.text("Authorization:\nNext: keep"); got != "Authorization:\nNext: keep" {
		t.Errorf("header swallowed line: %q", got)
	}
	if _, e := newRedactor(&Redaction{Headers: []string{" "}}); e == nil {
		t.Error("blank header accepted")
	}
	f := runFixture()
	f[0].Elements[0].Type = "run_hook"
	p := prepared(t, f, RunConfig{Redaction: &Redaction{Patterns: []string{"run_hook", "failed", "check"}}})
	if p.Summary.RunHooks != 1 || p.Summary.Counts["failed"] != 1 {
		t.Errorf("redaction changed structure: %+v", p.Summary)
	}
	f = runFixture()
	f[0].Elements[0].Steps[0].Embeddings = []cucumber.Embedding{{Data: "invalid base64"}}
	if _, e := preparePublicRun([]runInput{{Name: "a", Features: f}}, renderOptions{Version: 2}, RunConfig{Redaction: &Redaction{}}); e != nil {
		t.Errorf("empty policy decoded evidence: %v", e)
	}
}

func TestRootDurationGateRequiresComparablePasses(t *testing.T) {
	for _, status := range []cucumber.Status{cucumber.StatusSkipped, cucumber.StatusPassed} {
		for _, baseNanos := range []int64{0, 1, 1000} {
			f := runFixture()
			f[0].Elements = f[0].Elements[:1]
			f[0].Elements[0].Steps[0].Result.Status = status
			d := time.Duration(baseNanos)
			f[0].Elements[0].Steps[0].Result.Duration = &d
			base := prepared(t, f, RunConfig{ID: "base"})
			f[0].Elements[0].Steps[0].Result.Status = cucumber.StatusPassed
			d = time.Duration(baseNanos + 1000)
			now := prepared(t, f, RunConfig{ID: "now", Baseline: &base.Summary, Gates: GatePolicy{MaxRegressionPercent: 50}})
			if len(now.Summary.GateFailures) > 0 || len(now.Summary.Comparison.Slower) > 0 {
				t.Errorf("incomparable duration gated: %s %d", status, baseNanos)
			}
		}
	}
}

func TestRootHistoryRoundTripBudget(t *testing.T) {
	p := prepared(t, runFixture(), RunConfig{})
	rows := make([]ScenarioSummary, 3000)
	for i := range rows {
		rows[i] = ScenarioSummary{Identity: fmt.Sprintf("case-%d", i), Name: strings.Repeat("description", 70), URI: "features/check.feature", ID: fmt.Sprintf("nav-%d", i), Status: "passed", Duration: 1}
	}
	p.History = nil
	for i := 0; i < 30; i++ {
		p.History = append(p.History, RunSummary{Version: SummaryVersion, ID: fmt.Sprintf("run-%d", i), Scenarios: rows})
	}
	dir, err := writeRunDir(p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 64<<20 {
		t.Fatalf("writer exceeded reader budget: %d", len(raw))
	}
	var history []RunSummary
	if err = json.Unmarshal(raw, &history); err != nil {
		t.Fatal(err)
	}
	for _, s := range history {
		if err = validateSummary(s); err != nil {
			t.Fatal(err)
		}
	}
	if len(history) != 30 {
		t.Fatalf("needlessly lost histories: %d", len(history))
	}
}

func TestRootBundleDeduplicatesOnlyContentAddressedEvidence(t *testing.T) {
	f := runFixture()
	f[0].Elements = f[0].Elements[:1]
	for _, body := range []string{"binary evidence", "binary evidence", "distinct binary evidence"} {
		f[0].Elements[0].Steps[0].Embeddings = append(f[0].Elements[0].Steps[0].Embeddings, cucumber.Embedding{MimeType: "application/octet-stream", Data: base64.StdEncoding.EncodeToString([]byte(body))})
	}
	dir, err := renderFeaturesDirTo(f, t.TempDir(), renderOptions{Version: 2, BundleAttachments: true, Limits: pub.Limits{InlineAttachment: 1}})
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "attachment-*"))
	if err != nil || len(files) != 2 {
		t.Fatalf("deduplication failed: %v %v", files, err)
	}
	writer := newSidecarWriter(t.TempDir())
	if err = writer("position.txt", []byte("same")); err != nil {
		t.Fatal(err)
	}
	if err = writer("position.txt", []byte("same")); err == nil {
		t.Fatal("positional name collision accepted")
	}
}

func TestRootByteRedactionAndStructuralLinks(t *testing.T) {
	r, err := newRedactor(&Redaction{Patterns: []string{`sk_live_\w+`}})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"café sk_live_SECRET", "\xffsk_live_SECRET", "sk_live_A sk_live_B", "[REDACTED] sk_live_C"} {
		if string(r.textBytes([]byte(body))) != r.text(body) {
			t.Fatal("string and byte masking differ")
		}
	}
	f := runFixture()
	f[0].URI = "login.feature"
	f[0].Elements = f[0].Elements[:1]
	cfg := RunConfig{Redaction: &Redaction{Patterns: []string{"login", "redacted", "failed", "scenario"}}}
	base := prepared(t, f, cfg)
	f[0].Elements[0].Steps[0].Result.Status = cucumber.StatusFailed
	cfg.Baseline = &base.Summary
	p := prepared(t, f, cfg)
	if len(p.Summary.Comparison.NewFailures) != 1 || p.Summary.Comparison.NewFailures[0] != p.Summary.Scenarios[0].ID || p.Summary.Counts["failed"] != 1 {
		t.Fatal("structural links or classification changed")
	}
	for _, mt := range []string{"image/png", "audio/wav", "video/mp4", "font/woff2", "application/pdf", "application/zip", "application/octet-stream", "application/gzip"} {
		raw, e := r.attachmentText([]byte("sk_live_SECRET"), mt)
		if e != nil || string(raw) != "sk_live_SECRET" {
			t.Errorf("binary type mutated: %s", mt)
		}
	}
}

func TestRootJSONPatternTokenContract(t *testing.T) {
	raw := []byte(`{"token":"abc","nested":"{\"password\":\"value\"}"}`)
	r, err := newRedactor(&Redaction{Patterns: []string{`"token"\s*:\s*"[^"]*"`}, JSONKeys: []string{"password"}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.attachmentText(raw, "application/json")
	if err != nil || !bytes.Equal(out, raw) {
		t.Fatalf("documented token-only contract changed: %s %v", out, err)
	}
	r, _ = newRedactor(&Redaction{JSONKeys: []string{"token", "nested"}})
	out, err = r.attachmentText(raw, "application/json")
	if err != nil || bytes.Contains(out, []byte("abc")) || bytes.Contains(out, []byte("value")) {
		t.Fatal("explicit outer-key masking failed")
	}
}

func TestRootMessagesMetadataRedactionAndSingleMerge(t *testing.T) {
	makeInput := func(name, uri string) runInput {
		f := runFixture()
		sc := f[0].Elements[0]
		sc.ID = "runner-reuses-id"
		failed := sc
		failed.Steps = append([]cucumber.Step(nil), sc.Steps...)
		failed.Steps[0].Result.Status = cucumber.StatusFailed
		return runInput{Name: name, Attempts: []cucumber.MessageAttempt{
			{URI: uri, Scenario: sc, Number: 1, Worker: "sk_live_FINAL"},
			{URI: uri, Scenario: failed, Number: 0, Worker: "sk_live_RETRY"},
		}}
	}
	a, b := makeInput("a", "a/check.feature"), makeInput("b", "b/check.feature")
	for _, inputs := range [][]runInput{{a, b}, {b, a}, {b}} {
		p, err := preparePublicRun(inputs, renderOptions{Version: 2}, RunConfig{Bundle: true, Redaction: &Redaction{Patterns: []string{`sk_live_\w+`}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, sc := range p.Summary.Scenarios {
			if len(sc.Attempts) != 2 || sc.Attempts[0].Status != "failed" || sc.Attempts[1].Status != "passed" || sc.Worker != "[REDACTED]" {
				t.Fatalf("attempt merge: %+v", sc)
			}
			if len(p.Metadata[sc.ID].Attempts[1].Steps) != 0 {
				t.Fatal("final evidence duplicated")
			}
		}
		dir, err := writeRunDir(p, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"index.html", "summary.json", "history.json", "scenarios.csv", "summary.md"} {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte("sk_live_")) {
				t.Fatalf("worker leaked in %s", name)
			}
		}
	}
	if a.Attempts[0].Worker != "sk_live_FINAL" || a.Attempts[0].Scenario.Steps[0].Result.Status != cucumber.StatusPassed {
		t.Fatal("caller input changed")
	}
}

func TestRootLibraryHooksValidateFoldedEvidence(t *testing.T) {
	f := runFixture()
	f[0].Elements[0].Before = []cucumber.Step{{Result: cucumber.Result{Status: "invented"}}}
	if _, err := preparePublicRun([]runInput{{Name: "library", Features: f}}, renderOptions{Version: 2}, RunConfig{}); err == nil {
		t.Fatal("invalid direct-library hook passed preparation")
	}
}
