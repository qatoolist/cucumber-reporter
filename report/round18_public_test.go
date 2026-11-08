package report_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/report"
)

// K5-L3: caller-supplied Input.Diagnostics are cleaned, bounded and cannot
// forge the reserved runner diagnostic.
func TestRound18InputDiagnosticsAreValidated(t *testing.T) {
	f := round17Parse(t, round17JSON)
	prepare := func(ds []report.Diagnostic) (*report.PreparedRun, error) {
		return report.PrepareRun([]report.Input{{Name: "a", Features: f, Diagnostics: ds}}, round17Options, report.RunConfig{})
	}
	for _, path := range []string{"runner.outcome", " Runner.Outcome ", "/home/secret/path", `C:\secret\path`} {
		if _, err := prepare([]report.Diagnostic{{Path: path, Message: "forged"}}); !errors.Is(err, report.ErrInvalidOptions) {
			t.Fatalf("diagnostic path %q accepted: %v", path, err)
		}
	}
	msg := "</script>\u202e\x1b[31mred\nnext" + strings.Repeat("A", 1<<20)
	p, err := prepare([]report.Diagnostic{{Path: "input[0].feature[0]", Message: msg}})
	if err != nil {
		t.Fatal(err)
	}
	got := p.Summary.Diagnostics[len(p.Summary.Diagnostics)-1].Message
	if len(got) > 2048+len("…") || strings.ContainsAny(got, "\u202e\x1b\n") || !strings.HasPrefix(got, `</script>\u202ered next`) {
		t.Fatalf("diagnostic message not cleaned or bounded: %d %.80q", len(got), got)
	}
	var many []report.Diagnostic
	for i := 0; i < 500; i++ {
		many = append(many, report.Diagnostic{Path: fmt.Sprintf("input[0].feature[%d]", i), Message: "note"})
	}
	p, err = prepare(many)
	if err != nil {
		t.Fatal(err)
	}
	last := p.Summary.Diagnostics[len(p.Summary.Diagnostics)-1]
	if len(p.Summary.Diagnostics) > 201 || last.Path != "input.diagnostics" || !strings.Contains(last.Message, "300 more") {
		t.Fatalf("diagnostic count not bounded: %d %+v", len(p.Summary.Diagnostics), last)
	}
}
