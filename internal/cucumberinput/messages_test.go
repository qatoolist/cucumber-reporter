package cucumber

import (
	"fmt"
	"strings"
	"testing"
)

func messagesFixture() string {
	return `{"gherkinDocument":{"feature":{"children":[{"scenario":{"id":"ast","location":{"line":7}}}]}}}
{"pickle":{"id":"p","uri":"check.feature","name":"checks","astNodeIds":["ast"],"steps":[{"id":"ps","text":"accepted"}]}}
{"hook":{"id":"hook","name":"before scenario"}}
{"testCase":{"id":"tc","pickleId":"p","testSteps":[{"id":"hs","hookId":"hook"},{"id":"ts","pickleStepId":"ps"}]}}
` + attemptMessages("a", 0, "FAILED", true) + attemptMessages("b", 1, "PASSED", false) + `{"testRunFinished":{"success":true,"timestamp":{"seconds":"2","nanos":0}}}`
}

func attemptMessages(id string, n int, status string, retry bool) string {
	return fmt.Sprintf(`{"testCaseStarted":{"id":%q,"testCaseId":"tc","attempt":%d,"workerId":"w1","timestamp":{"seconds":"1","nanos":0}}}
{"testStepFinished":{"testCaseStartedId":%q,"testStepId":"hs","testStepResult":{"status":"PASSED","duration":{"seconds":"0","nanos":10}}}}
{"testStepFinished":{"testCaseStartedId":%q,"testStepId":"ts","testStepResult":{"status":%q,"message":"detail","duration":{"seconds":"0","nanos":100}}}}
{"attachment":{"testCaseStartedId":%q,"testStepId":"ts","body":"evidence","mediaType":"text/plain","contentEncoding":"IDENTITY"}}
{"testCaseFinished":{"testCaseStartedId":%q,"willBeRetried":%t,"timestamp":{"seconds":"2","nanos":0}}}
`, id, n, id, id, status, id, id, retry)
}

