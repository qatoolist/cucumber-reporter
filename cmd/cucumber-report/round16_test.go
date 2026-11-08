package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRound16StrictFlakyKnown(t *testing.T) {
	for _, name := range []string{"k23-known-plus-flaky", "k24-known-z-strictflaky"} {
		// Locate the captured name rather than depending on a prose abbreviation.
		matches, err := filepath.Glob("../../testdata/round16/F3/cap/" + name[:3] + "*")
		if err != nil || len(matches) != 1 {
			t.Fatalf("capture %s: %v %v", name, matches, err)
		}
		for _, format := range []string{"json", "ndjson"} {
			for _, known := range []bool{false, true} {
				t.Run(name+"/"+format+"/known="+map[bool]string{true: "yes", false: "no"}[known], func(t *testing.T) {
					args := []string{"-enhanced", "-fail-on-unexpected", "-o", t.TempDir()}
					if format == "ndjson" {
						args = append(args, "-format", "messages")
					} else {
						cfg := filepath.Join(t.TempDir(), "cfg.json")
						if err := os.WriteFile(cfg, []byte(`{"gates":{"flaky":true}}`), 0o600); err != nil {
							t.Fatal(err)
						}
						args = append(args, "-run-config", cfg)
					}
					if known {
						args = append(args, "-options", "../../testdata/round16/F3/adv/knownz.opt")
					}
					args = append(args, matches[0]+"/run."+format)
					_, stderr, code := command(t, args...)
					if code != 3 {
						t.Fatalf("wanted cucumber failure exit 3, got %d: %s", code, stderr)
					}
				})
			}
		}
	}
}

func TestRound16RealDisjointShards(t *testing.T) {
	for _, ext := range []string{"json", "ndjson"} {
		args := []string{"-enhanced", "-fail-on-unexpected", "-o", t.TempDir()}
		if ext == "ndjson" {
			args = append(args, "-format", "messages")
		}
		args = append(args, "../../testdata/round16/G3/cap/a."+ext, "../../testdata/round16/G3/cap/b."+ext)
		stdout, stderr, code := command(t, args...)
		if code != 3 {
			t.Fatalf("legitimate shards %s: %d %s", ext, code, stderr)
		}
		b, err := os.ReadFile(filepath.Join(filepath.Dir(reportPath(t, stdout)), "summary.json"))
		if err != nil {
			t.Fatal(err)
		}
		var summary struct {
			ObservedScenarios int
			Counts            map[string]int
			Computed          struct{ Label, Tone string }
			GateFailures      []string
		}
		if err = json.Unmarshal(b, &summary); err != nil {
			t.Fatal(err)
		}
		if summary.ObservedScenarios != 9 || len(summary.Counts) != 2 || summary.Counts["failed"] != 1 || summary.Counts["passed"] != 8 ||
			summary.Computed.Label != "Failed: 1 unexpected failure" || summary.Computed.Tone != "fail" || strings.Join(summary.GateFailures, ",") != "unexpected failures" {
			t.Fatalf("shards %s: wrong verdict or counts: %s", ext, b)
		}
	}
}

func TestRound16AllStrictMessages(t *testing.T) {
	for _, name := range []string{"g3-same-strict", "strict-undefined-only", "strict-undefined-retry1", "strict-retry1", "strict-pending-retry1", "strict-flaky-retry1"} {
		matches, err := filepath.Glob("../../testdata/round*/captured*/" + name + "/run.ndjson")
		if err != nil || len(matches) == 0 {
			t.Fatalf("missing capture %s", name)
		}
		for _, file := range matches {
			t.Run(name, func(t *testing.T) {
				out := t.TempDir()
				_, stderr, code := command(t, "-enhanced", "-fail-on-unexpected", "-format", "messages", "-o", out, file)
				if code != 3 || strings.Contains(stderr, "reporter bug") {
					t.Fatalf("%d %s", code, stderr)
				}
				paths, e := filepath.Glob(filepath.Join(out, "html-*", "summary.json"))
				if e != nil || len(paths) != 1 {
					t.Fatal("missing summary")
				}
				b, e := os.ReadFile(paths[0])
				if e != nil {
					t.Fatal(e)
				}
				var summary struct {
					Computed     struct{ Tone string }
					GateFailures []string
				}
				if e = json.Unmarshal(b, &summary); e != nil {
					t.Fatal(e)
				}
				if summary.Computed.Tone != "fail" || len(summary.GateFailures) == 0 {
					t.Fatalf("missing fail tone/gates: %s", b)
				}
			})
		}
	}
}

func TestRound16RawEmptyOwnStepsAndKnownIssue(t *testing.T) {
	_, stderr, code := command(t, "-enhanced", "-format", "messages", "-fail-on-unexpected", "-options", "../../testdata/round16/independent/known.opt", "-o", t.TempDir(), "../../testdata/round16/independent/empty-with-known.ndjson")
	if code != 3 {
		t.Fatalf("known failure cannot explain hook-only empty execution: %d %s", code, stderr)
	}
}

func TestRound16StrictAnnotationMismatch(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "cucumber.json")
	opt := filepath.Join(dir, "options.json")
	if err := os.WriteFile(raw, []byte(`[{"uri":"a.feature","name":"A","elements":[{"type":"scenario","name":"S","line":1,"steps":[{"name":"actual","result":{"status":"failed"}}]}]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opt, []byte(`{"version":2,"annotations":[{"scenario":"a-L1","kind":"known","ref":"K","title":"K","step":1,"stepText":"different"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, enhanced := range []bool{false, true} {
		args := []string{"-validate", "-strict", "-options", opt}
		if enhanced {
			args = append(args, "-enhanced")
		}
		args = append(args, raw)
		_, stderr, code := command(t, args...)
		if code != 1 || !strings.Contains(stderr, "stepText") {
			t.Fatalf("mismatch enhanced=%v: %d %s", enhanced, code, stderr)
		}
	}
}

func TestRound16PolicyAndRunnerRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"reported-known", []string{"-options", "../../testdata/round15/known/k1.opt", "-run-config", "../../testdata/round15/known/n5c-rc.json", "../../testdata/round15/known/n5c.json"}},
		{"empty-policy", nil},
		{"zero-runner", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"-enhanced", "-fail-on-unexpected", "-o", t.TempDir()}
			switch tc.name {
			case "empty-policy":
				cfg := filepath.Join(t.TempDir(), "config.json")
				if err := os.WriteFile(cfg, []byte(`{"gates":{"empty":true}}`), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "-run-config", cfg, "../../testdata/round13/captured-ruby-9.2.1/zero-tags/run.json")
			case "zero-runner":
				input := filepath.Join(t.TempDir(), "empty.ndjson")
				if err := os.WriteFile(input, []byte("{\"testRunFinished\":{\"success\":false}}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "-format", "messages", input)
			default:
				args = append(args, tc.args...)
			}
			_, stderr, code := command(t, args...)
			if tc.name == "zero-runner" {
				if code != 1 || !strings.Contains(stderr, "runner reported failure without execution results") {
					t.Fatalf("expected input refusal, got %d: %s", code, stderr)
				}
			} else if code != 3 {
				t.Fatalf("expected failed gate, got %d: %s", code, stderr)
			}
		})
	}
}
