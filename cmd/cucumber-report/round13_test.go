package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/report"
)

func TestRound13SingleMessageScopeBaseline(t *testing.T) {
	baseline := writeFile(t, `{"version":4,"id":"baseline","counts":{},"scenarios":[]}`)
	config := writeFile(t, `{}`)
	_, stderr, code := command(t, "-enhanced", "-format", "messages", "-run-config", config, "-baseline", baseline, "-o", t.TempDir(), "../../testdata/round13/scope.ndjson")
	if code != 0 {
		t.Fatalf("empty explicit scope refused runner facts: %d %s", code, stderr)
	}
}

func TestRound13StrictFailingMessages(t *testing.T) {
	_, stderr, code := command(t, "-validate", "-strict", "-format", "messages", "../../testdata/captured/ruby-9.2.1-r12/statuses/run.ndjson")
	if code != 0 {
		t.Fatalf("ordinary runner failure rejected as invalid input: %d %s", code, stderr)
	}
}

func TestRound13GateArtifactsAgree(t *testing.T) {
	config := writeFile(t, `{"gates":{"pending":true}}`)
	output, stderr, code := command(t, "-enhanced", "-run-config", config, "-o", t.TempDir(), "../../testdata/captured/ruby-9.2.1-r12/pendonly/run.json")
	if code != 3 {
		t.Fatalf("gate: %d %s", code, stderr)
	}
	dir := filepath.Dir(reportPath(t, output))
	raw, e := os.ReadFile(filepath.Join(dir, "summary.json"))
	if e != nil {
		t.Fatal(e)
	}
	var summary report.RunSummary
	if e = json.Unmarshal(raw, &summary); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"index.html", "summary.md"} {
		b, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(b), summary.Computed.Label) {
			t.Fatalf("%s missing computed verdict %s", name, summary.Computed.Label)
		}
	}
	rerun, e := os.ReadFile(filepath.Join(dir, "rerun.txt"))
	if e != nil || len(rerun) == 0 {
		t.Fatalf("gate rerun missing: %v", e)
	}
}

func TestRound13VersionOverride(t *testing.T) {
	previous := developmentVersion
	t.Cleanup(func() { developmentVersion = previous })
	developmentVersion = "9.8.7"
	if got := versionFromModule("(devel)"); got != "9.8.7" {
		t.Fatalf("release override ignored: %s", got)
	}
}
