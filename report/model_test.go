package report

import (
	"encoding/json"
	"errors"
	"html"
	"reflect"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// Independently written mirror of the data block's shape (DESIGN-UI §8); the tests decode the page's
// JSON into it rather than reusing the renderer's own types.
type (
	tData struct {
		Schema      int           `json:"schema"`
		Title       string        `json:"title"`
		Name        string        `json:"name"`
		Verdict     Verdict       `json:"verdict"`
		Facts       []Fact        `json:"facts"`
		Top         []string      `json:"top"`
		WallClock   string        `json:"wallClock"`
		Sections    []Section     `json:"sections"`
		Annotations []tAnnotation `json:"annotations"`
		Defaults    ViewDefaults  `json:"defaults"`
		Features    []tFeature    `json:"features"`
	}
	tAnnotation struct {
		Annotation
		Invalid bool `json:"invalid"`
	}
	tFeature struct {
		ID, URI, Name, Description, Status string
		Tags                               []string
		Elements                           []tScenario
	}
	tScenario struct {
		ID, Keyword, Name, Description, Status string
		Line                                   int
		Tags                                   []string
		Steps                                  []tStep
	}
	tStep struct {
		Keyword, Name, Match, Status, Error string
		Duration                            *int64
		Known                               bool
		Doc                                 *struct{ Type, Value string }
		Rows                                [][]string
		Att                                 []tAtt
	}
	tAtt struct {
		Name, Mime, Data, Href, Note string
		Size                         int64
	}
)

// dataBlock extracts and decodes the page's JSON data block.
func dataBlock(t testing.TB, doc string) (tData, string) {
	t.Helper()
	const open = `<script type="application/json" id="report-data">`
	i := strings.Index(doc, open)
	if i < 0 || strings.Count(doc, open) != 1 {
		t.Fatal("want exactly one report-data block")
	}
	raw := doc[i+len(open):]
	raw = raw[:strings.Index(raw, "</script>")]
	var d tData
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		t.Fatalf("data block: %v", err)
	}
	return d, raw
}

func scenarioByID(t testing.TB, d tData, id string) tScenario {
	t.Helper()
	for _, f := range d.Features {
		for _, s := range f.Elements {
			if s.ID == id {
				return s
			}
		}
	}
	t.Fatalf("no scenario %s", id)
	return tScenario{}
}

func markedSteps(s tScenario) []int {
	var out []int
	for i, st := range s.Steps {
		if st.Known {
			out = append(out, i+1)
		}
	}
	return out
}

func TestIDs(t *testing.T) {
	features := []cucumber.Feature{
		{URI: "features/orders/Order_Processing.feature:12", Elements: []cucumber.Scenario{{Line: 6}, {Line: 30}, {Line: 30}, {Line: 30}}},
		{URI: "other/order-processing.feature", Elements: []cucumber.Scenario{{Line: 4}}},
		{URI: "", Elements: []cucumber.Scenario{{Line: 1}}},
		{URI: "x/order-processing-2.feature", Elements: []cucumber.Scenario{{Line: 9}}},
		{URI: `C:\suite\Ünïcode Name.feature`, Elements: []cucumber.Scenario{{Line: 2}}},
	}
	wantF := []string{"order-processing", "order-processing-2", "feature", "order-processing-2-2", "n-code-name"}
	if got := FeatureIDs(features); !reflect.DeepEqual(got, wantF) {
		t.Errorf("FeatureIDs = %q\nwant         %q", got, wantF)
	}
	wantS := [][]string{
		{"order-processing-L6", "order-processing-L30", "order-processing-L30-2", "order-processing-L30-3"},
		{"order-processing-2-L4"},
		{"feature-L1"},
		{"order-processing-2-2-L9"},
		{"n-code-name-L2"},
	}
	if got := ScenarioIDs(features); !reflect.DeepEqual(got, wantS) {
		t.Errorf("ScenarioIDs = %q\nwant          %q", got, wantS)
	}
}

