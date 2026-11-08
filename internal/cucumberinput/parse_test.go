package cucumber

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func readFixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "synthetic", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustParse(t *testing.T, input string) []Feature {
	t.Helper()
	features, err := Parse(strings.NewReader(input), Limits{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return features
}

// TestParseGodogOutput reads real Godog v0.16.0 formatter output for invented features
// (testdata/synthetic/godog-gen) and checks every shape it contains.
func TestParseGodogOutput(t *testing.T) {
	features, err := Parse(strings.NewReader(string(readFixture(t, "godog-v0.16.0.json"))), Limits{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(features) != 2 {
		t.Fatalf("features = %d, want 2", len(features))
	}
	billing, orders := features[0], features[1]
	if billing.URI != "features/billing.feature" || billing.Name != "Billing" || billing.Keyword != "Feature" || billing.Line != 1 {
		t.Errorf("billing header = %+v", billing)
	}
	if len(billing.Elements) != 3 || len(orders.Elements) != 3 {
		t.Fatalf("scenarios = %d, %d; want 3, 3", len(billing.Elements), len(orders.Elements))
	}
	counts := map[Status]int{}
	steps := 0
	for _, f := range features {
		for _, sc := range f.Elements {
			if sc.Type != "scenario" {
				t.Errorf("type = %q", sc.Type)
			}
			for _, st := range sc.Steps {
				steps++
				counts[st.Result.Status]++
				hasDuration := st.Result.Duration != nil
				wantDuration := st.Result.Status == StatusPassed || st.Result.Status == StatusFailed
				if hasDuration != wantDuration {
					t.Errorf("%s: duration present = %v for status %s", st.Name, hasDuration, st.Result.Status)
				}
			}
		}
	}
	if steps != 15 {
		t.Errorf("steps = %d, want 15", steps)
	}
	want := map[Status]int{"passed": 9, "failed": 2, "skipped": 1, "pending": 1, "undefined": 1, "ambiguous": 1}
	for s, n := range want {
		if counts[s] != n {
			t.Errorf("status %s = %d, want %d", s, counts[s], n)
		}
	}

	ambiguous := billing.Elements[2].Steps[0]
	if ambiguous.Result.Status != StatusAmbiguous || !strings.HasPrefix(ambiguous.Result.Error, "ambiguous step definition") {
		t.Errorf("ambiguous step = %+v", ambiguous.Result)
	}

	confirm := orders.Elements[0]
	if confirm.Name != "Confirm a draft order" || len(confirm.Tags) != 1 || confirm.Tags[0] != (Tag{Name: "@orders", Line: 1}) {
		t.Errorf("confirm scenario = %+v", confirm)
	}
	table := confirm.Steps[0]
	if len(table.Rows) != 3 || strings.Join(table.Rows[1].Cells, ",") != "A-100,2" {
		t.Errorf("rows = %+v", table.Rows)
	}
	if table.Match.Location != "main_test.go:18" {
		t.Errorf("match = %q", table.Match.Location)
	}
	if len(table.Embeddings) != 1 || table.Embeddings[0] != (Embedding{Name: "lines.txt", MimeType: "text/plain", Data: "bGluZXMgYWNjZXB0ZWQK"}) {
		t.Errorf("embeddings = %+v", table.Embeddings)
	}
	confirmed := confirm.Steps[1]
	var mimes []string
	for _, e := range confirmed.Embeddings {
		mimes = append(mimes, e.MimeType)
	}
	if strings.Join(mimes, ",") != "application/json,text/html,image/png" {
		t.Errorf("mime types = %v", mimes)
	}
	doc := confirm.Steps[2].DocString
	if doc == nil || doc.ContentType != "application/json" || doc.Value != `{"state": "confirmed"}` || doc.Line != 12 {
		t.Errorf("doc string = %+v", doc)
	}

	outline := orders.Elements[1]
	if !strings.HasSuffix(outline.ID, ";invalid-quantities;2") || outline.Line != 24 || len(outline.Tags) != 2 {
		t.Errorf("outline row = %+v", outline)
	}
	if got := outline.Steps[2].Result; got.Status != StatusFailed || got.Error != "expected rejection, got confirmation" || got.Duration == nil {
		t.Errorf("failed result = %+v", got)
	}
}

func TestStatusRules(t *testing.T) {
	st := func(statuses ...Status) Scenario {
		var sc Scenario
		for _, s := range statuses {
			sc.Steps = append(sc.Steps, Step{Result: Result{Status: s}})
		}
		return sc
	}
	tests := []struct {
		name string
		sc   Scenario
		want Status
	}{
		{"all passed", st("passed", "passed"), StatusPassed},
		{"failed then skipped", st("passed", "failed", "skipped"), StatusFailed},
		{"passed and skipped", st("passed", "skipped"), StatusSkipped},
		{"pending then skipped", st("passed", "pending", "skipped"), StatusPending},
		{"undefined", st("passed", "undefined"), StatusUndefined},
		{"ambiguous outranks undefined", st("undefined", "ambiguous"), StatusAmbiguous},
		{"failed outranks ambiguous", st("ambiguous", "failed"), StatusFailed},
	}
	for _, tt := range tests {
		if got := tt.sc.Status(); got != tt.want {
			t.Errorf("%s: Status() = %s, want %s", tt.name, got, tt.want)
		}
	}
	f := Feature{Elements: []Scenario{st("passed"), st("passed", "skipped"), st("failed")}}
	if got := f.Status(); got != StatusFailed {
		t.Errorf("feature Status() = %s, want failed", got)
	}
	for _, s := range Statuses {
		if !s.Valid() {
			t.Errorf("%s not valid", s)
		}
	}
	for _, s := range []Status{"", "Passed", "error", "skip"} {
		if s.Valid() {
			t.Errorf("%q valid", s)
		}
	}
}

func TestDurations(t *testing.T) {
	d := func(ns int64) *time.Duration { v := time.Duration(ns); return &v }
	sc := Scenario{Steps: []Step{{Result: Result{Status: "passed", Duration: d(1500)}}, {Result: Result{Status: "skipped"}}, {Result: Result{Status: "failed", Duration: d(500)}}}}
	if got := sc.Duration(); got != 2000 {
		t.Errorf("scenario Duration() = %d, want 2000", got)
	}
	f := Feature{Elements: []Scenario{sc, sc}}
	if got := f.Duration(); got != 4000 {
		t.Errorf("feature Duration() = %d, want 4000", got)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{-time.Second, "0s"},
		{400 * time.Microsecond, "<1ms"},
		{500 * time.Microsecond, "1ms"},
		{32 * time.Microsecond, "<1ms"},
		{40 * time.Millisecond, "40ms"},
		{999 * time.Millisecond, "999ms"},
		{999600 * time.Microsecond, "1000ms"},
		{time.Second, "1s"},
		{1160 * time.Millisecond, "1s"},
		{61 * time.Second, "1m 1s"},
		{time.Hour, "1h"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1h 2m 3s"},
		{26*time.Hour + time.Minute + time.Second, "1d 2h 1m 1s"},
		{48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.d); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	const step = `{"keyword":"Given ","name":"s","line":3,"match":{"location":"x.go:1"},"result":{"status":"passed","duration":5}}`
	wrap := func(steps string) string {
		return `[{"uri":"a.feature","name":"F","elements":[{"name":"S","type":"scenario","steps":[` + steps + `]}]}]`
	}
	nested := strings.Repeat("[", 200) + strings.Repeat("]", 200)
	tests := []struct {
		name  string
		input string
		lim   Limits
		want  error
		path  string
	}{
		{"empty input", "", Limits{}, ErrInvalidJSON, ""},
		{"whitespace only", "  \n", Limits{}, ErrInvalidJSON, ""},
		{"object instead of array", `{"name":"F"}`, Limits{}, ErrInvalidJSON, ""},
		{"null", `null`, Limits{}, ErrInvalidJSON, ""},
		{"string", `"x"`, Limits{}, ErrInvalidJSON, ""},
		{"empty array", `[]`, Limits{}, nil, ""},
		{"truncated file", wrap(step)[:60], Limits{}, ErrInvalidJSON, ""},
		{"trailing data", wrap(step) + ` x`, Limits{}, ErrInvalidJSON, ""},
		{"second document", wrap(step) + wrap(step), Limits{}, ErrInvalidJSON, ""},
		{"feature without elements", `[{"name":"F"}]`, Limits{}, ErrIncomplete, "feature[0]"},
		{"feature with null elements", `[{"name":"F","elements":null}]`, Limits{}, ErrIncomplete, "feature[0]"},
		{"feature with empty elements", `[{"name":"F","elements":[]}]`, Limits{}, nil, ""},
		{"second feature incomplete", `[{"name":"F","elements":[{"steps":[` + step + `]}]},{"name":"G"}]`, Limits{}, ErrIncomplete, "feature[1]"},
		{"step of a stepless scenario's neighbor incomplete", `[{"name":"F","elements":[{"name":"S"},{"steps":[{"name":"s"}]}]}]`, Limits{}, ErrIncomplete, "feature[0].elements[1].steps[0]"},
		{"step without result", wrap(`{"name":"s"}`), Limits{}, ErrIncomplete, "feature[0].elements[0].steps[0]"},
		{"step with null result", wrap(`{"name":"s","result":null}`), Limits{}, ErrIncomplete, "feature[0].elements[0].steps[0]"},
		{"step without status", wrap(`{"name":"s","result":{"duration":5}}`), Limits{}, ErrIncomplete, "feature[0].elements[0].steps[0]"},
		{"step with null status", wrap(`{"name":"s","result":{"status":null}}`), Limits{}, ErrIncomplete, "feature[0].elements[0].steps[0]"},
		{"status of wrong type", wrap(`{"name":"s","result":{"status":1}}`), Limits{}, ErrInvalidJSON, ""},
		{"empty status", wrap(`{"name":"s","result":{"status":""}}`), Limits{}, ErrInvalid, "feature[0].elements[0].steps[0].result.status"},
		{"unknown status", wrap(`{"name":"s","result":{"status":"broken"}}`), Limits{}, ErrInvalid, "feature[0].elements[0].steps[0].result.status"},
		{"unknown status in second step", wrap(step + `,{"name":"s","result":{"status":"Passed"}}`), Limits{}, ErrInvalid, "feature[0].elements[0].steps[1].result.status"},
		{"negative duration", wrap(`{"name":"s","result":{"status":"passed","duration":-1}}`), Limits{}, ErrInvalid, "feature[0].elements[0].steps[0].result.duration"},
		{"fractional duration", wrap(`{"name":"s","result":{"status":"passed","duration":1.5}}`), Limits{}, ErrInvalidJSON, ""},
		{"line of wrong type", wrap(`{"name":"s","line":"3","result":{"status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"rows of wrong type", wrap(`{"name":"s","rows":{"cells":[]},"result":{"status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"duplicate key", wrap(`{"name":"s","name":"t","result":{"status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"duplicate key differing in case", wrap(`{"name":"s","Name":"t","result":{"status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"duplicate key by Unicode folding", wrap(`{"keyword":"s","Keyword":"t","result":{"status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"duplicate status key", wrap(`{"name":"s","result":{"status":"failed","status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"duplicate key in unknown field", wrap(`{"name":"s","extra":{"a":1,"a":2},"result":{"status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"deep nesting in unknown field", wrap(`{"name":"s","extra":` + nested + `,"result":{"status":"passed"}}`), Limits{}, ErrInvalidJSON, ""},
		{"oversize input", wrap(step), Limits{Input: 64}, ErrTooLarge, ""},
		{"oversize embedding", wrap(`{"name":"s","result":{"status":"passed"},"embeddings":[{"name":"a","mime_type":"text/plain","data":"QUJDREVGR0g="}]}`), Limits{Embedding: 8}, ErrTooLarge, "feature[0].elements[0].steps[0].embeddings[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.input), tt.lim)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			var pe *ParseError
			if errors.As(err, &pe) {
				if pe.Path != tt.path {
					t.Errorf("path = %q, want %q", pe.Path, tt.path)
				}
			} else if tt.path != "" {
				t.Errorf("err %v carries no path, want %q", err, tt.path)
			}
		})
	}
}

// TestParseErrorsWithholdContent: no error message may echo input text, which can hold service data.
func TestParseErrorsWithholdContent(t *testing.T) {
	const marker = "SECRET-MARKER-7f3a"
	inputs := []string{
		`[{"name":"` + marker + `"}]`,
		`[{"name":"F","elements":[{"steps":[{"name":"s","result":{"status":"` + marker + `"}}]}]}]`,
		`[{"name":"F","elements":[{"steps":[{"name":"s","result":{"status":` + marker + `}}]}]}]`,
		`[{"name":"F","` + marker + `":1,"` + marker + `":2}]`,
		`[{"name":"F","line":"` + marker + `"}]`,
		`{"` + marker + `":true}`,
		`[]` + marker,
	}
	for i, in := range inputs {
		_, err := Parse(strings.NewReader(in), Limits{})
		if err == nil {
			t.Fatalf("input %d: no error", i)
		}
		if strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), "7f3a") {
			t.Errorf("input %d: error %q contains input content", i, err)
		}
	}
}

func TestParseIgnoresUnknownFields(t *testing.T) {
	features := mustParse(t, `[{"uri":"a.feature","name":"F","comments":[{"value":"# c","line":1}],"elements":[
		{"name":"S","type":"scenario","before":[{"result":{"status":"passed"}}],"steps":[
			{"keyword":"Given ","name":"s","hidden":true,"output":["x"],"result":{"status":"passed","duration":7}}]}]}]`)
	if got := *features[0].Elements[0].Steps[0].Result.Duration; got != 7 {
		t.Errorf("duration = %d", got)
	}
}

func TestParseKeepsInvalidBase64ForDecode(t *testing.T) {
	features := mustParse(t, `[{"name":"F","elements":[{"steps":[{"name":"s","result":{"status":"passed"},
		"embeddings":[{"name":"bad.txt","mime_type":"text/plain","data":"not base64!"}]}]}]}]`)
	e := features[0].Elements[0].Steps[0].Embeddings[0]
	if e.Data != "not base64!" {
		t.Fatalf("data = %q", e.Data)
	}
	if _, err := e.Decode(1 << 20); !errors.Is(err, ErrInvalidBase64) {
		t.Errorf("Decode err = %v, want ErrInvalidBase64", err)
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		data string
		max  int64
		want string
		err  error
	}{
		{"", 10, "", nil},
		{"aGk=", 10, "hi", nil},
		{"aGVsbG8=", 5, "hello", nil},
		{"aGVsbG8=", 4, "", ErrTooLarge},
		{"aGk", 10, "", ErrInvalidBase64},          // missing padding
		{"aGl=", 10, "", ErrInvalidBase64},         // non-zero padding bits
		{"aG\nk=", 10, "", ErrInvalidBase64},       // line break
		{"aGk=\n", 10, "", ErrInvalidBase64},       // trailing newline
		{" aGk=", 10, "", ErrInvalidBase64},        // leading space
		{"-_8=", 10, "", ErrInvalidBase64},         // URL alphabet
		{"aGk=aGk=", 10, "", ErrInvalidBase64},     // padding in the middle
		{"!!!!", 10, "", ErrInvalidBase64},         // not base64 at all
		{"/+8=", 10, "\xff\xef", nil},              // standard alphabet
		{"AAAA", 3, "\x00\x00\x00", nil},           // exact limit
		{"AAAAAAAA", 5, "", ErrTooLarge},           // over limit before decoding
		{"aGk=", 0, "", ErrTooLarge},               // zero limit admits only empty
		{"", 0, "", nil},                           // empty within zero limit
		{"aGk=", -1, "", ErrTooLarge},              // negative limit admits nothing but empty
		{"YWJj", 10, "abc", nil},                   // no padding needed
		{"YWJjZA==", 10, "abcd", nil},              // two padding characters
		{"YWJjZA=", 10, "", ErrInvalidBase64},      // short padding
		{"YWJjZA===", 10, "", ErrInvalidBase64},    // extra padding
		{"YWJjZA\r\n==", 10, "", ErrInvalidBase64}, // CRLF inside
		// Line breaks that keep the length a multiple of four pass the length check and are skipped by
		// Go's decoder; only the canonical re-encoding comparison refuses them.
		{"aGk=\n\n\n\n", 10, "", ErrInvalidBase64},
		{"aG\r\nk=\r\n", 10, "", ErrInvalidBase64},
		{"\n\n\n\nYWJj", 10, "", ErrInvalidBase64},
	}
	for _, tt := range tests {
		got, err := Embedding{Data: tt.data}.Decode(tt.max)
		if !errors.Is(err, tt.err) || (err == nil && string(got) != tt.want) {
			t.Errorf("Decode(%q, %d) = %q, %v; want %q, %v", tt.data, tt.max, got, err, tt.want, tt.err)
		}
		if tt.err == nil && err == nil && got == nil {
			t.Errorf("Decode(%q) returned nil slice", tt.data)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := Step{Result: Result{Status: StatusPassed}}
	tests := []struct {
		name     string
		features []Feature
		want     error
		path     string
	}{
		{"nil", nil, nil, ""},
		{"valid", []Feature{{Elements: []Scenario{{Steps: []Step{ok}}}}}, nil, ""},
		{"no scenarios", []Feature{{}}, nil, ""},
		{"stepless scenario (Godog writes it as undefined)", []Feature{{Elements: []Scenario{{}}}}, nil, ""},
		{"bad status", []Feature{{Elements: []Scenario{{Steps: []Step{ok, {Result: Result{Status: "x"}}}}}}}, ErrInvalid, "feature[0].elements[0].steps[1].result.status"},
		{"missing status", []Feature{{Elements: []Scenario{{Steps: []Step{{}}}}}}, ErrIncomplete, "feature[0].elements[0].steps[0]"},
	}
	for _, tt := range tests {
		err := Validate(tt.features)
		if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
			continue
		}
		var pe *ParseError
		if errors.As(err, &pe) && pe.Path != tt.path {
			t.Errorf("%s: path = %q, want %q", tt.name, pe.Path, tt.path)
		}
	}
}

// TestParseSteplessScenario reads real Godog v0.16.0 output for a scenario without steps. Godog writes
// it with no "steps" key and its own summary (testdata/synthetic/godog-gen, TestStepless) reads
// "3 scenarios (1 passed, 1 pending, 1 undefined)" and "3 steps (2 passed, 1 pending)".
func TestParseSteplessScenario(t *testing.T) {
	features, err := Parse(strings.NewReader(string(readFixture(t, "godog-v0.16.0-stepless.json"))), Limits{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(features) != 1 || len(features[0].Elements) != 3 {
		t.Fatalf("want 1 feature with 3 scenarios, got %+v", features)
	}
	scenarios := map[Status]int{}
	steps := map[Status]int{}
	for _, sc := range features[0].Elements {
		scenarios[sc.Status()]++
		for _, st := range sc.Steps {
			steps[st.Result.Status]++
		}
	}
	if scenarios[StatusPassed] != 1 || scenarios[StatusPending] != 1 || scenarios[StatusUndefined] != 1 {
		t.Errorf("scenarios = %v, want Godog's 1 passed, 1 pending, 1 undefined", scenarios)
	}
	if steps[StatusPassed] != 2 || steps[StatusPending] != 1 || len(steps) != 2 {
		t.Errorf("steps = %v, want Godog's 2 passed, 1 pending", steps)
	}
	if sc := features[0].Elements[1]; sc.Name != "No steps at all" || len(sc.Steps) != 0 || sc.Status() != StatusUndefined {
		t.Errorf("stepless scenario = %+v, status %s", sc, sc.Status())
	}
	if got := features[0].Status(); got != StatusUndefined {
		t.Errorf("feature status = %s, want undefined", got)
	}
	for _, in := range []string{`[{"name":"F","elements":[{"name":"S","steps":null}]}]`, `[{"name":"F","elements":[{"name":"S","steps":[]}]}]`} {
		f, err := Parse(strings.NewReader(in), Limits{})
		if err != nil || f[0].Elements[0].Status() != StatusUndefined {
			t.Errorf("%s: err %v", in, err)
		}
	}
}

// TestCombine follows multiple-cucumber-html-reporter's feature rule: failed, then ambiguous, then
// undefined if any scenario has that status; pending only when every scenario is pending; skipped only
// when every scenario is skipped or pending; otherwise passed.
func TestCombine(t *testing.T) {
	tests := []struct {
		in   []Status
		want Status
	}{
		{[]Status{"passed", "skipped"}, StatusPassed},
		{[]Status{"skipped", "skipped"}, StatusSkipped},
		{[]Status{"skipped", "pending"}, StatusSkipped},
		{[]Status{"pending", "pending"}, StatusPending},
		{[]Status{"passed", "pending"}, StatusPassed},
		{[]Status{"passed", "pending", "skipped"}, StatusPassed},
		{[]Status{"passed", "undefined", "skipped"}, StatusUndefined},
		{[]Status{"pending", "ambiguous"}, StatusAmbiguous},
		{[]Status{"undefined", "ambiguous"}, StatusAmbiguous},
		{[]Status{"skipped", "failed", "ambiguous"}, StatusFailed},
		{[]Status{"passed"}, StatusPassed},
		{[]Status{"skipped"}, StatusSkipped},
		{nil, StatusPassed},
	}
	for _, tt := range tests {
		if got := Combine(tt.in...); got != tt.want {
			t.Errorf("Combine(%v) = %s, want %s", tt.in, got, tt.want)
		}
	}
	sc := func(s Status) Scenario { return Scenario{Steps: []Step{{Result: Result{Status: s}}}} }
	f := Feature{Elements: []Scenario{sc("passed"), sc("skipped")}}
	if got := f.Status(); got != StatusPassed {
		t.Errorf("feature with a passed and a skipped scenario = %s, want passed", got)
	}
	run := []Feature{f, {Elements: []Scenario{sc("pending")}}}
	if got := RunStatus(run); got != StatusPassed {
		t.Errorf("RunStatus = %s, want passed (a pending scenario does not make the run pending)", got)
	}
	if got := RunStatus([]Feature{{Elements: []Scenario{sc("skipped"), sc("pending")}}}); got != StatusSkipped {
		t.Errorf("RunStatus of skipped and pending = %s, want skipped", got)
	}
}

// TestParseKeepsTextExactly: the model holds text exactly as given, including leading newlines,
// carriage returns, trailing spaces and multi-line descriptions; display decisions belong to renderers.
func TestParseKeepsTextExactly(t *testing.T) {
	const text = "\nfirst line was empty\r\n  indented  \n\ttab\u202eend "
	quoted := strconv.Quote(text)
	in := `[{"name":` + quoted + `,"description":` + quoted + `,"elements":[{"name":` + quoted + `,"description":` + quoted + `,"steps":[
		{"keyword":"Then ","name":` + quoted + `,"doc_string":{"value":` + quoted + `},"rows":[{"cells":[` + quoted + `]}],
		 "result":{"status":"failed","error_message":` + quoted + `}}]}]}]`
	f := mustParse(t, in)
	sc := f[0].Elements[0]
	st := sc.Steps[0]
	for name, got := range map[string]string{
		"feature name": f[0].Name, "feature description": f[0].Description, "scenario name": sc.Name,
		"scenario description": sc.Description, "step name": st.Name, "doc string": st.DocString.Value,
		"cell": st.Rows[0].Cells[0], "error": st.Result.Error,
	} {
		if got != text {
			t.Errorf("%s = %q, want %q", name, got, text)
		}
	}
}
