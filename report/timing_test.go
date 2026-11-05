package report

import (
	"errors"
	"strings"
	"testing"
)

func TestElapsedTimeAndDetailSectionAreNativeOptions(t *testing.T) {
	features, opt := loadShowcase(t, "")
	opt.WallClock = "19m26s"
	opt.TimingSection = "execution-time"
	opt.Sections = append(opt.Sections, Section{ID: "execution-time", Title: "Execution time", Rows: []Fact{{Label: "Setup", Value: "2m"}}})
	page := mustRender(t, features, opt)
	if !strings.Contains(page, `"timingSection":"execution-time"`) || !strings.Contains(page, `"wallClock":"19m26s"`) || !strings.Contains(page, "Total elapsed time") || !strings.Contains(page, "Summed step time") {
		t.Fatal("native timing presentation missing")
	}
	if !strings.Contains(page, "encodeURIComponent(D.timingSection)") {
		t.Fatal("timing section navigation missing")
	}
	opt.TimingSection = "missing"
	if _, err := normalize(opt); !errors.Is(err, ErrInvalidOptions) {
		t.Fatal("accepted nonexistent detail section")
	}
	// Even when the reference and section agree, section IDs must be valid.
	opt.TimingSection = "../unsafe"
	opt.Sections = []Section{{ID: "../unsafe", Title: "Invalid ID"}}
	if _, err := normalize(opt); !errors.Is(err, ErrInvalidOptions) {
		t.Fatal("accepted invalid section ID with a matching timing reference")
	}
}

func TestWallClockWhitespaceIsNormalized(t *testing.T) {
	features, base := loadShowcase(t, "")
	for _, input := range []string{"", " \t\n\u00a0", " 19m26s \n"} {
		t.Run(input, func(t *testing.T) {
			opt := base
			opt.WallClock = input
			got, err := normalize(opt)
			if err != nil || got.WallClock != strings.TrimSpace(input) {
				t.Fatalf("normalized wall clock = %q, error = %v", got.WallClock, err)
			}
			page := mustRender(t, features, opt)
			if got.WallClock == "" {
				if strings.Contains(page, `"wallClock":`) {
					t.Fatal("blank elapsed time was emitted")
				}
			} else if !strings.Contains(page, `"wallClock":"19m26s"`) {
				t.Fatal("trimmed elapsed time missing from report data")
			}
		})
	}
}
