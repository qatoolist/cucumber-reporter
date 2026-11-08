package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/report"
)

func TestEnhancedFirstRunAndGateArtifacts(t *testing.T) {
	dir := newRun(t, []byte(`[{"uri":"checks.feature","name":"checks","elements":[{"name":"fails","line":4,"steps":[{"name":"check","result":{"status":"failed","error_message":"wrong value"}}]}]}]`))
	stdout, stderr, code := command(t, "-enhanced", "-bundle", "-fail-on-unexpected", dir)
	if code != 3 || !strings.Contains(stderr, "Quality gate:") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	root := filepath.Dir(reportPath(t, stdout))
	for _, name := range []string{"index.html", "summary.json", "history.json", "scenarios.csv", "summary.md", "rerun.txt", "attachments.json", "report.zip"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary report.RunSummary
	if err = json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Version != report.SummaryVersion || summary.Comparison != nil || len(summary.History) != 0 || summary.Counts["failed"] != 1 {
		t.Fatalf("unexpected first run: %+v", summary)
	}
	selectors, err := os.ReadFile(filepath.Join(root, "rerun.txt"))
	if err != nil || string(selectors) != "checks.feature:4\n" {
		t.Fatalf("rerun %q: %v", selectors, err)
	}
}

func TestEnhancedComparisonAndValidation(t *testing.T) {
	dir := newRun(t, []byte(`[{"uri":"checks.feature","elements":[{"name":"passes","line":4,"steps":[{"result":{"status":"passed"}}]}]}]`))
	config := writeFile(t, `{"gates":{"newFailures":true}}`)
	out, stderr, code := command(t, "-run-config", config, dir)
	if code != 3 || !strings.Contains(stderr, "baseline") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	baseline := filepath.Join(filepath.Dir(reportPath(t, out)), "summary.json")
	_, stderr, code = command(t, "-baseline", baseline, "-run-config", config, dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	unsupported := newRun(t, []byte(`[{"elements":[{"steps":[{"result":{"status":"passed"},"before":[{"secret":"never echo this"}]}]}]}]`))
	stdout, stderr, code := command(t, "-validate", "-strict", unsupported)
	if code != 1 || stdout != "" || strings.Contains(stderr, "never echo this") {
		t.Fatalf("unsafe diagnostic: %d %s %s", code, stdout, stderr)
	}
	entries, err := os.ReadDir(unsupported)
	if err != nil || len(entries) != 1 {
		t.Fatal("validation wrote output", err)
	}
}

func TestMessagesCLI(t *testing.T) {
	input := writeFile(t, `{"gherkinDocument":{"uri":"check.feature","feature":{"name":"Named feature","tags":[{"name":"@feature"}]}}}
{"pickle":{"id":"p","uri":"check.feature","name":"passes","steps":[{"id":"ps","text":"accepted"}]}}
{"testCase":{"id":"tc","pickleId":"p","testSteps":[{"id":"ts","pickleStepId":"ps"}]}}
{"testCaseStarted":{"id":"a","testCaseId":"tc","attempt":0,"workerId":"w1","timestamp":{"seconds":"1","nanos":0}}}
{"testStepFinished":{"testCaseStartedId":"a","testStepId":"ts","testStepResult":{"status":"PASSED","duration":{"seconds":"0","nanos":10}}}}
{"testCaseFinished":{"testCaseStartedId":"a","timestamp":{"seconds":"2","nanos":0}}}
{"testRunFinished":{"success":true}}`)
	stdout, stderr, code := command(t, "-format", "messages", input)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	doc, err := os.ReadFile(reportPath(t, stdout))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name":"Named feature"`, `"@feature"`, `"execution":`, `"worker":"w1"`} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestValidateAppliesRequestedGates(t *testing.T) {
	dir := newRun(t, []byte(`[{"uri":"f.feature","elements":[{"name":"fails","line":1,"steps":[{"result":{"status":"failed"}}]}]}]`))
	_, stderr, code := command(t, "-validate", "-fail-on-unexpected", dir)
	if code != 3 || !strings.Contains(stderr, "Quality gate:") {
		t.Fatalf("validation ignored gate: %d %s", code, stderr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("validation wrote report artifacts")
	}
}

func TestShardedMessagesOutlineAndRunHooks(t *testing.T) {
	dir := t.TempDir()
	var files []string
	for i, line := range []int{10, 11} {
		file := filepath.Join(dir, fmt.Sprintf("shard-%d.ndjson", i))
		input := fmt.Sprintf(`{"gherkinDocument":{"uri":"outline.feature","feature":{"name":"Outline","children":[{"scenario":{"id":"ast","location":{"line":3},"examples":[{"tableBody":[{"id":"row","location":{"line":%d}}]}]}}]}}}
{"pickle":{"id":"p","uri":"outline.feature","name":"same outline name","astNodeIds":["ast","row"],"steps":[{"id":"ps","text":"accepted"}]}}
{"testCase":{"id":"tc","pickleId":"p","testSteps":[{"id":"ts","pickleStepId":"ps"}]}}
{"testCaseStarted":{"id":"a","testCaseId":"tc","attempt":0,"timestamp":{"seconds":"1","nanos":0}}}
{"testStepFinished":{"testCaseStartedId":"a","testStepId":"ts","testStepResult":{"status":"PASSED","duration":{"seconds":"0","nanos":10}}}}
{"testCaseFinished":{"testCaseStartedId":"a","timestamp":{"seconds":"2","nanos":0}}}
{"hook":{"id":"h","name":"BeforeAll"}}
{"testRunHookStarted":{"id":"r","hookId":"h","timestamp":{"seconds":"1","nanos":0}}}
{"testRunHookFinished":{"testRunHookStartedId":"r","result":{"status":"PASSED","duration":{"seconds":"0","nanos":10}},"timestamp":{"seconds":"2","nanos":0}}}
{"testRunFinished":{"success":true}}`, line)
		if err := os.WriteFile(file, []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	config := writeFile(t, `{"expectedScenarios":2,"gates":{"incomplete":true}}`)
	out, stderr, code := command(t, "-enhanced", "-format", "messages", "-run-config", config, "-o", dir, files[0], files[1])
	if code != 0 {
		t.Fatalf("sharded outlines/hooks rejected: %d %s", code, stderr)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(reportPath(t, out)), "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary report.RunSummary
	if err = json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.ObservedScenarios != 2 || summary.RunHooks != 2 || len(summary.Scenarios) != 4 {
		t.Fatalf("lost execution records: %+v", summary)
	}
	ids := map[string]bool{}
	for _, s := range summary.Scenarios {
		if ids[s.Identity] {
			t.Fatal("identity collision")
		}
		ids[s.Identity] = true
		if len(s.Attempts) != 1 {
			t.Fatal("cross-shard attempts mixed")
		}
	}
}

func TestLegacyOutlineInputWorksInBothModes(t *testing.T) {
	dir := newRun(t, []byte(`[{"uri":"outline.feature","elements":[{"type":"background","name":"setup","line":2,"steps":[{"result":{"status":"passed"}}]},{"type":"scenario","keyword":"Scenario Outline","name":"same name","line":10,"steps":[{"result":{"status":"passed"}}]},{"type":"background","name":"setup","line":2,"steps":[{"result":{"status":"passed"}}]},{"type":"scenario","keyword":"Scenario Outline","name":"same name","line":11,"steps":[{"result":{"status":"passed"}}]}]}]`))
	for _, args := range [][]string{{dir}, {"-enhanced", dir}} {
		if _, stderr, code := command(t, args...); code != 0 {
			t.Fatalf("%v: exit %d %s", args, code, stderr)
		}
	}
}

// Reproduction from the release review: the failure lives only on the Background.
func TestEnhancedBackgroundFailure(t *testing.T) {
	data, err := os.ReadFile("../../testdata/synthetic/background-failure.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := newRun(t, data)
	out, stderr, code := command(t, "-enhanced", "-fail-on-unexpected", dir)
	if code != 3 {
		t.Fatalf("lost Background failure: exit %d %s", code, stderr)
	}
	doc, err := os.ReadFile(reportPath(t, out))
	if err != nil || !strings.Contains(string(doc), "BG BOOM") {
		t.Fatalf("missing Background evidence: %v", err)
	}
	summaryData, err := os.ReadFile(filepath.Join(filepath.Dir(reportPath(t, out)), "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary report.RunSummary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.ObservedScenarios != 2 || summary.Counts["failed"] != 1 || summary.Counts["passed"] != 1 || summary.Scenarios[0].Duration != 6 {
		t.Fatalf("Background accounting: %+v", summary)
	}
}

func TestRunHookIDsIgnoreInputDirectory(t *testing.T) {
	input := `{"pickle":{"id":"p","uri":"ordinary.feature","name":"one","steps":[{"id":"ps","text":"accepted"}]}}
{"testCase":{"id":"tc","pickleId":"p","testSteps":[{"id":"ts","pickleStepId":"ps"}]}}
{"testCaseStarted":{"id":"a","testCaseId":"tc","attempt":0,"timestamp":{"seconds":"1","nanos":0}}}
{"testStepFinished":{"testCaseStartedId":"a","testStepId":"ts","testStepResult":{"status":"PASSED","duration":{"seconds":"0","nanos":1}}}}
{"testCaseFinished":{"testCaseStartedId":"a","timestamp":{"seconds":"2","nanos":0}}}
{"hook":{"id":"h","name":"BeforeAll"}}
{"testRunHookStarted":{"id":"r","hookId":"h","timestamp":{"seconds":"1","nanos":0}}}
{"testRunHookFinished":{"testRunHookStartedId":"r","result":{"status":"PASSED","duration":{"seconds":"0","nanos":1}},"timestamp":{"seconds":"2","nanos":0}}}
{"testRunFinished":{"success":true}}`
	var baseline, hookID, identity string
	parent := t.TempDir()
	for _, name := range []string{"2026-11-06", "ordinary", "2026-11-07"} {
		dir := filepath.Join(parent, name, "stable-shard")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cucumber.ndjson"), []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		args := []string{"-format", "messages"}
		if baseline != "" {
			args = append(args, "-baseline", baseline)
		}
		args = append(args, dir)
		out, stderr, code := command(t, args...)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		summaryFile := filepath.Join(filepath.Dir(reportPath(t, out)), "summary.json")
		data, err := os.ReadFile(summaryFile)
		if err != nil {
			t.Fatal(err)
		}
		var summary report.RunSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			t.Fatal(err)
		}
		for _, sc := range summary.Scenarios {
			if !sc.RunHook {
				if !strings.HasPrefix(sc.ID, "ordinary-L0-") {
					t.Fatalf("feature ID changed: %s", sc.ID)
				}
				continue
			}
			if hookID == "" {
				hookID, identity = sc.ID, sc.Identity
			}
			if !strings.HasPrefix(sc.ID, "run-hooks-") || sc.ID != hookID || sc.Identity != identity {
				t.Fatalf("unstable hook: %+v", sc)
			}
		}
		if summary.Comparison != nil && (len(summary.Comparison.Added) > 0 || len(summary.Comparison.Missing) > 0) {
			t.Fatalf("false baseline changes: %+v", summary.Comparison)
		}
		baseline = summaryFile
	}
}

func TestMalformedJSONKeysWriteNoArtifacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/synthetic/malformed-redaction.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture []map[string]any
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	step := fixture[0]["elements"].([]any)[0].(map[string]any)["steps"].([]any)[0].(map[string]any)
	embeddings := step["embeddings"].([]any)
	doc := step["doc_string"]
	config := writeFile(t, `{"redaction":{"patterns":["sk_live_\\w+"],"jsonKeys":["password"]}}`)
	for _, variant := range []string{"attachment", "ndjson", "doc string", "combined"} {
		t.Run(variant, func(t *testing.T) {
			delete(step, "embeddings")
			delete(step, "doc_string")
			switch variant {
			case "attachment":
				step["embeddings"] = embeddings[:1]
			case "ndjson":
				step["embeddings"] = embeddings[2:]
			case "doc string":
				step["doc_string"] = doc
			case "combined":
				step["embeddings"] = embeddings
				step["doc_string"] = doc
			}
			input, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			dir := newRun(t, input)
			parent := t.TempDir()
			stdout, stderr, code := command(t, "-enhanced", "-bundle", "-run-config", config, "-o", parent, dir)
			if code != 1 || stdout != "" || !strings.Contains(stderr, "cannot apply jsonKeys to malformed JSON") {
				t.Fatalf("redaction did not fail closed: %d %s %s", code, stdout, stderr)
			}
			for _, secret := range []string{"hunter2", "docpass1", "ndpass2", "sk_live_NAME", "sk_live_ERR"} {
				if strings.Contains(stderr, secret) {
					t.Fatal("diagnostic leaked evidence")
				}
			}
			files, err := os.ReadDir(parent)
			if err != nil || len(files) != 0 {
				t.Fatalf("failed redaction wrote artifacts, including possible bundle: %v %v", files, err)
			}
		})
	}
}
