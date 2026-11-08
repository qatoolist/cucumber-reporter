package cucumber

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// K5-L2: an envelope value of the wrong type names the Messages field instead of
// reporting a generic "not valid cucumber JSON" error.
func TestRound18MessagesFieldTypeIsNamed(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/round16/F3/adv/a14-success-string.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ input, field, at string }{
		{string(raw), "testRunFinished.success", "message[38]"},
		{`{"testStepFinished":{"testCaseStartedId":"a","testStepId":"b","testStepResult":{"status":7}}}` + "\n", "testStepFinished.testStepResult.status", "message[1]"},
	} {
		_, err = ParseMessages(strings.NewReader(tc.input), Limits{})
		if !errors.Is(err, ErrInvalid) || errors.Is(err, ErrInvalidJSON) || !strings.Contains(err.Error(), "Messages envelope field "+tc.field+" has the wrong type") || !strings.Contains(err.Error(), tc.at) || strings.Contains(err.Error(), "JSON") {
			t.Fatalf("%s: %v", tc.field, err)
		}
	}
}
