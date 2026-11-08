package cucumber

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPublicParsersHaveConcreteComparableModels(t *testing.T) {
	raw := `[{"uri":"sample.feature","elements":[{"name":"sample","before":[{"result":{"status":"failed","error_message":"setup"}}],"steps":[{"result":{"status":"skipped"}}]}]}]`
	for _, parse := range []func(*strings.Reader, Limits) ([]Feature, error){func(r *strings.Reader, l Limits) ([]Feature, error) { return Parse(r, l) }, func(r *strings.Reader, l Limits) ([]Feature, error) { return ParseStream(r, l) }, func(r *strings.Reader, l Limits) ([]Feature, error) { return ParseEnhanced(r, l) }} {
		features, err := parse(strings.NewReader(raw), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if features[0].Elements[0].Status() != StatusFailed {
			t.Fatal("failed hook lost")
		}
		for _, v := range []any{features[0], features[0].Elements[0], features[0].Elements[0].Steps[0]} {
			typ := reflect.TypeOf(v)
			for i := 0; i < typ.NumField(); i++ {
				if !typ.Field(i).IsExported() {
					t.Fatalf("private bookkeeping leaked into %v", typ)
				}
			}
		}
	}
}

func TestPublicOutcomeAndValidationContract(t *testing.T) {
	for _, status := range []Status{StatusPassed, StatusFailed, StatusUndefined, StatusAmbiguous, StatusPending, StatusSkipped} {
		sc := Scenario{Steps: []Step{{Result: Result{Status: status}}}}
		f := []Feature{{Elements: []Scenario{sc}}}
		if sc.Status() != status || f[0].Status() != status || RunStatus(f) != status {
			t.Fatalf("outcome disagreement for %s", status)
		}
		if e := Validate(f); e != nil {
			t.Fatal(e)
		}
	}
	if Combine(StatusPassed, StatusFailed) != StatusFailed || Combine(StatusPending, StatusSkipped) != StatusSkipped {
		t.Fatal("aggregation disagrees")
	}
	if RunStatus(nil) != StatusUndefined || (Scenario{}).Status() != StatusUndefined {
		t.Fatal("empty execution became green")
	}
	bad := []Feature{{Elements: []Scenario{{Steps: []Step{{Result: Result{Status: "exploded"}}}}}}}
	if Validate(bad) == nil {
		t.Fatal("invalid status accepted")
	}
}

func TestPublicCapturedRetryAndMessages(t *testing.T) {
	for _, name := range []string{"retryfinal", "flakyonly"} {
		raw, e := os.ReadFile("../testdata/captured/ruby-9.2.1-r12/" + name + "/run.json")
		if e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		want := StatusPassed
		if name == "retryfinal" {
			want = StatusFailed
		}
		if got := RunStatus(f); got != want {
			t.Fatalf("%s: %s", name, got)
		}
		raw, e = os.ReadFile("../testdata/captured/ruby-9.2.1-r12/" + name + "/run.ndjson")
		if e != nil {
			t.Fatal(e)
		}
		run, e := ParseMessages(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(run.Attempts) < 2 {
			t.Fatal("retry evidence missing")
		}
		for _, a := range run.Attempts {
			if !a.Scenario.Status().Valid() {
				t.Fatal("invalid public execution status")
			}
		}
	}
}

func TestPublicParseErrorOwnership(t *testing.T) {
	if reflect.TypeFor[ParseError]().PkgPath() != "github.com/qatoolist/cucumber-reporter/cucumber" {
		t.Fatal("ParseError is not publicly owned")
	}
	_, err := Parse(strings.NewReader(`[{"elements":[{"steps":[{"result":{"status":"invalid"}}]}]}]`), Limits{})
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Unwrap() == nil {
		t.Fatalf("public parse error missing: %T %v", err, err)
	}
}
