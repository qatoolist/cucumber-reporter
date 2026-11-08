package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRound10PlainCLIAndValidation(t *testing.T) {
	raw := `[{"uri":"case.feature","elements":[{"type":"background","steps":[{"result":{"status":"passed"}}]},{"name":"case","line":3,"before":[{"result":{"status":"failed","error_message":"BEFORE BOOM"}}],"after":[{"result":{"status":"failed","error_message":"AFTER BOOM"}}],"steps":[{"result":{"status":"passed"}}]}]}]`
	input := newRun(t, []byte(raw))
	out, errText, code := command(t, input)
	if code != 0 {
		t.Fatal(code, errText)
	}
	html, _ := os.ReadFile(reportPath(t, out))
	for _, text := range []string{`"tone":"fail"`, "BEFORE BOOM", "AFTER BOOM"} {
		if !strings.Contains(string(html), text) {
			t.Fatal("missing plain verdict/evidence", text)
		}
	}
	out, errText, code = command(t, "-validate", input)
	if code != 0 || !strings.Contains(out, "1 scenarios; outcome failed") {
		t.Fatal(code, out, errText)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	_, errText, code = command(t, "-o", missing, input)
	if code != 1 || !strings.Contains(errText, "output directory does not exist") || strings.Contains(errText, "cucumber.json") {
		t.Fatal(code, errText)
	}
}

func TestRound10ModuleVersion(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"(devel)", developmentVersion}, {"v0.2.0+dirty", developmentVersion}, {"v0.2.1-20261107000000-abcdef123456", "0.3.0-dev (abcdef123456)"}, {"v0.3.1", "0.3.1"}, {"v0.3.0-rc.1", "0.3.0-rc.1"}} {
		if got := versionFromModule(tc.in); got != tc.want {
			t.Fatal(got)
		}
	}
}
