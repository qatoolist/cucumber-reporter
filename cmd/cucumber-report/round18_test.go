package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// K5-M1: a known issue excluded by its issue policy (other environment, expired
// exception, non-matching error signature) must not rescue a real failing
// capture; the verdict follows cucumber's exit code.
func TestRound18IssuePolicyExclusionKeepsCucumberExit(t *testing.T) {
	caps, err := filepath.Glob("../../testdata/round16/F3/cap/k2[2-5]-*")
	if err != nil || len(caps) != 4 {
		t.Fatalf("captures: %v %v", caps, err)
	}
	policies := map[string]map[string]any{
		"environment": {"environment": "prod", "issues": []any{map[string]any{"ref": "K", "environments": []string{"staging"}}}},
		"expired":     {"recordedAt": "2026-03-02T10:00:00Z", "issues": []any{map[string]any{"ref": "K", "expires": "2026-03-01"}}},
		"signature":   {"issues": []any{map[string]any{"ref": "K", "errorPattern": "^not this error$"}}},
	}
	for _, dir := range caps {
		summary, err := os.ReadFile(filepath.Join(dir, "summary.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(summary), "exit=1") {
			t.Fatalf("%s: cucumber did not fail", dir)
		}
		for name, policy := range policies {
			for _, format := range []string{"json", "ndjson"} {
				t.Run(filepath.Base(dir)+"/"+name+"/"+format, func(t *testing.T) {
					cfg := map[string]any{}
					for k, v := range policy {
						cfg[k] = v
					}
					args := []string{"-enhanced", "-fail-on-unexpected", "-o", t.TempDir(), "-options", "../../testdata/round16/F3/adv/knownz.opt"}
					if format == "ndjson" {
						args = append(args, "-format", "messages")
					} else if strings.Contains(string(summary), "--strict-flaky") {
						cfg["gates"] = map[string]bool{"flaky": true}
					} else if strings.Contains(string(summary), "--strict") {
						cfg["gates"] = map[string]bool{"pending": true, "undefined": true}
					}
					b, err := json.Marshal(cfg)
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(t.TempDir(), "run-config.json")
					if err = os.WriteFile(path, b, 0o600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "-run-config", path, filepath.Join(dir, "run."+format))
					_, stderr, code := command(t, args...)
					if code != 3 || strings.Contains(stderr, "the report was refused") {
						t.Fatalf("wanted cucumber failure exit 3, got %d: %s", code, stderr)
					}
				})
			}
		}
	}
}

// K5-L1: identical arguments fall back to the base name (absolute argument) or
// the cleaned relative path, never to an absolute workstation path.
func TestRound18RepeatedAbsoluteInputIsNotPublished(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"/abs/dg/r.json", "r.json"},
		{"dg/./r.json", "dg/r.json"},
	} {
		if got := shortestInputName(tc.name, []string{tc.name, tc.name}); got != tc.want {
			t.Errorf("%q twice = %q, want %q", tc.name, got, tc.want)
		}
	}
	root := writeTree(t, map[string]string{"dg/r.json": shardRun("features/x.feature")})
	file := filepath.Join(root, "dg", "r.json")
	_, stderr, code := command(t, "-validate", "-enhanced", file, file)
	if code != 1 || !strings.Contains(stderr, `input "r.json" given twice`) || strings.Contains(stderr, filepath.ToSlash(strings.TrimPrefix(filepath.Dir(file), "/"))) {
		t.Fatalf("repeated absolute input: %d %s", code, stderr)
	}
}