func knownRun() []cucumber.Feature {
	return []cucumber.Feature{{URI: "features/k.feature", Keyword: "Feature", Name: "K", Elements: []cucumber.Scenario{
		{Keyword: "Scenario", Name: "raw failure", Line: 3, Steps: []cucumber.Step{step("a", "passed", 1), step("b", "failed", 1), step("c", "skipped", -1)}},
		{Keyword: "Scenario", Name: "observed", Line: 9, Steps: []cucumber.Step{step("a", "passed", 1), step("b", "passed", 1), step("c", "passed", 1), step("d", "passed", 1)}},
		{Keyword: "Scenario", Name: "unobserved", Line: 15, Steps: []cucumber.Step{step("a", "passed", 1)}},
		{Keyword: "Scenario", Name: "undefined", Line: 20, Steps: []cucumber.Step{step("a", "undefined", -1)}},
		{Keyword: "Scenario", Name: "failure with declared step", Line: 25, Steps: []cucumber.Step{step("a", "passed", 1), step("b", "failed", 1), step("c", "skipped", -1)}},
	}}}
}

func knownAnnotations() []Annotation {
	return []Annotation{
		{Scenario: "k-L3", Kind: KindKnown, Ref: "EX-1", Title: "raw", Step: 2, StepText: "b"},
		{Scenario: "k-L9", Kind: KindKnown, Ref: "EX-2", Title: "declared", Step: 3, StepText: "c", Observed: true, Outcomes: []Outcome{{Step: 2, Outcome: "not applicable"}, {Step: 3, Outcome: "expected failure", Text: "c"}}},
		{Scenario: "k-L15", Kind: KindKnown, Ref: "EX-3", Step: 1, StepText: "a"},
		{Scenario: "k-L20", Kind: KindKnown, Ref: "EX-4"},
		{Scenario: "k-L25", Kind: KindKnown, Ref: "EX-5", Step: 1, StepText: "a", Observed: true},
		{Scenario: "k-L15", Kind: KindSkip, Title: "why"},
	}
}

func annotationsOf(d tData) []Annotation {
	out := make([]Annotation, len(d.Annotations))
	for i, a := range d.Annotations {
		out[i] = a.Annotation
	}
	return out
}

func TestKnownIssueRules(t *testing.T) {
	doc := mustRender(t, knownRun(), Options{Version: OptionsVersion, Annotations: knownAnnotations()})
	d, _ := dataBlock(t, doc)
	for _, tc := range []struct {
		id, status string
		marked     []int
	}{
		{"k-L3", "known", []int{2}},
		{"k-L9", "known", []int{3}},
		{"k-L15", "passed", nil},
		{"k-L20", "undefined", nil},
		{"k-L25", "failed", nil}, // failed at step 2, declared step 1: unexpected
	} {
		s := scenarioByID(t, d, tc.id)
		if s.Status != tc.status || !reflect.DeepEqual(markedSteps(s), tc.marked) {
			t.Errorf("%s: status %s, marked %v; want %s, %v", tc.id, s.Status, markedSteps(s), tc.status, tc.marked)
		}
		for _, st := range s.Steps {
			if st.Status == "known" {
				t.Errorf("%s: raw step status replaced", tc.id)
			}
		}
	}
	if d.Features[0].Status != "failed" {
		t.Errorf("feature status %s; an unexpected failure outranks known", d.Features[0].Status)
	}
	if !reflect.DeepEqual(annotationsOf(d), knownAnnotations()) {
		t.Errorf("annotations not passed through unchanged:\n%+v", d.Annotations)
	}
	if d.Verdict != (Verdict{Label: "Failed", Tone: ToneFail}) {
		t.Errorf("verdict = %+v", d.Verdict)
	}

	// Without the undefined and the unexpectedly failed scenarios the run passes with known failures.
	run := knownRun()
	run[0].Elements = run[0].Elements[:3]
	var anns []Annotation
	for _, a := range knownAnnotations() {
		if a.Scenario != "k-L20" && a.Scenario != "k-L25" {
			anns = append(anns, a)
		}
	}
	d, _ = dataBlock(t, mustRender(t, run, Options{Version: OptionsVersion, Annotations: anns}))
	if d.Features[0].Status != "known" || d.Verdict != (Verdict{Label: "Passed with known failures", Tone: ToneWarn}) {
		t.Errorf("feature %s, verdict %+v", d.Features[0].Status, d.Verdict)
	}
}

