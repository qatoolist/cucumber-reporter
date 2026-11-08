package report

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func TestRedactionReviewReproductions(t *testing.T) {
	r, err := newRedactor(&Redaction{Patterns: []string{`sk_live_\w+`, `\d{16}`, `user=\w+`, `Bearer .*`, `REDACTED`}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, mime, body, want string }{
		{"JSON keys and numbers", "application/json", `{"sk_live_KEYNAME":1,"card":4111111111111111}`, `{"[REDACTED]":1,"card":"[REDACTED]"}`},
		{"JSON formatting", "application/json", "{\n  \"z\": \"<b>&\", \"a\": \"sk_live_abc\", \"n\": 9007199254740993\n}", "{\n  \"z\": \"<b>&\", \"a\": \"[REDACTED]\", \"n\": \"[REDACTED]\"\n}"},
		{"truncated JSON", "application/json", `{"truncated": "sk_live_abc`, `{"truncated": "[REDACTED]`},
		{"yaml", "application/x-yaml", "token: sk_live_abc", "token: [REDACTED]"},
		{"ndjson", "application/x-ndjson", "{\"t\":\"sk_live_abc\"}\n{\"t\":\"sk_live_def\"}\n", "{\"t\":\"[REDACTED]\"}\n{\"t\":\"[REDACTED]\"}\n"},
		{"form", "application/x-www-form-urlencoded", "t=sk_live_abc", "t=[REDACTED]"},
		{"graphql", "application/graphql", "query { sk_live_abc }", "query { [REDACTED] }"},
		{"missing MIME", "", "sk_live_abc", "[REDACTED]"},
		{"greedy", "text/plain", "Authorization: Bearer user=bob TOKENTAIL", "Authorization: [REDACTED]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := []cucumber.Step{{Embeddings: []cucumber.Embedding{{MimeType: tc.mime, Data: base64.StdEncoding.EncodeToString([]byte(tc.body))}}}}
			if err := r.attachments(steps); err != nil {
				t.Fatal(err)
			}
			got, err := steps[0].Embeddings[0].Decode(10000)
			if err != nil || string(got) != tc.want {
				t.Fatalf("got %q, want %q; %v", got, tc.want, err)
			}
		})
	}
}

func TestRedactionOverlappingPatterns(t *testing.T) {
	for _, patterns := range [][]string{{`user=\w+`, `Bearer .*`, `REDACTED`}, {`Bearer .*`, `user=\w+`, `REDACTED`}} {
		r, _ := newRedactor(&Redaction{Patterns: patterns})
		for _, input := range []string{"Bearer user=bob TOKENTAIL", "Bearer [REDACTED] TOKENTAIL"} {
			if got := r.text(input); got != "[REDACTED]" {
				t.Fatalf("%q => %q", input, got)
			}
		}
		if got := r.text("already [REDACTED] safe"); got != "already [REDACTED] safe" {
			t.Fatal(got)
		}
	}
}

func TestRedactionJSONTokensAndKeys(t *testing.T) {
	r, _ := newRedactor(&Redaction{Patterns: []string{`sk_live_\w+`, `4111111111111111`, `https://secret.test/"quoted"`}, JSONKeys: []string{"password"}})
	input := " {\n\"z\":1, \"password\": {\"nested\": [1,2]}, \"sk_live_KEY\":\"a\", \"sk_live_KEY\":\"b\", \"a\": [4111111111111111,9007199254740993], \"url\":\"https:\\/\\/secret.test\\/\\\"quoted\\\"\"\n} "
	want := " {\n\"z\":1, \"password\": \"[REDACTED]\", \"[REDACTED]\":\"a\", \"[REDACTED]\":\"b\", \"a\": [\"[REDACTED]\",9007199254740993], \"url\":\"[REDACTED]\"\n} "
	out, err := r.jsonValue([]byte(input))
	if err != nil || !json.Valid(out) || string(out) != want {
		t.Fatalf("%s\nwant %s\n%v", out, want, err)
	}
	// Rewriting must not collapse duplicate keys or disturb unchanged tokens.
	again, err := r.jsonValue(out)
	if err != nil || string(again) != want {
		t.Fatalf("non-idempotent JSON: %s %v", again, err)
	}
}

func TestMalformedJSONDocStringFallsBack(t *testing.T) {
	f := runFixture()
	f[0].Elements[0].Steps[0].DocString = &cucumber.DocString{ContentType: "application/json", Value: `{"truncated":"sk_live_abc`}
	p := prepared(t, f, RunConfig{Redaction: &Redaction{Patterns: []string{`sk_live_\w+`}}})
	if got := p.Features[0].Elements[0].Steps[0].DocString.Value; strings.Contains(got, "sk_live_") || !strings.Contains(got, "[REDACTED]") {
		t.Fatal(got)
	}
}

