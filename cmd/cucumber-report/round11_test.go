package main

import (
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

func TestMessageFactsPrecedenceAndIntervals(t *testing.T) {
	opt := report.Options{Version: 2}
	cfg := report.RunConfig{Branch: "explicit", Revision: "explicit-revision"}
	runs := []cucumber.MessageRun{{Branch: "runner", Revision: "runner-revision", Started: "2026-11-07T10:00:00Z", Finished: "2026-11-07T10:00:02Z", Remote: "repository"}}
	applyMessageFacts(runs, &opt, &cfg)
	if cfg.Branch != "explicit" || cfg.Revision != "explicit-revision" || opt.WallClock != "2s" || len(cfg.Phases) != 1 || len(opt.RunFacts) != 3 {
		t.Fatalf("facts: %+v %+v", opt, cfg)
	}
	opt = report.Options{}
	cfg = report.RunConfig{}
	applyMessageFacts(append(runs, cucumber.MessageRun{Branch: "other", Revision: "other"}), &opt, &cfg)
	if cfg.Branch != "" || cfg.Revision != "" || opt.WallClock != "" {
		t.Fatal("conflicting/incomplete stream metadata inferred")
	}
}

func TestCapturedRubyCLI(t *testing.T) {
	for _, format := range []string{"json", "messages"} {
		file := "../../testdata/captured/cucumber-ruby-9.2.1-retry/retry.json"
		if format == "messages" {
			file = "../../testdata/captured/cucumber-ruby-9.2.1-retry/retry.ndjson"
		}
		stdout, stderr, code := command(t, "-enhanced", "-validate", "-fail-on-unexpected", "-format", format, file)
		if code != 3 || !strings.Contains(stdout, "4 scenarios") || !strings.Contains(stderr, "Quality gate") {
			t.Fatalf("%s: exit %d stdout=%s stderr=%s", format, code, stdout, stderr)
		}
	}
}
