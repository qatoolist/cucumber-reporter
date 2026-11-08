package cucumber

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRound12SyntaxOffsets(t *testing.T) {
	for _, raw := range []string{`[{"name":"` + strings.Repeat("x", 120) + `",,"elements":[]}]`, `[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]},,{"elements":[]}]`, `[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]}],`, `[{"name":"unterminated`} {
		var parsed any
		native := json.Unmarshal([]byte(raw), &parsed)
		var syntax *json.SyntaxError
		if !errors.As(native, &syntax) {
			t.Fatal(native)
		}
		for _, parse := range []func(*bytes.Reader, Limits) ([]Feature, error){func(r *bytes.Reader, l Limits) ([]Feature, error) { return Parse(r, l) }, func(r *bytes.Reader, l Limits) ([]Feature, error) { return ParseStream(r, l) }, func(r *bytes.Reader, l Limits) ([]Feature, error) { return ParseEnhanced(r, l) }} {
			_, err := parse(bytes.NewReader([]byte(raw)), Limits{})
			var pe *ParseError
			if !errors.As(err, &pe) || pe.Offset != syntax.Offset {
				t.Errorf("offset got %v; want %d", err, syntax.Offset)
			}
		}
	}
	raw := `[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]}]`
	_, err := ParseStream(strings.NewReader(raw), Limits{Input: 20})
	var pe *ParseError
	if !errors.Is(err, ErrTooLarge) || errors.As(err, &pe) {
		t.Fatalf("size limit has an offset: %v", err)
	}
}

func TestRound12UnicodeKnownDuplicateKeys(t *testing.T) {
	for _, key := range []string{"Status", "ſtatus"} {
		raw := `[{"elements":[{"steps":[{"result":{"status":"failed","` + key + `":"passed"}}]}]}]`
		if _, err := Parse(strings.NewReader(raw), Limits{}); err == nil {
			t.Fatal("duplicate accepted")
		}
	}
	raw := strings.Replace(messagesFixture(), `"status":"FAILED"`, `"status":"FAILED","ſtatus":"PASSED"`, 1)
	if _, err := ParseMessages(strings.NewReader(raw), Limits{}); err == nil {
		t.Fatal("Messages Unicode duplicate accepted")
	}
}
