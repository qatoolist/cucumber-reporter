package report

import (
	"io"
	"testing"
)

func TestReportSizeBudget(t *testing.T) {
	cases := []struct {
		name                                 string
		features, scenarios, steps, maxBytes int
	}{{"small", 1, 1, 2, 240000}, {"5000 steps", 40, 25, 5, 2500000}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := syntheticRun(tc.features, tc.scenarios, tc.steps)
			b, err := renderPublicModel(f, renderOptions{Version: 2})
			if err != nil {
				t.Fatal(err)
			}
			if len(b) > tc.maxBytes {
				t.Fatalf("report %d bytes exceeds %d-byte budget", len(b), tc.maxBytes)
			}
		})
	}
}

func BenchmarkRenderToLarge(b *testing.B) {
	f := syntheticRun(40, 25, 5)
	b.ReportAllocs()
	for b.Loop() {
		if err := renderTo(io.Discard, f, renderOptions{Version: 2}); err != nil {
			b.Fatal(err)
		}
	}
}
