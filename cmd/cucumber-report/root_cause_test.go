package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootLegacyHooksAndBackgroundAnnotations(t *testing.T) {
	for _, hook := range []string{"before", "after"} {
		raw := `[{"uri":"h.feature","elements":[{"name":"case","line":3,"` + hook + `":[{"result":{"status":"failed","error_message":"HOOK BOOM"}}],"steps":[{"name":"go","result":{"status":"skipped"}}]}]}]`
		dir := newRun(t, []byte(raw))
		out, stderr, code := command(t, "-enhanced", "-fail-on-unexpected", dir)
		if code != 3 {
			t.Errorf("%s ignored hook: %d %s", hook, code, stderr)
			continue
		}
		html, _ := os.ReadFile(reportPath(t, out))
		rerun, _ := os.ReadFile(filepath.Join(filepath.Dir(reportPath(t, out)), "rerun.txt"))
		if !strings.Contains(string(html), "HOOK BOOM") || !strings.Contains(string(rerun), "h.feature:3") {
			t.Error("hook evidence or rerun missing")
		}
	}
	raw := `[{"uri":"pay.feature","elements":[{"type":"background","line":1,"steps":[{"name":"setup","result":{"status":"passed"}}]},{"type":"scenario","name":"pay","line":5,"before":[{"result":{"status":"passed"}}],"steps":[{"name":"pay","result":{"status":"failed"}}]}]}]`
	options := writeFile(t, `{"version":2,"annotations":[{"scenario":"pay-L5","kind":"known","ref":"KNOWN","step":1,"stepText":"pay","outcomes":[{"step":1,"outcome":"known","text":"wrong"}]}]}`)
	_, stderr, code := command(t, "-enhanced", "-fail-on-unexpected", "-options", options, newRun(t, []byte(raw)))
	if code != 0 {
		t.Fatalf("known issue shifted: %d %s", code, stderr)
	}
}

func TestRootComputedOptionsRejected(t *testing.T) {
	for _, field := range []string{`"analysis":{"gateFailures":["FAKE"]}`, `"bundleAttachments":true`} {
		options := writeFile(t, `{"version":2,`+field+`}`)
		parent := t.TempDir()
		_, _, code := command(t, "-options", options, "-o", parent, newRun(t, []byte(`[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]}]`)))
		entries, _ := os.ReadDir(parent)
		if code != 1 || len(entries) != 0 {
			t.Errorf("computed option accepted: %d %d", code, len(entries))
		}
	}
}