func TestAnnotationValidation(t *testing.T) {
	cases := map[string]Annotation{
		"unknown scenario":       {Scenario: "k-L99", Kind: KindKnown},
		"bad kind":               {Scenario: "k-L3", Kind: "bug"},
		"step too large":         {Scenario: "k-L3", Kind: KindKnown, Step: 4},
		"negative step":          {Scenario: "k-L3", Kind: KindKnown, Step: -1},
		"outcome step too large": {Scenario: "k-L3", Kind: KindKnown, Outcomes: []Outcome{{Step: 9, Outcome: "x"}}},
		"observed without step":  {Scenario: "k-L3", Kind: KindKnown, Observed: true},
		"step without text":      {Scenario: "k-L3", Kind: KindKnown, Step: 2},
		"text without step":      {Scenario: "k-L3", Kind: KindKnown, StepText: "b"},
		"step on a skip":         {Scenario: "k-L3", Kind: KindSkip, Step: 1},
	}
	for name, a := range cases {
		_, err := Render(knownRun(), Options{Version: OptionsVersion, Annotations: []Annotation{a}})
		if !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	dup := []Annotation{{Scenario: "k-L3", Kind: KindKnown, Ref: "EX-1"}, {Scenario: "k-L3", Kind: KindKnown, Ref: "EX-1"}}
	if _, err := Render(knownRun(), Options{Version: OptionsVersion, Annotations: dup}); !errors.Is(err, ErrInvalidOptions) {
		t.Errorf("duplicate known issue: err = %v", err)
	}
	two := []Annotation{{Scenario: "k-L3", Kind: KindKnown, Ref: "EX-1"}, {Scenario: "k-L3", Kind: KindKnown, Ref: "EX-2"}}
	if _, err := Render(knownRun(), Options{Version: OptionsVersion, Annotations: two}); err != nil {
		t.Errorf("two issues on one scenario: %v", err)
	}
}

func TestDataBlock(t *testing.T) {
	opt := sampleOptions()
	doc := mustRender(t, loadFixture(t, "godog-v0.16.0.json"), opt)
	d, raw := dataBlock(t, doc)
	if d.Schema != 1 || d.Title != "Example suite" || d.Name != "Example suite execution results" {
		t.Errorf("header fields: %d %q %q", d.Schema, d.Title, d.Name)
	}
	if !reflect.DeepEqual(d.Facts, opt.RunFacts) || !reflect.DeepEqual(d.Top, []string{"2026-01-02T03:04:05Z", "0123456789ab", "go1.27.0"}) {
		t.Errorf("facts %v, top %v", d.Facts, d.Top)
	}
	if d.Verdict != (Verdict{Label: "Failed", Tone: ToneFail}) {
		t.Errorf("verdict %+v", d.Verdict)
	}
	if len(d.Sections) != 2 || d.Sections[0].Role != RoleKnownIssues || d.Sections[1].Tone != ToneInfo {
		t.Errorf("sections %+v", d.Sections)
	}
	want := ViewDefaults{
		Theme: "auto", Density: "comfortable", Expand: "failed", View: "dashboard",
		Widgets: map[string]bool{"hero": true, "kpis": true, "status": true, "features": true, "attention": true, "known": true, "duration": false, "sections": true},
		Nav:     map[string]bool{"dashboard": true, "features": true, "scenarios": true, "failures": true, "known": true, "skipped": true, "timeline": false, "run": true},
	}
	if !reflect.DeepEqual(d.Defaults, want) {
		t.Errorf("defaults %+v", d.Defaults)
	}
	if len(d.Features) != 2 || d.Features[0].ID != "billing" || d.Features[0].Status != "ambiguous" || d.Features[1].Status != "failed" {
		t.Fatalf("features %+v", d.Features)
	}
	s := d.Features[0].Elements[0]
	if s.ID != "billing-L2" || s.Status != "pending" || s.Steps[0].Match != "main_test.go:32" || s.Steps[0].Duration == nil || s.Steps[1].Duration != nil {
		t.Errorf("first scenario %+v", s)
	}
	for _, banned := range []string{"<", ">", "&", "\u2028", "\u2029"} {
		if strings.Contains(raw, banned) {
			t.Errorf("data block contains %q unescaped", banned)
		}
	}

	opt.WallClock, opt.TopFacts = "9m 35s", []string{"Go version"}
	opt.Verdict = &Verdict{Label: "Passed with known failures"}
	d, _ = dataBlock(t, mustRender(t, loadFixture(t, "godog-v0.16.0.json"), opt))
	if d.WallClock != "9m 35s" || !reflect.DeepEqual(d.Top, []string{"go1.27.0"}) || d.Verdict != (Verdict{Label: "Passed with known failures", Tone: ToneFail}) {
		t.Errorf("wall clock %q, top %v, verdict %+v", d.WallClock, d.Top, d.Verdict)
	}
}

func TestAttachments(t *testing.T) {
	features := one(step("s", cucumber.StatusFailed, 1,
		cucumber.Embedding{Name: "note.txt", MimeType: "text/plain", Data: b64("line 1\n  line <2>\n")},
		cucumber.Embedding{Name: "pixel.png", MimeType: "image/png", Data: b64("\x89PNG\x00\x01")},
		cucumber.Embedding{Name: "bad.txt", MimeType: "text/plain", Data: "%%%"},
		cucumber.Embedding{Name: "big.json", MimeType: "application/json", Data: b64(`{"a":"` + strings.Repeat("x", 2000) + `"}`)},
		cucumber.Embedding{Name: "big.bin", MimeType: "application/octet-stream", Data: b64(strings.Repeat("\x00", 2000))},
	))
	d, _ := dataBlock(t, mustRender(t, features, Options{Version: OptionsVersion, Limits: cucumber.Limits{InlineAttachment: 1024}}))
	att := d.Features[0].Elements[0].Steps[0].Att
	want := []tAtt{
		{Name: "note.txt", Mime: "text/plain", Data: b64("line 1\n  line <2>\n"), Size: 18},
		{Name: "pixel.png", Mime: "image/png", Data: b64("\x89PNG\x00\x01"), Size: 6},
		{Name: "bad.txt", Mime: "text/plain", Note: "Attachment data is not valid base64; content not shown."},
		{Name: "big.json", Mime: "application/json", Size: 2008, Note: "Larger than the 1.0 KiB inline limit; content not included."},
		{Name: "big.bin", Mime: "application/octet-stream", Size: 2000, Note: "Larger than the 1.0 KiB inline limit; content not included."},
	}
	if !reflect.DeepEqual(att, want) {
		t.Errorf("attachments\n got %+v\nwant %+v", att, want)
	}
}

// TestFormatNS pins the duration format the fallback shares with the page script, including the
// unit boundaries where rounding must carry into the next unit.
func TestFormatNS(t *testing.T) {
	for ns, want := range map[int64]string{
		0: "0 ns", 999: "999 ns", 1000: "1 µs", 999_499: "999 µs", 999_500: "1.0 ms", 1_250_000: "1.3 ms",
		9_949_999: "9.9 ms", 9_950_000: "9.9 ms", 9_950_001: "10 ms", 999_499_999: "999 ms", 999_500_000: "1.00 s",
		9_994_999_999: "9.99 s", 9_999_000_001: "10.0 s", 59_940_000_000: "59.9 s", 59_960_000_000: "1m 00s",
		119_600_000_000: "2m 00s", 754_000_000_000: "12m 34s", 3_599_400_000_000: "59m 59s",
		3_599_700_000_000: "1h 00m", 7_530_000_000_000: "2h 06m",
	} {
		if got := formatNS(ns); got != want {
			t.Errorf("formatNS(%d) = %q, want %q", ns, got, want)
		}
	}
}

func TestFixedMatchesToFixed(t *testing.T) {
	// Expected values from Node's Number.prototype.toFixed.
	for _, c := range []struct {
		x    float64
		f    int
		want string
	}{{9.95, 1, "9.9"}, {1.25, 1, "1.3"}, {0.125, 2, "0.13"}, {9.995, 2, "9.99"}, {99.96, 1, "100.0"}, {0.5, 0, "1"}, {2.5, 0, "3"}, {59.95, 1, "60.0"}, {1.005, 2, "1.00"}} {
		if got := fixed(c.x, c.f); got != c.want {
			t.Errorf("fixed(%v, %d) = %q, want %q", c.x, c.f, got, c.want)
		}
	}
}

// TestKnownIssueRuleEdges: two issues on one passed scenario, only one observed, mark only the observed
// declared step; an observed issue on a pending or skipped scenario changes nothing.
func TestKnownIssueRuleEdges(t *testing.T) {
	run := []cucumber.Feature{{URI: "e.feature", Keyword: "Feature", Name: "E", Elements: []cucumber.Scenario{
		{Keyword: "Scenario", Name: "two issues", Line: 2, Steps: []cucumber.Step{step("a", "passed", 1), step("b", "passed", 1), step("c", "passed", 1)}},
		{Keyword: "Scenario", Name: "pending", Line: 8, Steps: []cucumber.Step{step("a", "passed", 1), step("b", "pending", -1)}},
		{Keyword: "Scenario", Name: "skipped", Line: 12, Steps: []cucumber.Step{step("a", "skipped", -1)}},
	}}}
	anns := []Annotation{
		{Scenario: "e-L2", Kind: KindKnown, Ref: "EX-1", Step: 1, StepText: "a"},
		{Scenario: "e-L2", Kind: KindKnown, Ref: "EX-2", Step: 3, StepText: "c", Observed: true},
		{Scenario: "e-L8", Kind: KindKnown, Ref: "EX-3", Step: 2, StepText: "b", Observed: true},
		{Scenario: "e-L12", Kind: KindKnown, Ref: "EX-4", Step: 1, StepText: "a", Observed: true},
	}
	d, _ := dataBlock(t, mustRender(t, run, Options{Version: OptionsVersion, Annotations: anns}))
	for _, tc := range []struct {
		id, status string
		marked     []int
	}{{"e-L2", "known", []int{3}}, {"e-L8", "pending", nil}, {"e-L12", "skipped", nil}} {
		s := scenarioByID(t, d, tc.id)
		if s.Status != tc.status || !reflect.DeepEqual(markedSteps(s), tc.marked) {
			t.Errorf("%s: status %s, marked %v; want %s, %v", tc.id, s.Status, markedSteps(s), tc.status, tc.marked)
		}
	}
}

// TestKnownIssueDeclaredStep: a known issue covers only its declared step. A failure there is known;
// a failure at an earlier or a later step is unexpected; a passed scenario stays passed; a record whose
// step text does not match is invalid and covers nothing.
func TestKnownIssueDeclaredStep(t *testing.T) {
	steps := func(failAt int) []cucumber.Step {
		var out []cucumber.Step
		for i, name := range []string{"open", "submit", "verify", "close"} {
			st := step(name, "passed", 1)
			switch {
			case i+1 == failAt:
				st.Result.Status, st.Result.Error = "failed", "boom"
			case failAt > 0 && i+1 > failAt:
				st.Result.Status, st.Result.Duration = "skipped", nil
			}
			out = append(out, st)
		}
		return out
	}
	run := []cucumber.Feature{{URI: "d.feature", Keyword: "Feature", Name: "D", Elements: []cucumber.Scenario{
		{Keyword: "Scenario", Name: "at declared", Line: 1, Steps: steps(3)},
		{Keyword: "Scenario", Name: "earlier", Line: 2, Steps: steps(1)},
		{Keyword: "Scenario", Name: "later", Line: 3, Steps: steps(4)},
		{Keyword: "Scenario", Name: "passed", Line: 4, Steps: steps(0)},
		{Keyword: "Scenario", Name: "text mismatch", Line: 5, Steps: steps(3)},
	}}}
	var anns []Annotation
	for _, id := range []string{"d-L1", "d-L2", "d-L3", "d-L4"} {
		anns = append(anns, Annotation{Scenario: id, Kind: KindKnown, Ref: "EX-9", Title: "verify is broken", Step: 3, StepText: "verify"})
	}
	anns = append(anns, Annotation{Scenario: "d-L5", Kind: KindKnown, Ref: "EX-9", Step: 3, StepText: "submit"})
	doc := mustRender(t, run, Options{Version: OptionsVersion, Annotations: anns})
	d, _ := dataBlock(t, doc)
	for _, tc := range []struct {
		id, status string
		marked     []int
	}{
		{"d-L1", "known", []int{3}},
		{"d-L2", "failed", nil},
		{"d-L3", "failed", nil},
		{"d-L4", "passed", nil},
		{"d-L5", "failed", nil},
	} {
		s := scenarioByID(t, d, tc.id)
		if s.Status != tc.status || !reflect.DeepEqual(markedSteps(s), tc.marked) {
			t.Errorf("%s: status %s, marked %v; want %s, %v", tc.id, s.Status, markedSteps(s), tc.status, tc.marked)
		}
	}
	for i, a := range d.Annotations {
		if a.Invalid != (i == 4) {
			t.Errorf("annotation %d invalid = %v", i, a.Invalid)
		}
	}
	if d.Verdict != (Verdict{Label: "Failed", Tone: ToneFail}) {
		t.Errorf("verdict %+v", d.Verdict)
	}
	text := html.UnescapeString(noscript(t, doc))
	for _, want := range []string{
		"Known failure EX-9: verify is broken (the failure is at step 1, not at declared step 3: unexpected)",
		"Known failure EX-9: verify is broken (the failure is at step 4, not at declared step 3: unexpected)",
		"Known failure EX-9 (declared step 3 does not match the recorded step text; this record covers nothing)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fallback lacks %q", want)
		}
	}
}
