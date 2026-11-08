package cucumber

import (
	"strings"
	"testing"
)

func TestRound11BOMAtExactLimit(t *testing.T) {
	raw := `[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]}]`
	for _, parse := range []func(*strings.Reader, Limits) ([]Feature, error){func(r *strings.Reader, l Limits) ([]Feature, error) { return Parse(r, l) }, func(r *strings.Reader, l Limits) ([]Feature, error) { return ParseStream(r, l) }, func(r *strings.Reader, l Limits) ([]Feature, error) { return ParseEnhanced(r, l) }} {
		if _, err := parse(strings.NewReader("\ufeff"+raw), Limits{Input: int64(len(raw))}); err != nil {
			t.Fatal(err)
		}
	}
	raw = messagesFixture()
	if _, err := ParseMessages(strings.NewReader("\ufeff"+raw), Limits{Input: int64(len(raw))}); err != nil {
		t.Fatal(err)
	}
}

func TestMessageArrivalOrderAndRunFacts(t *testing.T) {
	raw := strings.ReplaceAll(messagesFixture(), `"attempt":0`, `"attempt":9`)
	raw = strings.ReplaceAll(raw, `"attempt":1`, `"attempt":2`)
	raw = strings.ReplaceAll(raw, `"willBeRetried":true`, `"willBeRetried":false`)
	raw = `{"meta":{"ci":{"git":{"revision":"revision","branch":"branch","remote":"remote"}}}}` + "\n" + `{"testRunStarted":{"timestamp":{"seconds":"1","nanos":0}}}` + "\n" + raw
	run, err := ParseMessages(strings.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Attempts[0].Number != 0 || run.Attempts[1].Number != 1 || run.Attempts[1].Scenario.Status() != StatusPassed {
		t.Fatal("runner numbers overrode event order")
	}
	if run.Started != "1970-01-01T00:00:01Z" || run.Finished != "1970-01-01T00:00:02Z" || run.Branch != "branch" || run.Revision != "revision" || run.Remote != "remote" {
		t.Fatalf("lost run facts: %+v", run)
	}
}
