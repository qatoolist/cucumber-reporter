package report

import (
	"strings"
	"testing"
)

// R19-H1/M1: the floor checks a redacted run against the unmasked snapshot. A page
// whose known coverage relies on masked text, or on a masked policy, is refused,
// and an unmasked policy match is not.
func TestRound19FloorUsesUnmaskedEvidence(t *testing.T) {
	for _, file := range []string{"known-redaction.json", "known-redaction.ndjson"} {
		id := "a-L2"
		if strings.HasSuffix(file, ".ndjson") {
			id = "a-a8fde43a2452-L9"
		}
		for _, tc := range []struct {
			name, stepText string
			cfg            RunConfig
			refuse         bool
		}{
			{"steptext", "check secret-different", RunConfig{Redaction: &Redaction{Patterns: []string{`secret-\w+`}}}, true},
			{"environment", "check secret-original", RunConfig{Environment: "env-SECRET9", Issues: []IssuePolicy{{Ref: "K", Environments: []string{"env-SECRET2"}}}, Redaction: &Redaction{Patterns: []string{`SECRET\d`}}}, true},
			{"signature", "check secret-original", RunConfig{Issues: []IssuePolicy{{Ref: "K", ErrorPattern: "^BOOM SECRET123$"}}, Redaction: &Redaction{Patterns: []string{"SECRET123"}}}, false},
		} {
			if tc.name == "signature" && strings.HasSuffix(file, ".ndjson") {
				continue // the Messages capture's error is not the JSON fixture's signature
			}
			opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: id, Kind: KindKnown, Ref: "K", Title: "t", Step: 1, StepText: tc.stepText}}}
			plainCfg := tc.cfg
			plainCfg.Redaction = nil
			// The unredacted run gives the snapshot the production code takes.
			plain, err := preparePublicRun([]runInput{round14Input(t, "one", "../testdata/round19/"+file)}, opt, plainCfg)
			if err != nil {
				t.Fatalf("%s %s: %v", file, tc.name, err)
			}
			orig, err := snapshotEvidence(plain.Features, scenarioIDs(plain.Features), plain.Options.Annotations)
			if err != nil {
				t.Fatal(err)
			}
			p, err := preparePublicRun([]runInput{round14Input(t, "one", "../testdata/round19/"+file)}, opt, tc.cfg)
			if err != nil {
				t.Fatalf("%s %s: %v", file, tc.name, err)
			}
			// Simulate a decision that trusts masked values: drop every policy reason
			// and the carried step-text mismatch, then classify the redacted page.
			lost := p.Options
			lost.Annotations = append([]Annotation(nil), lost.Annotations...)
			for i := range lost.Annotations {
				lost.Annotations[i].PolicyReason = ""
				lost.Annotations[i].StepText = strings.TrimSuffix(lost.Annotations[i].StepText, " (differs before redaction)")
			}
			lost.Analysis = nil
			page, err := buildPage(p.Features, lost, nil)
			if err != nil {
				t.Fatal(err)
			}
			masked := tc.cfg
			masked.Environment = "env-[REDACTED]"
			masked.Issues = []IssuePolicy{{Ref: "K", Environments: []string{"env-[REDACTED]"}}}
			if tc.name != "environment" {
				masked = tc.cfg
			}
			// Without the snapshot the floor reads masked values and gets each case
			// wrong: fooled into accepting, or falsely refusing the signature match.
			if err = checkScenarioOutcomes(p.Features, page, &masked, nil); (err != nil) == tc.refuse {
				t.Fatalf("%s %s: masked floor unexpectedly right, case proves nothing: %v", file, tc.name, err)
			}
			err = checkScenarioOutcomes(p.Features, page, &plainCfg, orig)
			if tc.refuse != (err != nil && strings.Contains(err.Error(), "evidence floor")) {
				t.Fatalf("%s %s: refuse=%v got %v", file, tc.name, tc.refuse, err)
			}
		}
	}
}

// R19 re-review H1 and L2: matching the visible text adds to the raw matches and
// never replaces them; raw bytes, colors included, are kept when visible matching
// finds nothing more.
func TestRound19RedactionUnionsRawAndVisibleMatches(t *testing.T) {
	for _, tc := range []struct {
		patterns []string
		in, want string
	}{
		{[]string{`\x1b\[33m\w+`}, "pw \x1b[33mhunter2\x1b[0m", "pw [REDACTED]\x1b[0m"},
		{[]string{`TOK123`, `\x1b\[1mX\w+`}, "TOK\x1b[1m123 \x1b[1mXSECRET", "[REDACTED] [REDACTED]"},
		{[]string{`TOK123`}, "BOOM TOK\x1b[31m123\x1b[0m", "BOOM [REDACTED]"},
		{[]string{`TOK123`}, "BOOM TOK\u200b123", "BOOM [REDACTED]"},
		{[]string{`TOK123`}, "BOOM TO\u00adK123", "BOOM [REDACTED]"},
		{[]string{`hunter2`}, "x \x1b[33mhunter2\x1b[0m", "x \x1b[33m[REDACTED]\x1b[0m"},
		{[]string{`zzz`}, "a\x1b[1mb\u200bc", "a\x1b[1mb\u200bc"},
		{[]string{`secret-\w+`}, "check secret-orig\x1b[0minal", "check [REDACTED]"},
	} {
		r, err := newRedactor(&Redaction{Patterns: tc.patterns})
		if err != nil {
			t.Fatal(err)
		}
		if got := r.text(tc.in); got != tc.want {
			t.Errorf("%q %q: got %q want %q", tc.patterns, tc.in, got, tc.want)
		}
	}
}
