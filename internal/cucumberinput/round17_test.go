package cucumber

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// F4-N5: a Messages stream without testRunFinished.success names that field
// instead of reporting a generic JSON syntax error.
func TestRound17MissingRunSuccessIsNamed(t *testing.T) {
	f, err := os.Open("../../testdata/round16/F3/adv/a13-success-missing.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = ParseMessages(f, Limits{})
	if !errors.Is(err, ErrInvalid) || errors.Is(err, ErrInvalidJSON) || !strings.Contains(err.Error(), "testRunFinished.success is missing") || !strings.Contains(err.Error(), "message[38]") || strings.Contains(err.Error(), "JSON") {
		t.Fatalf("missing success: %v", err)
	}
}
