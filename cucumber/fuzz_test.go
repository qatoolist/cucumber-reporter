package cucumber

import (
	"bytes"
	"errors"
	"testing"
)

// FuzzParse: Parse never panics, honors its input limit, returns only the documented errors and,
// when it succeeds, returns a model that passes Validate.
func FuzzParse(f *testing.F) {
	f.Add(readFixture(f, "godog-v0.16.0.json"))
	for _, seed := range []string{
		``, `[]`, `{}`, `null`, `[{}]`, `[{"elements":[{"steps":[{"result":{"status":"passed"}}]}]}]`,
		`[{"elements":[{"steps":[{"result":{"status":"passed","duration":-1}}]}]}]`,
		`[{"a":1,"A":2}]`, `[[[[[[[[[[]]]]]]]]]]`, `[{"elements":[{"steps":[{"result":{"status":"failed"},"embeddings":[{"data":"aGk="}]}]}]}] x`,
	} {
		f.Add([]byte(seed))
	}
	lim := Limits{Input: 1 << 20, Embedding: 1 << 16}
	sentinels := []error{ErrNoResults, ErrInvalidJSON, ErrIncomplete, ErrInvalid, ErrTooLarge}
	f.Fuzz(func(t *testing.T, data []byte) {
		features, err := Parse(bytes.NewReader(data), lim)
		if err != nil {
			for _, s := range sentinels {
				if errors.Is(err, s) {
					return
				}
			}
			t.Fatalf("undocumented error %v", err)
		}
		if int64(len(data)) > lim.Input {
			t.Fatalf("accepted %d bytes over the %d limit", len(data), lim.Input)
		}
		if err := Validate(features); err != nil {
			t.Fatalf("Parse returned a model that fails Validate: %v", err)
		}
		for _, f := range features {
			for _, sc := range f.Elements {
				for _, st := range sc.Steps {
					for _, e := range st.Embeddings {
						if int64(len(e.Data)) > lim.Embedding {
							t.Fatalf("accepted an embedding over the limit")
						}
						if raw, err := e.Decode(lim.Embedding); err == nil && int64(len(raw)) > lim.Embedding {
							t.Fatalf("Decode exceeded its limit")
						}
					}
				}
			}
		}
	})
}
