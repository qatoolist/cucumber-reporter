package report

import (
	"bytes"
	"os"
	"testing"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestCapturedRubyRetryFormats(t *testing.T) {
	var summaries []RunSummary
	for _, format := range []string{"json", "ndjson"} {
		raw, err := os.ReadFile("../testdata/captured/cucumber-ruby-9.2.1-retry/retry." + format)
		if err != nil {
			t.Fatal(err)
		}
		in := runInput{Name: "ruby"}
		if format == "json" {
			in.Features, err = cucumber.ParseEnhanced(bytes.NewReader(raw), cucumber.Limits{})
			plain, e := cucumber.Parse(bytes.NewReader(raw), cucumber.Limits{})
			if e != nil {
				t.Fatal(e)
			}
			opt, _ := normalize(renderOptions{Version: 2})
			page, e := buildPage(plain, opt, nil)
			if e != nil {
				t.Fatal(e)
			}
			counts := map[string]int{}
			for _, f := range page.Features {
				for _, s := range f.Elements {
					counts[s.Status]++
				}
			}
			if counts["failed"] != 1 || counts["passed"] != 3 {
				t.Fatalf("plain: %v", counts)
			}
		} else {
			var m cucumber.MessageRun
			m, err = cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
			in.Attempts = m.Attempts
		}
		if err != nil {
			t.Fatal(err)
		}
		p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{Gates: GatePolicy{Unexpected: true}})
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if p.Summary.Counts["failed"] != 1 || p.Summary.Counts["passed"] != 3 || len(p.Summary.GateFailures) != 1 {
			t.Fatalf("%s: %+v", format, p.Summary)
		}
		for _, s := range p.Summary.Scenarios {
			if s.Name == "Flaky payment" && (!s.Retried || s.FirstStatus != "failed" || s.Status != "passed" || len(s.Attempts) != 2) {
				t.Fatalf("%s: flaky final: %+v", format, s)
			}
			if s.Name == "Always fails" && (s.Status != "failed" || len(s.Attempts) != 3) {
				t.Fatalf("%s: repeated failure: %+v", format, s)
			}
		}
		summaries = append(summaries, p.Summary)
	}
	for i, s := range summaries[0].Scenarios {
		other := summaries[1].Scenarios[i]
		if s.Identity != other.Identity || s.Status != other.Status {
			t.Fatalf("format disagreement: %+v / %+v", s, other)
		}
	}
}