func TestRedactionPreservesBinaryAndUnchangedJSON(t *testing.T) {
	r, _ := newRedactor(&Redaction{Patterns: []string{`sk_live_\w+`}})
	for _, tc := range []struct{ mime, body string }{
		{"application/octet-stream", "sk_live_abc"},
		{"application/octet-stream", "\xffsk_live_abc"},
		{"application/json", " {\n \"z\":\"<b>&\", \"a\":9007199254740993, \"escaped\":\"a\\/b\"\n} "},
	} {
		out, err := r.attachmentText([]byte(tc.body), tc.mime)
		if err != nil || string(out) != tc.body {
			t.Fatalf("changed %s attachment: %q %v", tc.mime, out, err)
		}
	}
}

func FuzzRedactionJSON(f *testing.F) {
	for _, seed := range []string{
		`{"sk_live_KEYNAME":1,"card":4111111111111111}`,
		`{"truncated":"sk_live_abc`,
		`{"token":{"nested":[1,{"t":2}]},"x":3}`,
		`{"url":"https:\/\/secret.test\/\"quoted\""}`,
		`{"x":"Bearer user=bob TOKENTAIL","duplicate":1,"duplicate":2}`,
		` [true,false,null, -1.234e+56, "\\\"", "\u0022"] `,
	} {
		f.Add([]byte(seed))
	}
	r, err := newRedactor(&Redaction{Patterns: []string{`sk_live_\w+`, `\d{16}`, `user=\w+`, `Bearer .*`, `REDACTED`}, JSONKeys: []string{"token"}})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		untyped, untypedErr := r.attachmentText(input, "")
		if untypedErr != nil {
			t.Fatalf("untyped evidence was refused: %v", untypedErr)
		}
		if jsonEvidence("", input) && !json.Valid(untyped) {
			t.Fatal("inferred JSON was corrupted")
		}
		out, err := r.jsonValue(input)
		if len(bytes.Trim(input, " \t\r\n")) == 0 {
			if err != nil || !bytes.Equal(out, input) {
				t.Fatal("empty JSON was rejected or changed")
			}
			return
		}
		if !json.Valid(input) {
			if err == nil || out != nil {
				t.Fatal("malformed JSON bypassed structural redaction")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if json.Valid(input) && !json.Valid(out) {
			t.Fatalf("valid JSON corrupted: %q => %q", input, out)
		}
		if r.patterns[0].Match(out) && !strings.Contains(string(input), `\`) {
			t.Fatalf("secret leaked: %q", out)
		}
	})
}

func TestJSONKeyRedactionFailsClosedOnMalformedEvidence(t *testing.T) {
	r, initErr := newRedactor(&Redaction{JSONKeys: []string{"password"}})
	if initErr != nil {
		t.Fatal(initErr)
	}
	for _, tc := range []struct{ name, mime, body string }{
		{"truncated object", "application/json", `{"password":"hunter2"`},
		{"truncated string", "application/json", `{"password":"hunter2`},
		{"nested object", "application/json", `{"outer":{"password":"hunter2"}`},
		{"escaped key", "application/json", `{"pass\u0077ord":"hunter2"`},
		{"truncated NDJSON line", "application/x-ndjson", "{\"password\":\"safe\"}\n{\"password\":\"hunter2\",\"x\":"},
		{"malformed NDJSON bytes", "application/ndjson", "{\"password\":\"hunter2\",\"x\":\xff}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := r.attachmentText([]byte(tc.body), tc.mime)
			if err == nil || out != nil {
				t.Fatalf("malformed evidence did not fail closed: %q %v", out, err)
			}
			if !strings.Contains(err.Error(), "jsonKeys") || strings.Contains(err.Error(), "hunter2") {
				t.Fatalf("unsafe or unclear diagnostic: %v", err)
			}
		})
	}
	// Blank NDJSON lines and a trailing newline are not malformed records.
	raw := "\n \t\r\n{\"password\":\"hunter2\"}\r\n\n"
	out, err := r.attachmentText([]byte(raw), "application/x-ndjson")
	if err != nil || string(out) != "\n \t\r\n{\"password\":\"[REDACTED]\"}\r\n\n" {
		t.Fatalf("valid NDJSON rejected or leaked: %q %v", out, err)
	}
}

func TestMalformedJSONKeyRedactionInRetryEvidence(t *testing.T) {
	f := runFixture()
	steps := append([]cucumber.Step(nil), f[0].Elements[0].Steps...)
	steps[0].DocString = &cucumber.DocString{ContentType: "application/json", Value: `{"password":"hunter2"`}
	id := runScenarioIDs(f)[0][0]
	p, err := preparePublicRun([]runInput{{Name: "input", Features: f}}, renderOptions{Version: 2}, RunConfig{
		Redaction: &Redaction{JSONKeys: []string{"password"}},
		Metadata:  map[string]ScenarioMeta{id: {Attempts: []Attempt{{Number: 0, Status: "passed", Steps: publicSteps(steps, 0)}}}},
	})
	if err == nil || p != nil || !strings.Contains(err.Error(), "jsonKeys") || !strings.Contains(err.Error(), "feature[0].elements[0].attempts[0].steps[0].doc_string") || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("retry evidence bypassed redaction or had no safe location: %v", err)
	}
}

func TestExplicitlyExcludedMalformedJSONIsNotInspected(t *testing.T) {
	f := runFixture()
	f[0].Elements[0].Steps[0].Embeddings = []cucumber.Embedding{{Name: "broken-response", MimeType: "application/json", Data: base64.StdEncoding.EncodeToString([]byte(`{"password":"hunter2"`))}}
	p := prepared(t, f, RunConfig{Redaction: &Redaction{JSONKeys: []string{"password"}, ExcludeAttachments: []string{"^broken-response$"}}})
	if len(p.Features[0].Elements[0].Steps[0].Embeddings) != 0 {
		t.Fatal("excluded malformed evidence retained")
	}
}

func TestJSONEvidenceAliasesAndMissingTypes(t *testing.T) {
	for _, mt := range []string{"json", "text/json", "text/x-json", "application/json", "application/problem+json", ""} {
		for _, body := range []string{`{"password":"DOCSECRET9"}`, `[{"password":"DOCSECRET9"}]`, `{"password":"DOCSECRET9"`} {
			t.Run(mt+"/"+body, func(t *testing.T) {
				f := runFixture()
				f[0].Elements[0].Steps[0].Embeddings = []cucumber.Embedding{{MimeType: mt, Data: base64.StdEncoding.EncodeToString([]byte(body))}}
				policy := &Redaction{JSONKeys: []string{"password"}}
				if mt == "" && !json.Valid([]byte(body)) {
					policy.Patterns = []string{"DOCSECRET9"}
				}
				p, err := preparePublicRun([]runInput{{Name: "input", Features: f}}, renderOptions{Version: 2}, RunConfig{Redaction: policy})
				if mt != "" && !json.Valid([]byte(body)) {
					if err == nil || p != nil || !strings.Contains(err.Error(), "feature[0].elements[0].steps[0].embeddings[0]") || strings.Contains(err.Error(), "DOCSECRET9") {
						t.Fatalf("unsafe or missing refusal: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				raw, decodeErr := p.Features[0].Elements[0].Steps[0].Embeddings[0].Decode(10000)
				if decodeErr != nil || strings.Contains(string(raw), "DOCSECRET9") || !strings.Contains(string(raw), "[REDACTED]") {
					t.Fatalf("alias bypass: %q %v", raw, decodeErr)
				}
			})
		}
	}
}

func TestEmptyJSONEvidenceIsSafe(t *testing.T) {
	r, err := newRedactor(&Redaction{JSONKeys: []string{"password"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"", " \r\n\t"} {
		steps := []cucumber.Step{{Embeddings: []cucumber.Embedding{{MimeType: "application/json", Data: base64.StdEncoding.EncodeToString([]byte(body))}}}}
		if evidenceErr := r.attachments(steps); evidenceErr != nil {
			t.Fatal(evidenceErr)
		}
		out, redactErr := r.jsonValue([]byte(body))
		if redactErr != nil || string(out) != body {
			t.Fatalf("empty body rejected or changed: %q %v", out, redactErr)
		}
	}
}

func TestUntypedPlainDocStringsRemainText(t *testing.T) {
	for _, mt := range []string{"", "text/plain", "sql"} {
		f := runFixture()
		f[0].Elements[0].Steps[0].DocString = &cucumber.DocString{ContentType: mt, Value: "instructions secret-value"}
		p := prepared(t, f, RunConfig{Redaction: &Redaction{JSONKeys: []string{"password"}, Patterns: []string{"secret-value"}}})
		if got := p.Features[0].Elements[0].Steps[0].DocString.Value; got != "instructions [REDACTED]" {
			t.Fatalf("text doc string changed semantics: %q", got)
		}
	}
}

func TestRedactionErrorLocatesEvidenceWithoutNames(t *testing.T) {
	f := runFixture()
	f[0].Name = "PRIVATEFEATURE"
	f[0].Elements[1].Name = "PRIVATESCENARIO"
	step := cucumber.Step{Name: "PRIVATESTEP", Result: cucumber.Result{Status: cucumber.StatusPassed}, Embeddings: []cucumber.Embedding{
		{Name: "excluded", MimeType: "application/json", Data: base64.StdEncoding.EncodeToString([]byte("broken"))},
		{Name: "PRIVATEATTACHMENT", MimeType: "application/x-ndjson", Data: base64.StdEncoding.EncodeToString([]byte("{}\n{\"password\":\"PRIVATEVALUE\""))},
	}}
	f[0].Elements[1].Steps = append(f[0].Elements[1].Steps, step)
	_, err := preparePublicRun([]runInput{{Name: "input", Features: f}}, renderOptions{Version: 2}, RunConfig{Redaction: &Redaction{JSONKeys: []string{"password"}, ExcludeAttachments: []string{"^excluded$"}}})
	if err == nil || !strings.Contains(err.Error(), "feature[0].elements[1].steps[1].embeddings[1].line[1]") || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatalf("unsafe or unhelpful evidence location: %v", err)
	}
}

func TestUntypedEvidenceUsesTextUnlessValidJSON(t *testing.T) {
	for _, body := range []string{"[INFO] user logged in", "{{x}} logs in", "[docs](https://example.test)", "[2026-11-06 10:00] started", `{"password":"unfinished"`} {
		for _, withPatterns := range []bool{false, true} {
			f := runFixture()
			step := &f[0].Elements[0].Steps[0]
			step.DocString = &cucumber.DocString{Value: body}
			step.Embeddings = []cucumber.Embedding{{Data: base64.StdEncoding.EncodeToString([]byte(body))}}
			policy := &Redaction{JSONKeys: []string{"password"}}
			want := body
			if withPatterns {
				policy.Patterns = []string{"logged in|logs in|started|unfinished|example.test"}
				re := regexp.MustCompile(policy.Patterns[0])
				want = re.ReplaceAllString(body, "[REDACTED]")
			}
			p := prepared(t, f, RunConfig{Redaction: policy})
			got := p.Features[0].Elements[0].Steps[0]
			raw, err := got.Embeddings[0].Decode(10000)
			if err != nil || string(raw) != want || got.DocString.Value != want {
				t.Fatalf("untyped text changed semantics: %q => doc=%q attachment=%q; %v", body, got.DocString.Value, raw, err)
			}
		}
	}
}

func TestJSONLinesAliases(t *testing.T) {
	for _, mt := range []string{"application/jsonl", "application/jsonlines", "application/x-jsonlines", "jsonl", "jsonlines", "ndjson", "text/x-ndjson"} {
		for _, malformed := range []bool{false, true} {
			body := "{\"password\":\"LINESECRET\"}\n{\"password\":\"LINESECRET\"}\n"
			if malformed {
				body += "{\"password\":\"LINESECRET\""
			}
			r, err := newRedactor(&Redaction{JSONKeys: []string{"password"}})
			if err != nil {
				t.Fatal(err)
			}
			steps := []cucumber.Step{{Embeddings: []cucumber.Embedding{{MimeType: mt, Data: base64.StdEncoding.EncodeToString([]byte(body))}}}, {DocString: &cucumber.DocString{ContentType: mt, Value: body}}}
			for i := range steps {
				redactErr := r.attachments(steps[i : i+1])
				if malformed {
					if redactErr == nil {
						t.Fatalf("%s accepted malformed JSON Lines", mt)
					}
					continue
				}
				if redactErr != nil {
					t.Fatal(redactErr)
				}
				var got string
				if i == 0 {
					raw, decodeErr := steps[i].Embeddings[0].Decode(10000)
					if decodeErr != nil {
						t.Fatal(decodeErr)
					}
					got = string(raw)
				} else {
					got = steps[i].DocString.Value
				}
				if strings.Contains(got, "LINESECRET") || strings.Count(got, "[REDACTED]") != 2 {
					t.Fatalf("%s not key-redacted: %q", mt, got)
				}
			}
		}
	}
}

func TestJSON5AttachmentsUseLiteralPatterns(t *testing.T) {
	r, err := newRedactor(&Redaction{Patterns: []string{"SECRET"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mt := range []string{"application/json5", "json5"} {
		out, redactErr := r.attachmentText([]byte(`{password:'SECRET'}`), mt)
		if redactErr != nil || string(out) != `{password:'[REDACTED]'}` {
			t.Fatalf("JSON5 text scanning failed: %q %v", out, redactErr)
		}
	}
}