func TestMessagesPreserveAttemptsHooksAndEvidence(t *testing.T) {
	r, err := ParseMessages(strings.NewReader(messagesFixture()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Attempts) != 2 || r.Attempts[1].Number != 1 || r.Attempts[1].Scenario.Status() != StatusPassed || r.Attempts[0].Scenario.Status() != StatusFailed {
		t.Fatalf("%+v", r)
	}
	sc := r.Attempts[0].Scenario
	if sc.Line != 7 || len(sc.Steps) != 2 || sc.Steps[0].Keyword != "Hook " {
		t.Fatalf("%+v", sc)
	}
	b, err := sc.Steps[1].Embeddings[0].Decode(100)
	if err != nil || string(b) != "evidence" {
		t.Fatal("evidence lost")
	}
}

func TestMessagesRefuseIncompleteAndExternal(t *testing.T) {
	for _, input := range []string{strings.Split(messagesFixture(), `{"testRunFinished"`)[0], messagesFixture() + "\n" + `{"externalAttachment":{"url":"https://example.invalid/secret"}}`, strings.Replace(messagesFixture(), `"status":"PASSED"`, `"status":"invented"`, 1)} {
		if _, err := ParseMessages(strings.NewReader(input), Limits{}); err == nil {
			t.Fatal("invalid stream accepted")
		}
	}
}

func TestMessagesRunHookFailure(t *testing.T) {
	input := `{"hook":{"id":"h","name":"database setup"}}
{"testRunHookStarted":{"id":"r","hookId":"h","timestamp":{"seconds":"1","nanos":0}}}
{"testRunHookFinished":{"testRunHookStartedId":"r","result":{"status":"FAILED","message":"setup failed","duration":{"seconds":"1","nanos":0}},"timestamp":{"seconds":"2","nanos":0}}}
{"testRunFinished":{"success":false,"timestamp":{"seconds":"2","nanos":0}}}`
	r, err := ParseMessages(strings.NewReader(input), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Attempts) != 1 || r.Attempts[0].Scenario.Status() != StatusFailed {
		t.Fatal("run hook failure hidden")
	}
}

func TestStreamParity(t *testing.T) {
	for _, input := range []string{`[]`, `null`, `[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]}]`, `[{"elements":[{"steps":[{"result":{"status":"passed","Status":"failed"}}]}]}]`, `[{"elements":[]}]`, `[] {}`} {
		a, e1 := Parse(strings.NewReader(input), Limits{})
		b, e2 := ParseStream(strings.NewReader(input), Limits{})
		if (e1 == nil) != (e2 == nil) || len(a) != len(b) {
			t.Fatalf("parity: %s %v %v", input, e1, e2)
		}
	}
}

func TestMessagesRejectOrphanedExecution(t *testing.T) {
	for _, extra := range []string{
		`{"attachment":{"testCaseStartedId":"missing","testStepId":"ts","body":"evidence","mediaType":"text/plain","contentEncoding":"IDENTITY"}}`,
		`{"testStepFinished":{"testCaseStartedId":"missing","testStepId":"ts","testStepResult":{"status":"FAILED","duration":{"seconds":"0","nanos":10}}}}`,
		`{"testCaseFinished":{"testCaseStartedId":"missing","timestamp":{"seconds":"2","nanos":0}}}`,
	} {
		if _, err := ParseMessages(strings.NewReader(messagesFixture()+"\n"+extra), Limits{}); err == nil {
			t.Fatal("orphaned execution discarded")
		}
	}
	input := strings.ReplaceAll(messagesFixture(), `"success":true`, `"success":false`)
	if run, err := ParseMessages(strings.NewReader(input), Limits{}); err != nil || !run.Attempts[0].RunnerFailed {
		t.Fatalf("runner failure signal lost: %v", err)
	}
}

func TestMessagesExactInputLimit(t *testing.T) {
	input := messagesFixture()
	if _, err := ParseMessages(strings.NewReader(input), Limits{Input: int64(len(input))}); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMessages(strings.NewReader(input), Limits{Input: int64(len(input) - 1)}); err == nil {
		t.Fatal("oversized stream accepted")
	}
}

func FuzzMessages(f *testing.F) {
	f.Add(messagesFixture())
	f.Add(`{"testRunFinished":{"success":false}}`)
	f.Fuzz(func(t *testing.T, input string) {
		run, err := ParseMessages(strings.NewReader(input), Limits{Input: 1 << 20, Embedding: 1 << 16})
		if err != nil {
			return
		}
		if len(input) > 1<<20 {
			t.Fatal("input limit ignored")
		}
		for _, a := range run.Attempts {
			if err := Validate([]Feature{{Elements: []Scenario{a.Scenario}}}); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestMessagesUnknownIsVisiblePending(t *testing.T) {
	input := strings.ReplaceAll(messagesFixture(), `"PASSED"`, `"UNKNOWN"`)
	run, err := ParseMessages(strings.NewReader(input), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	last := run.Attempts[len(run.Attempts)-1].Scenario
	if last.Status() != StatusPending || !strings.Contains(last.Steps[0].Result.Error, "UNKNOWN") {
		t.Fatal("unknown outcome was hidden")
	}
}

func TestMessagesEmptyCompletedShard(t *testing.T) {
	run, err := ParseMessages(strings.NewReader(`{"testRunFinished":{"success":true}}`), Limits{})
	if err != nil || len(run.Attempts) != 0 {
		t.Fatalf("empty shard: %+v %v", run, err)
	}
	for _, raw := range []string{"", `{"testRunFinished":{"success":false}}`, `{"testCaseFinished":{"testCaseStartedId":"orphan"}}` + "\n" + `{"testRunFinished":{"success":true}}`} {
		if _, err := ParseMessages(strings.NewReader(raw), Limits{}); err == nil {
			t.Fatal("incomplete empty run accepted")
		}
	}
}
