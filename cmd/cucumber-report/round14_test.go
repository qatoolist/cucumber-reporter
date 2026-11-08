package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRound14CrossInputRerunsRefused(t *testing.T) {
	paths := []string{"../../testdata/round14/shards/chrome/cucumber.json", "../../testdata/round14/shards/firefox/cucumber.json"}
	for _, validate := range []bool{false, true} {
		args := []string{"-enhanced", "-o", t.TempDir()}
		if validate {
			args = append(args, "-validate")
		}
		args = append(args, paths...)
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 1 || !strings.Contains(err.String(), "not supported in v0.3.0") {
			t.Fatalf("%d: %s", code, err.String())
		}
	}
	var out, err bytes.Buffer
	if code := run([]string{"-reruns", paths[0]}, &out, &err); code != 2 {
		t.Fatalf("removed flag accepted: %d", code)
	}
}

func TestRound14StrictMessagesDiagnostics(t *testing.T) {
	_, err, code := command(t, "-validate", "-strict", "-enhanced", "-format", "messages", "../../testdata/round13/captured-ruby-9.2.1/around-fail-msgonly/run.ndjson")
	if code != 1 || !strings.Contains(err, "strict validation rejected run diagnostics") {
		t.Fatalf("%d %s", code, err)
	}
}

func TestRound14KnownIssueDoesNotAbsorbHook(t *testing.T) {
	_, err, code := command(t, "-enhanced", "-fail-on-unexpected", "-options", "../../testdata/round14/known-hook/options.json", "-o", t.TempDir(), "../../testdata/round14/known-hook/cucumber.json")
	if code != 3 {
		t.Fatalf("%d %s", code, err)
	}
}

func TestRemovedRerunsConfigurationRefused(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "run.json")
	if e := os.WriteFile(cfg, []byte(`{"reruns":true}`), 0o600); e != nil {
		t.Fatal(e)
	}
	_, err, code := command(t, "-run-config", cfg, "../../testdata/round14/shards/chrome/cucumber.json")
	if code != 1 || !strings.Contains(err, "invalid run configuration") {
		t.Fatalf("removed config: %d %s", code, err)
	}
	out, _, code := command(t, "-h")
	if code != 0 || strings.Contains(out, "-reruns") {
		t.Fatalf("removed flag in help: %d %s", code, out)
	}
}

func TestRound15AllCrossInputModesRefuse(t *testing.T) {
	for _, fixture := range []struct{ name, dir, first, second string }{
		{"Q2", "round14/captured/a1-rerun-allpass", "run1", "run2"},
		{"matrix", "round15/captured/m1-matrix-browser", "chrome", "firefox"},
		{"matrix-retry", "round15/captured/m2-matrix-retry", "chrome", "firefox"},
		{"unrelated-signal", "round15/runner-signal", "r1", "r2"},
	} {
		for _, format := range []string{"json", "ndjson"} {
			if fixture.name == "unrelated-signal" && format == "json" {
				continue
			} // This reviewer probe is Messages-only.
			for _, validation := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/validate=%v", fixture.name, format, validation), func(t *testing.T) {
					args := []string{"-enhanced", "-o", t.TempDir()}
					if validation {
						args = append(args, "-validate")
					}
					if format == "ndjson" {
						args = append(args, "-format", "messages")
					}
					base := "../../testdata/" + fixture.dir + "/"
					args = append(args, base+fixture.first+"."+format, base+fixture.second+"."+format)
					_, diagnostic, code := command(t, args...)
					if code != 1 || !strings.Contains(diagnostic, "merging reruns or matrix cells across inputs is not supported in v0.3.0") || !strings.Contains(diagnostic, fixture.first) || !strings.Contains(diagnostic, fixture.second) {
						t.Fatalf("%d: %s", code, diagnostic)
					}
				})
			}
		}
	}
}
