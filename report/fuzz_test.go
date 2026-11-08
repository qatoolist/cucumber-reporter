package report

import (
	"strings"
	"testing"
)

// FuzzRender puts one arbitrary string into every data field. The render must succeed, keep the
// fixed markup skeleton, contain no active content, carry the string intact in the data block and read
// it back as text in the fallback.
func FuzzRender(f *testing.F) {
	for _, s := range hostile {
		f.Add(s)
	}
	f.Add("")
	f.Add("plain text")
	baseline := strings.Join(shape(tokenize(f, renderHostile(f, "placeholder"))), "\n")
	f.Fuzz(func(t *testing.T, s string) {
		if display(s) == "" {
			return // a section must have a title; TestOptionsValidation covers the refusal
		}
		features, opt := hostileRun(s)
		doc := mustRender(t, features, opt)
		tokens := tokenize(t, doc)
		checkSafe(t, tokens)
		checkDataReadsBack(t, doc, s)
		if strings.TrimSpace(display(s)) == "" || strings.TrimSpace(s) == "" {
			return // empty fields are left out of the page, which changes the skeleton legitimately
		}
		if got := strings.Join(shape(tokens), "\n"); got != baseline {
			t.Fatalf("markup skeleton changed with data %q", s)
		}
		if !strings.Contains(allText(tokens), display(s)) {
			t.Fatalf("data %q does not read back as text", s)
		}
	})
}
