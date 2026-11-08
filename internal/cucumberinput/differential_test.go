package cucumber

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

func FuzzParseDifferential(f *testing.F) {
	for _, s := range []string{`[{"elements":[{"steps":[]}]}]`, `[{"elements":[{"type":"background","steps":[{"result":{"status":"passed"}}]},{"before":[{"result":{"status":"failed"}}],"steps":[]}]}]`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		lim := Limits{Input: 65536}
		a, ea := Parse(bytes.NewReader(b), lim)
		s, es := ParseStream(bytes.NewReader(b), lim)
		if (ea == nil) != (es == nil) {
			t.Fatalf("parser acceptance differs: %v / %v", ea, es)
		}
		enhanced, ee := ParseEnhanced(bytes.NewReader(b), lim)
		if ea == nil {
			if ee != nil {
				t.Fatalf("enhanced parser rejected plain input: %v", ee)
			}
			folded, err := modelmeta.PlainFeatures(enhanced)
			if err != nil || !reflect.DeepEqual(a, folded) {
				t.Fatal("enhanced/plain folded models differ")
			}
		} else if ee == nil && !errors.Is(ea, ErrNoResults) {
			t.Fatalf("unexpected enhanced acceptance: %v", ea)
		}
		if ea == nil && !reflect.DeepEqual(a, s) {
			t.Fatal("parser models differ")
		}
		var pa, ps *ParseError
		if bytes.HasPrefix(bytes.TrimSpace(b), []byte("[")) && errors.Is(ea, ErrInvalidJSON) && errors.Is(es, ErrInvalidJSON) && errors.As(ea, &pa) && errors.As(es, &ps) && pa.Offset >= 0 && ps.Offset >= 0 && errors.Unwrap(pa.Err) == nil && errors.Unwrap(ps.Err) == nil && pa.Offset != ps.Offset {
			t.Fatalf("syntax offsets differ: %v / %v", ea, es)
		}
		// A streaming parser may diagnose an invalid first feature before it sees
		// later truncation or duplicate keys. Once whole-document syntax (including
		// duplicate/depth checks) and wire types pass, semantic categories must agree.
		// A later wrong feature type can otherwise outrank an earlier incomplete
		// feature in the whole-document decoder (for example [{}, ""]).
		var typed []wireFeature
		if ea != nil && json.Valid(b) && checkSyntax(b) == nil && json.Unmarshal(b, &typed) == nil {
			for _, sentinel := range []error{ErrInvalid, ErrIncomplete, ErrInvalidJSON, ErrTooLarge, ErrNoResults} {
				if errors.Is(ea, sentinel) != errors.Is(es, sentinel) {
					t.Fatalf("parser error category differs: %v / %v", ea, es)
				}
			}
		}
	})
}
