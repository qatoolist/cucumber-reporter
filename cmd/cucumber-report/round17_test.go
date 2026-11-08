package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F4-N1: an annotation whose stepText does not match the failing step must not
// make a real failing capture green; the verdict follows cucumber's exit code.
func TestRound17KnownWrongStepTextKeepsCucumberExit(t *testing.T) {
	caps, err := filepath.Glob("../../testdata/round16/F3/cap/k2[2-5]-*")
	if err != nil || len(caps) != 4 {
		t.Fatalf("captures: %v %v", caps, err)
	}
	for _, dir := range caps {
		summary, err := os.ReadFile(filepath.Join(dir, "summary.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(summary), "exit=1") {
			t.Fatalf("%s: cucumber did not fail", dir)
		}
		for _, format := range []string{"json", "ndjson"} {
			t.Run(filepath.Base(dir)+"/"+format, func(t *testing.T) {
				args := []string{"-enhanced", "-fail-on-unexpected", "-o", t.TempDir(), "-options", "../../testdata/round16/F3/adv/knownwrong.opt"}
				if format == "ndjson" {
					args = append(args, "-format", "messages")
				} else if strings.Contains(string(summary), "--strict-flaky") {
					args = append(args, "-run-config", "../../testdata/round16/F3/adv/sflaky.json")
				} else if strings.Contains(string(summary), "--strict") {
					args = append(args, "-run-config", "../../testdata/round16/F3/adv/strict.json")
				}
				args = append(args, filepath.Join(dir, "run."+format))
				_, stderr, code := command(t, args...)
				if code != 3 || strings.Contains(stderr, "reporter bug") {
					t.Fatalf("wanted cucumber failure exit 3, got %d: %s", code, stderr)
				}
			})
		}
	}
}
