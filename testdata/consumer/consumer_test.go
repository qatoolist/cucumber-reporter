package consumer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestPublicModel(t *testing.T) {
	raw := `[{"uri":"check.feature","elements":[{"name":"example","steps":[{"name":"go","result":{"status":"passed"}}]}]}]`
	a, e := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := cucumber.Parse(strings.NewReader(raw), cucumber.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if d := cmp.Diff(a, b); d != "" {
		t.Fatal(d)
	}
	if fmt.Sprintf("%T", a[0]) != "cucumber.Feature" {
		t.Fatalf("wrong type: %T", a[0])
	}
	encoded, e := json.Marshal(a)
	if e != nil {
		t.Fatal(e)
	}
	var back []cucumber.Feature
	if e = json.Unmarshal(encoded, &back); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a, back) {
		t.Fatal("model JSON round trip changed")
	}
	for _, name := range []string{"Feature", "Scenario.Status", "Embedding.Decode"} {
		out, e := exec.Command("go", "doc", "github.com/qatoolist/cucumber-reporter/cucumber", name).CombinedOutput()
		if e != nil {
			t.Fatalf("go doc %s: %s: %v", name, out, e)
		}
	}
}

func TestPublicOptionsAndMessages(t *testing.T) {
	if d := cmp.Diff(report.Options{Version: 2}, report.Options{Version: 2}); d != "" {
		t.Fatal(d)
	}
	raw, err := os.ReadFile("../captured/cucumber-ruby-9.2.1-retry/retry.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	a, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(a, b); d != "" {
		t.Fatal(d)
	}
	for _, name := range []string{"MessageRun", "MessageAttempt"} {
		out, err := exec.Command("go", "doc", "github.com/qatoolist/cucumber-reporter/cucumber", name).CombinedOutput()
		if err != nil || !bytes.Contains(out, []byte("struct")) {
			t.Fatalf("missing defined type docs: %s %v", out, err)
		}
	}
}

func TestCopiedMessagesKeepIdentityContext(t *testing.T) {
	raw, err := os.ReadFile("../captured/ruby-9.2.1-r12/shapes/run.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	run, err := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	prepare := func(attempts []cucumber.MessageAttempt) *report.PreparedRun {
		t.Helper()
		p, err := report.PrepareRun([]report.Input{{Name: "copy", Attempts: attempts}}, report.Options{Version: 2}, report.RunConfig{ID: "same-execution"})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	full := prepare(run.Attempts)
	if diff := cmp.Diff(full, prepare(run.Attempts)); diff != "" {
		t.Fatal(diff)
	}
	copied := prepare(append([]cucumber.MessageAttempt(nil), run.Attempts...))
	if diff := cmp.Diff(full.Summary.Scenarios, copied.Summary.Scenarios); diff != "" {
		t.Fatal(diff)
	}
	for _, attempt := range run.Attempts {
		one := prepare([]cucumber.MessageAttempt{attempt})
		found := false
		for _, original := range full.Summary.Scenarios {
			if original.Identity == one.Summary.Scenarios[0].Identity {
				found = true
			}
		}
		if !found {
			t.Fatal("subset lost full-document identity context")
		}
	}
}

func TestPreparedRunValuesAndParseErrors(t *testing.T) {
	f, err := cucumber.Parse(strings.NewReader(`[{"uri":"equal.feature","elements":[{"name":"example","line":1,"steps":[{"name":"go","result":{"status":"passed"}}]}]}]`), cucumber.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	prepare := func(id string) *report.PreparedRun {
		t.Helper()
		p, e := report.PrepareRun([]report.Input{{Name: "one", Features: f}}, report.Options{Version: 2}, report.RunConfig{ID: id, RecordedAt: "2026-11-07T00:00:00Z"})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	a, b := prepare("same"), prepare("same")
	if diff := cmp.Diff(*a, *b); diff != "" {
		t.Fatal(diff)
	}
	if diff := cmp.Diff(a, b); diff != "" {
		t.Fatal(diff)
	}
	if prepare("").Equal(*prepare("")) {
		t.Fatal("generated run IDs unexpectedly equal")
	}
	_, err = report.Render([]cucumber.Feature{{Elements: []cucumber.Scenario{{Steps: []cucumber.Step{{Result: cucumber.Result{Status: "invalid"}}}}}}}, report.Options{Version: 2})
	var pe *cucumber.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("public error unavailable: %T %v", err, err)
	}
	if reflect.TypeFor[cucumber.ParseError]().PkgPath() != "github.com/qatoolist/cucumber-reporter/cucumber" {
		t.Fatal("internal alias escaped")
	}
}
