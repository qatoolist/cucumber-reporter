package cucumber

import (
	"bytes"
	"strings"
	"testing"
)

func TestRound10EncodingsAndTypeErrors(t *testing.T) {
	valid := `[{"elements":[{"steps":[{"result":{"status":"passed","duration":1}}]}]}]`
	for _, parse := range []struct {
		name string
		fn   func(*bytes.Reader) ([]Feature, error)
	}{
		{"Parse", func(r *bytes.Reader) ([]Feature, error) { return Parse(r, Limits{}) }},
		{"Stream", func(r *bytes.Reader) ([]Feature, error) { return ParseStream(r, Limits{}) }},
		{"Enhanced", func(r *bytes.Reader) ([]Feature, error) { return ParseEnhanced(r, Limits{}) }},
	} {
		t.Run(parse.name, func(t *testing.T) {
			if _, err := parse.fn(bytes.NewReader(append([]byte{0xef, 0xbb, 0xbf}, []byte(valid)...))); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				data []byte
				want string
			}{
				{[]byte{0xff, 0xfe, '[', 0}, "UTF-16"},
				{[]byte(strings.Replace(valid, `"duration":1`, `"duration":1.2`, 1)), "duration must be integer nanoseconds"},
				{[]byte(strings.Replace(valid, `"steps":`, `"line":"SECRET","steps":`, 1)), "line must be an integer"},
			} {
				_, err := parse.fn(bytes.NewReader(tc.data))
				if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "SECRET") {
					t.Fatalf("unsafe or unhelpful error: %v", err)
				}
			}
		})
	}
}
