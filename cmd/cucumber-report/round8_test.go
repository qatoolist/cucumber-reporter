package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRound8StepHooksCLI(t *testing.T) {
	for _, hooks := range []string{`"after":[]`, `"after":null`, `"after":[{"match":{"location":"Hooks.capture()"},"result":{"status":"passed"},"embeddings":[{"mime_type":"image/png","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="}]}]`} {
		raw := `[{"uri":"jvm.feature","elements":[{"type":"scenario","name":"pay","line":3,"steps":[{"name":"pay","result":{"status":"passed"},` + hooks + `}]}]}]`
		out, stderr, code := command(t, "-enhanced", "-fail-on-unexpected", newRun(t, []byte(raw)))
		if code != 0 {
			t.Fatalf("%s: %d %s", hooks, code, stderr)
		}
		if strings.Contains(hooks, "embeddings") {
			html, _ := os.ReadFile(reportPath(t, out))
			if !strings.Contains(string(html), "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=") {
				t.Fatal("step-hook evidence dropped")
			}
		}
	}
	raw := `[{"uri":"jvm.feature","elements":[{"type":"scenario","name":"pay","line":3,"steps":[{"name":"pay","before":[{"match":{"location":"Hooks.beforeStep()"},"result":{"status":"failed","error_message":"BEFORE STEP BOOM"}}],"result":{"status":"skipped"}}]}]}]`
	out, stderr, code := command(t, "-enhanced", "-fail-on-unexpected", newRun(t, []byte(raw)))
	if code != 3 {
		t.Fatalf("step hook ignored: %d %s", code, stderr)
	}
	html, _ := os.ReadFile(reportPath(t, out))
	if !strings.Contains(string(html), "BEFORE STEP BOOM") {
		t.Fatal("hook error missing")
	}
}

func TestRound8MissingShardGateCLI(t *testing.T) {
	input := newRun(t, []byte(`[{"uri":"present.feature","elements":[{"name":"pay","line":3,"steps":[{"result":{"status":"passed"}}]}]}]`))
	cfg := writeFile(t, `{"expectedShards":["`+filepath.Base(input)+`","missing"],"metadata":{"absent-L3":{"identity":"absent"}},"gates":{"incomplete":true}}`)
	out, stderr, code := command(t, "-enhanced", "-run-config", cfg, input)
	if code != 3 {
		t.Fatalf("want completeness gate, got %d %s", code, stderr)
	}
	if _, err := os.Stat(reportPath(t, out)); err != nil {
		t.Fatal(err)
	}
}

func TestRound8SafeMigrationErrors(t *testing.T) {
	input := newRun(t, []byte(`[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]}]`))
	for _, tc := range []struct{ flag, body, want string }{{"-baseline", `{"version":2,"id":"SECRET"}`, "-baseline: summary schema 2"}, {"-history", `[{"version":1,"id":"SECRET"}]`, "-history: summary schema 1"}, {"-options", `{"version":2,"bundleAttachments":true}`, "use -enhanced -bundle"}} {
		_, stderr, code := command(t, "-enhanced", tc.flag, writeFile(t, tc.body), input)
		if code != 1 || !strings.Contains(stderr, tc.want) || strings.Contains(stderr, "SECRET") {
			t.Fatalf("unsafe or unhelpful error: %d %s", code, stderr)
		}
	}
	raw := `[{"elements":[{"before":[{"result":{"status":"invented","error_message":"SECRET"}}],"steps":[]}]}]`
	_, stderr, code := command(t, "-enhanced", "-fail-on-unexpected", newRun(t, []byte(raw)))
	if code != 3 || !strings.Contains(stderr, "Quality gate") || strings.Contains(stderr, "SECRET") {
		t.Fatalf("hook cause lost: %d %s", code, stderr)
	}
}

func TestRound8PDFIntegrationRequiresEngineInCI(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCompressPDFCommand$")
	cmd.Env = append(os.Environ(), "CI=true", "GS="+filepath.Join(t.TempDir(), "missing-gs"))
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Ghostscript is required") {
		t.Fatalf("PDF CLI integration skipped in CI: %s %v", out, err)
	}
}
