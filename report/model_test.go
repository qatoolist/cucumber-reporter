package report

import (
	"encoding/json"
	"errors"
	"html"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// Independently written mirror of the data block's shape (DESIGN-UI §8); the tests decode the page's
// JSON into it rather than reusing the renderer's own types.
type (
	tData struct {
		VerdictSource string                   `json:"verdictSource"`
		Analysis      *RunSummary              `json:"analysis,omitempty"`
		Execution     map[string][]attemptData `json:"execution,omitempty"`
		TimingSection string                   `json:"timingSection,omitempty"`
		Schema        int                      `json:"schema"`
		Title         string                   `json:"title"`
		Name          string                   `json:"name"`
		Verdict       Verdict                  `json:"verdict"`
		Facts         []Fact                   `json:"facts"`
		Top           []string                 `json:"top"`
		WallClock     string                   `json:"wallClock"`
		Sections      []Section                `json:"sections"`
		Annotations   []tAnnotation            `json:"annotations"`
		Defaults      ViewDefaults             `json:"defaults"`
		Features      []tFeature               `json:"features"`
	}
	tAnnotation struct {
		Annotation
		State, Reason string
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
	if got := featureIDs(features); !reflect.DeepEqual(got, wantF) {
		t.Errorf("featureIDs = %q\nwant         %q", got, wantF)
	}
	wantS := [][]string{
		{"order-processing-L6", "order-processing-L30", "order-processing-L30-2", "order-processing-L30-3"},
		{"order-processing-2-L4"},
		{"feature-L1"},
		{"order-processing-2-2-L9"},
		{"n-code-name-L2"},
	}
	if got := scenarioIDs(features); !reflect.DeepEqual(got, wantS) {
		t.Errorf("scenarioIDs = %q\nwant          %q", got, wantS)
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

func statesOf(d tData) []string {
	out := make([]string, len(d.Annotations))
	for i, a := range d.Annotations {
		out[i] = a.State
	}
	return out
}

func annotationsOf(d tData) []Annotation {
	out := make([]Annotation, len(d.Annotations))
	for i, a := range d.Annotations {
		out[i] = a.Annotation
	}
	return out
}

func TestKnownIssueRules(t *testing.T) {
	doc := mustRender(t, knownRun(), renderOptions{Version: OptionsVersion, Annotations: knownAnnotations()})
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
	if got, want := statesOf(d), []string{"applied", "applied", "passed", "not-applied", "not-applied", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("states %q, want %q", got, want)
	}
	if !reflect.DeepEqual(annotationsOf(d), knownAnnotations()) {
		t.Errorf("annotations not passed through unchanged:\n%+v", d.Annotations)
	}
	if d.Verdict != (Verdict{Label: "Failed: 1 unexpected failure", Tone: ToneFail}) {
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
	d, _ = dataBlock(t, mustRender(t, run, renderOptions{Version: OptionsVersion, Annotations: anns}))
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
		_, err := renderPublicModel(knownRun(), renderOptions{Version: OptionsVersion, Annotations: []Annotation{a}})
		if !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	dup := []Annotation{{Scenario: "k-L3", Kind: KindKnown, Ref: "EX-1"}, {Scenario: "k-L3", Kind: KindKnown, Ref: "EX-1"}}
	if _, err := renderPublicModel(knownRun(), renderOptions{Version: OptionsVersion, Annotations: dup}); !errors.Is(err, ErrInvalidOptions) {
		t.Errorf("duplicate known issue: err = %v", err)
	}
	two := []Annotation{{Scenario: "k-L3", Kind: KindKnown, Ref: "EX-1"}, {Scenario: "k-L3", Kind: KindKnown, Ref: "EX-2"}}
	if _, err := renderPublicModel(knownRun(), renderOptions{Version: OptionsVersion, Annotations: two}); err != nil {
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
	if d.Verdict != (Verdict{Label: "Failed: 2 unexpected failures", Tone: ToneFail}) {
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
	d, _ := dataBlock(t, mustRender(t, features, renderOptions{Version: OptionsVersion, Limits: pub.Limits{InlineAttachment: 1024}}))
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
	d, _ := dataBlock(t, mustRender(t, run, renderOptions{Version: OptionsVersion, Annotations: anns}))
	if got, want := statesOf(d), []string{"not-applied", "applied", "not-applied", "not-applied"}; !reflect.DeepEqual(got, want) {
		t.Errorf("states %q, want %q", got, want)
	}
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
	doc := mustRender(t, run, renderOptions{Version: OptionsVersion, Annotations: anns})
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
	if got, want := statesOf(d), []string{"applied", "not-applied", "not-applied", "passed", "invalid"}; !reflect.DeepEqual(got, want) {
		t.Errorf("states %q, want %q", got, want)
	}
	if d.Verdict != (Verdict{Label: "Failed: 3 unexpected failures", Tone: ToneFail}) {
		t.Errorf("verdict %+v", d.Verdict)
	}
	text := html.UnescapeString(noscript(t, doc))
	for _, want := range []string{
		"Known issue, not applied EX-9: verify is broken (The failure is at step 1, not at the declared step 3, so it counts as an unexpected failure)",
		"Known issue, not applied EX-9: verify is broken (The failure is at step 4, not at the declared step 3, so it counts as an unexpected failure)",
		"Known issue, not applied EX-9 (Declared step 3 does not match the recorded step text, so this record covers nothing)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fallback lacks %q", want)
		}
	}
}

// TestScenarioOutcomes: the exported classifier returns exactly the outcomes the page shows.
func TestScenarioOutcomes(t *testing.T) {
	features, opt := loadShowcase(t, "")
	got, err := scenarioOutcomes(features, opt.Annotations)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := dataBlock(t, mustRender(t, features, opt))
	counts := map[string]int{}
	for i, f := range d.Features {
		for j, s := range f.Elements {
			if got[i][j] != s.Status {
				t.Errorf("%s: scenarioOutcomes %s, page %s", s.ID, got[i][j], s.Status)
			}
			counts[got[i][j]]++
		}
	}
	want := map[string]int{OutcomePassed: 27, OutcomeKnown: 4, OutcomeFailed: 3, OutcomePending: 2, OutcomeUndefined: 1, OutcomeAmbiguous: 1, OutcomeSkipped: 2}
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("counts %v, want %v", counts, want)
	}
	// The known variant is a pass with known failures only.
	kf, kopt := loadShowcase(t, "-known")
	kout, err := scenarioOutcomes(kf, kopt.Annotations)
	if err != nil {
		t.Fatal(err)
	}
	kc := map[string]int{}
	for _, f := range kout {
		for _, o := range f {
			kc[o]++
		}
	}
	if !reflect.DeepEqual(kc, map[string]int{OutcomePassed: 34, OutcomeKnown: 4, OutcomeSkipped: 2}) {
		t.Errorf("known variant counts %v", kc)
	}
	if _, err := scenarioOutcomes(features, []Annotation{{Scenario: "nope", Kind: KindKnown}}); !errors.Is(err, ErrInvalidOptions) {
		t.Errorf("unknown scenario: %v", err)
	}
	if _, err := scenarioOutcomes(nil, nil); err != nil {
		t.Errorf("no features: %v", err)
	}
}

// TestComputedHeadlineCountsUnexpectedFailures: without a caller verdict a failed run is labeled with
// its number of unexpected failures; known failures are not counted.
func TestComputedHeadlineCountsUnexpectedFailures(t *testing.T) {
	features, opt := loadShowcase(t, "")
	opt.Verdict = nil
	d, _ := dataBlock(t, mustRender(t, features, opt))
	if d.Verdict != (Verdict{Label: "Failed: 3 unexpected failures", Tone: ToneFail}) {
		t.Errorf("verdict %+v", d.Verdict)
	}
}

// TestNotApplicableOutcomes: a passed scenario whose record has only not-applicable outcomes reads
// "not applicable in this run", not "the issue may be fixed"; one applicable outcome keeps the flag.
func TestNotApplicableOutcomes(t *testing.T) {
	run := []cucumber.Feature{{URI: "n.feature", Keyword: "Feature", Name: "N", Elements: []cucumber.Scenario{
		{Keyword: "Scenario", Name: "out of scope", Line: 1, Steps: []cucumber.Step{step("a", "passed", 1), step("b", "passed", 1)}},
		{Keyword: "Scenario", Name: "mixed", Line: 2, Steps: []cucumber.Step{step("a", "passed", 1), step("b", "passed", 1)}},
	}}}
	anns := []Annotation{
		{Scenario: "n-L1", Kind: KindKnown, Ref: "EX-7", Step: 2, StepText: "b", Outcomes: []Outcome{{Step: 2, Outcome: "not applicable", NotApplicable: true}}},
		{Scenario: "n-L2", Kind: KindKnown, Ref: "EX-7", Step: 2, StepText: "b", Outcomes: []Outcome{{Step: 1, Outcome: "not applicable", NotApplicable: true}, {Step: 2, Outcome: "passed unexpectedly"}}},
	}
	doc := mustRender(t, run, renderOptions{Version: OptionsVersion, Annotations: anns})
	d, _ := dataBlock(t, doc)
	if got, want := statesOf(d), []string{"not-applicable", "passed"}; !reflect.DeepEqual(got, want) {
		t.Errorf("states %q, want %q", got, want)
	}
	if !d.Annotations[0].Outcomes[0].NotApplicable || d.Annotations[0].Reason != "Not applicable in this run." {
		t.Errorf("annotation %+v", d.Annotations[0])
	}
	for _, s := range d.Features[0].Elements {
		if s.Status != "passed" || len(markedSteps(s)) != 0 {
			t.Errorf("%s: %s, marked %v", s.ID, s.Status, markedSteps(s))
		}
	}
	if fb := html.UnescapeString(noscript(t, doc)); !strings.Contains(fb, "<p>Known issue EX-7 · not applicable in this run</p>") || strings.Contains(fb, "Known failure EX-7") {
		t.Error("fallback lacks the not-applicable note or calls a passed scenario a known failure")
	}
}

// TestEmbeddingLimitOnEveryPath: Limits.Embedding bounds the encoded attachment text on the direct
// render paths too, before anything is decoded, as Parse bounds it for JSON input.
func TestEmbeddingLimitOnEveryPath(t *testing.T) {
	const secret = "long attachment SECRET-CONTENT"
	features := one(step("s", cucumber.StatusPassed, 1, cucumber.Embedding{Name: "probe", MimeType: "text/plain", Data: b64(secret)}))
	opt := renderOptions{Version: OptionsVersion, Limits: pub.Limits{Embedding: 4}}
	if _, err := renderPublicModel(features, opt); !errors.Is(err, cucumber.ErrTooLarge) || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("renderPublicModel: %v", err)
	}
	parent := t.TempDir()
	if _, err := renderFeaturesDirTo(features, parent, opt); !errors.Is(err, cucumber.ErrTooLarge) {
		t.Errorf("renderFeaturesDirTo: %v", err)
	}
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Errorf("a refused render left %d entries", len(entries))
	}
	// At the limit the attachment is accepted.
	opt.Limits.Embedding = int64(len(b64(secret)))
	if _, err := renderPublicModel(features, opt); err != nil {
		t.Errorf("at the limit: %v", err)
	}
}

// TestScenarioDurationSaturates: the fallback's scenario duration saturates like
// cucumber.Scenario.Duration instead of overflowing.
func TestScenarioDurationSaturates(t *testing.T) {
	features := one(step("a", "passed", math.MaxInt64), step("b", "passed", math.MaxInt64))
	if got := features[0].Elements[0].Duration(); got != math.MaxInt64 {
		t.Fatalf("cucumber duration %d", got)
	}
	doc := mustRender(t, features, renderOptions{Version: OptionsVersion})
	if !strings.Contains(noscript(t, doc), "Scenario: <bdi>S</bdi> · "+formatNS(math.MaxInt64)) {
		t.Errorf("fallback scenario duration is not the saturated maximum %q", formatNS(math.MaxInt64))
	}
}

// TestEmptyAttachmentIsContent: an empty attachment is carried as empty data, not as missing data.
func TestEmptyAttachmentIsContent(t *testing.T) {
	doc := mustRender(t, one(step("s", "passed", 1, cucumber.Embedding{Name: "empty.txt", MimeType: "text/plain", Data: ""})), renderOptions{Version: OptionsVersion})
	if _, raw := dataBlock(t, doc); !strings.Contains(raw, `{"name":"empty.txt","mime":"text/plain","data":""}`) {
		t.Errorf("empty attachment not carried as empty data")
	}
}
