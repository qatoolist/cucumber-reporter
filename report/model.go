package report

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// The page's view model. It is marshaled with encoding/json (HTML-safe escaping) into the
// application/json data block that the page script reads, and the same values feed the html/template
// no-JS fallback. Fields tagged "-" serve the fallback only.

// Display statuses: the six cucumber statuses plus "known", a failure that a known issue covers.
const statusKnown = "known"

// statusOrder is the display order of the summary.
var statusOrder = []string{"passed", "failed", statusKnown, "ambiguous", "undefined", "pending", "skipped"}

var statusLabels = map[string]string{
	"passed": "✓ Passed", "failed": "✕ Failed", statusKnown: "⚑ Known failure", "ambiguous": "≠ Ambiguous",
	"undefined": "? Undefined", "pending": "… Pending", "skipped": "↷ Skipped",
}

type pageData struct {
	Schema      int           `json:"schema"`
	Title       string        `json:"title"`
	Name        string        `json:"name"`
	Verdict     Verdict       `json:"verdict"`
	Facts       []Fact        `json:"facts"`
	Top         []string      `json:"top"`
	WallClock   string        `json:"wallClock,omitempty"`
	Sections    []Section     `json:"sections"`
	Annotations []Annotation  `json:"annotations"`
	Defaults    ViewDefaults  `json:"defaults"`
	Features    []featureData `json:"features"`

	Summary []statusCount `json:"-"`
	Total   int           `json:"-"`
}

type statusCount struct {
	Status string
	N      int
}

type featureData struct {
	ID          string         `json:"id"`
	URI         string         `json:"uri"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Status      string         `json:"status"`
	Elements    []scenarioData `json:"elements"`
}

type scenarioData struct {
	ID          string     `json:"id"`
	Keyword     string     `json:"keyword"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Line        int        `json:"line"`
	Tags        []string   `json:"tags"`
	Status      string     `json:"status"`
	Steps       []stepData `json:"steps"`

	Duration int64    `json:"-"`
	Notes    []string `json:"-"`
}

type stepData struct {
	Keyword  string     `json:"keyword"`
	Name     string     `json:"name"`
	Match    string     `json:"match,omitempty"`
	Status   string     `json:"status"`
	Duration *int64     `json:"duration"`
	Error    string     `json:"error,omitempty"`
	Doc      *docData   `json:"doc,omitempty"`
	Rows     [][]string `json:"rows,omitempty"`
	Att      []attData  `json:"att,omitempty"`
	// Known marks the step that carries the known-failure marker.
	Known bool `json:"known,omitempty"`
}

type docData struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// attData is one attachment. Data is the canonical base64 text when the decoded content fits the inline
// limit; otherwise Href names a sidecar file beside the page, or Note says why the content is absent.
type attData struct {
	Name string `json:"name"`
	Mime string `json:"mime"`
	Size int64  `json:"size,omitempty"`
	Data string `json:"data,omitempty"`
	Href string `json:"href,omitempty"`
	Note string `json:"note,omitempty"`
}

// combine merges display statuses by the feature rule of cucumber.Combine, with known failures ranked
// after undefined: failed, ambiguous, undefined or known when any part has it (in that order); pending
// only when all are pending; skipped only when all are skipped or pending; otherwise passed.
func combine(statuses []string) string {
	seen := map[string]bool{}
	pending, skippedOrPending := 0, 0
	for _, s := range statuses {
		seen[s] = true
		switch s {
		case "pending":
			pending++
			skippedOrPending++
		case "skipped":
			skippedOrPending++
		}
	}
	for _, s := range []string{"failed", "ambiguous", "undefined", statusKnown} {
		if seen[s] {
			return s
		}
	}
	switch {
	case len(statuses) > 0 && pending == len(statuses):
		return "pending"
	case len(statuses) > 0 && skippedOrPending == len(statuses):
		return "skipped"
	}
	return "passed"
}

// computedVerdict is the default headline for a run status.
func computedVerdict(run string) Verdict {
	switch run {
	case "failed", "ambiguous", "undefined":
		return Verdict{Label: strings.ToUpper(run[:1]) + run[1:], Tone: ToneFail}
	case statusKnown:
		return Verdict{Label: "Passed with known failures", Tone: ToneWarn}
	case "pending", "skipped":
		return Verdict{Label: strings.ToUpper(run[:1]) + run[1:], Tone: ToneInfo}
	}
	return Verdict{Label: "Passed", Tone: ToneOK}
}

// clean replaces each byte that is not part of valid UTF-8 with U+FFFD, as encoding/json does, so the
// data block and the fallback show the same text.
func clean(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
	}
	return b.String()
}

func cleanFacts(facts []Fact) []Fact {
	if facts == nil {
		return nil
	}
	out := make([]Fact, len(facts))
	for i, f := range facts {
		out[i] = Fact{Label: clean(f.Label), Value: clean(f.Value)}
	}
	return out
}

func cleanAnnotation(a Annotation) Annotation {
	a.Ref, a.Title, a.Note = clean(a.Ref), clean(a.Title), clean(a.Note)
	if a.Outcomes != nil {
		out := make([]Outcome, len(a.Outcomes))
		for i, o := range a.Outcomes {
			out[i] = Outcome{Step: o.Step, Outcome: clean(o.Outcome), Text: clean(o.Text)}
		}
		a.Outcomes = out
	}
	return a
}

func tagNames(tags []cucumber.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = clean(t.Name)
	}
	return out
}

func buildPage(features []cucumber.Feature, opt Options, sidecar sidecarFunc) (*pageData, error) {
	sids := ScenarioIDs(features)
	fids := FeatureIDs(features)
	stepCount := map[string]int{}
	for i, f := range features {
		for j, sc := range f.Elements {
			stepCount[sids[i][j]] = len(sc.Steps)
		}
	}
	if err := checkAnnotations(opt.Annotations, stepCount); err != nil {
		return nil, err
	}
	p := &pageData{
		Schema: 1, Title: clean(opt.Title), Name: clean(opt.Name), Facts: cleanFacts(opt.RunFacts),
		WallClock: clean(opt.WallClock), Defaults: opt.Defaults,
	}
	anns := map[string][]Annotation{}
	for _, a := range opt.Annotations {
		a = cleanAnnotation(a)
		p.Annotations = append(p.Annotations, a)
		anns[a.Scenario] = append(anns[a.Scenario], a)
	}
	for _, v := range topFacts(opt) {
		p.Top = append(p.Top, clean(v))
	}
	for _, s := range opt.Sections {
		if s.Tone == "" {
			s.Tone = ToneInfo
		}
		s.Title, s.Note, s.Summary, s.Rows = clean(s.Title), clean(s.Note), cleanFacts(s.Summary), cleanFacts(s.Rows)
		p.Sections = append(p.Sections, s)
	}

	counts := map[string]int{}
	var all []string
	for i, f := range features {
		fd := featureData{
			ID: fids[i], URI: clean(f.URI), Name: clean(f.Name), Description: clean(strings.TrimSpace(f.Description)),
			Tags: tagNames(f.Tags),
		}
		var statuses []string
		for j, sc := range f.Elements {
			sd, err := buildScenario(sc, sids[i][j], anns[sids[i][j]], i, j, opt.Limits.InlineAttachment, sidecar)
			if err != nil {
				return nil, err
			}
			statuses = append(statuses, sd.Status)
			counts[sd.Status]++
			fd.Elements = append(fd.Elements, sd)
		}
		fd.Status = combine(statuses)
		all = append(all, statuses...)
		p.Features = append(p.Features, fd)
	}
	p.Verdict = computedVerdict(combine(all))
	if v := opt.Verdict; v != nil {
		tone := v.Tone
		if tone == "" {
			tone = p.Verdict.Tone
		}
		p.Verdict = Verdict{Label: clean(v.Label), Tone: tone}
	}
	for _, s := range statusOrder {
		if counts[s] > 0 {
			p.Summary = append(p.Summary, statusCount{s, counts[s]})
		}
	}
	p.Total = len(all)
	return p, nil
}

// topFacts returns the values of the facts named by TopFacts, or of the first three facts.
func topFacts(opt Options) []string {
	var out []string
	if len(opt.TopFacts) == 0 {
		for i := 0; i < len(opt.RunFacts) && i < 3; i++ {
			out = append(out, opt.RunFacts[i].Value)
		}
		return out
	}
	for _, label := range opt.TopFacts {
		for _, f := range opt.RunFacts {
			if f.Label == label {
				out = append(out, f.Value)
				break
			}
		}
	}
	return out
}

// buildScenario applies the known-issue rules documented on Annotation.
func buildScenario(sc cucumber.Scenario, id string, anns []Annotation, fi, si int, inline int64, sidecar sidecarFunc) (scenarioData, error) {
	sd := scenarioData{
		ID: id, Keyword: clean(sc.Keyword), Name: clean(sc.Name), Description: clean(strings.TrimSpace(sc.Description)),
		Line: sc.Line, Tags: tagNames(sc.Tags), Status: string(sc.Status()),
	}
	known, observed := false, map[int]bool{}
	for _, a := range anns {
		if a.Kind == KindKnown {
			known = true
			if a.Observed {
				observed[a.Step] = true
			}
		}
		sd.Notes = append(sd.Notes, annotationNote(a))
	}
	raw := sd.Status
	if known && (raw == "failed" || (raw == "passed" && len(observed) > 0)) {
		sd.Status = statusKnown
	}
	for k, st := range sc.Steps {
		d, err := buildStep(st, fi, si, k, inline, sidecar)
		if err != nil {
			return sd, err
		}
		if sd.Status == statusKnown {
			d.Known = (raw == "failed" && st.Result.Status == cucumber.StatusFailed) || (raw == "passed" && observed[k+1])
		}
		if d.Duration != nil {
			sd.Duration += *d.Duration
		}
		sd.Steps = append(sd.Steps, d)
	}
	return sd, nil
}

func annotationNote(a Annotation) string {
	label := map[string]string{KindKnown: "Known failure", KindSkip: "Skipped", KindLimit: "Not verified"}[a.Kind]
	if a.Ref != "" {
		label += " " + a.Ref
	}
	if a.Title != "" {
		label += ": " + a.Title
	}
	return label
}

func buildStep(st cucumber.Step, fi, si, ti int, inline int64, sidecar sidecarFunc) (stepData, error) {
	d := stepData{
		Keyword: clean(st.Keyword), Name: clean(st.Name), Match: clean(st.Match.Location),
		Status: string(st.Result.Status), Error: clean(st.Result.Error),
	}
	if st.Result.Duration != nil {
		ns := int64(*st.Result.Duration)
		d.Duration = &ns
	}
	if st.DocString != nil {
		d.Doc = &docData{Type: clean(st.DocString.ContentType), Value: clean(st.DocString.Value)}
	}
	for _, r := range st.Rows {
		cells := make([]string, len(r.Cells))
		for i, c := range r.Cells {
			cells[i] = clean(c)
		}
		d.Rows = append(d.Rows, cells)
	}
	for ai, e := range st.Embeddings {
		name := fmt.Sprintf("attachment-f%d-s%d-t%d-a%d", fi+1, si+1, ti+1, ai+1)
		a, err := buildAttachment(e, name, inline, sidecar)
		if err != nil {
			return d, err
		}
		d.Att = append(d.Att, a)
	}
	return d, nil
}

// mediaType returns the lower-case type and subtype without parameters.
func mediaType(mime string) string {
	base, _, _ := strings.Cut(mime, ";")
	return strings.ToLower(strings.TrimSpace(base))
}

func isJSON(mt string) bool { return mt == "application/json" || strings.HasSuffix(mt, "+json") }

// buildAttachment carries an attachment's base64 text when its decoded size fits the inline limit.
// Larger text (text/* or JSON, valid UTF-8) goes to a sidecar file when one can be written (RenderDir);
// anything else larger is left out with a note.
func buildAttachment(e cucumber.Embedding, name string, inline int64, sidecar sidecarFunc) (attData, error) {
	a := attData{Name: clean(e.Name), Mime: clean(e.MimeType)}
	raw, err := e.Decode(int64(len(e.Data)))
	if err != nil {
		a.Note = "Attachment data is not valid base64; content not shown."
		return a, nil //nolint:nilerr // one undecodable attachment is reported in the page, not as a render failure
	}
	a.Size = int64(len(raw))
	if a.Size <= inline {
		a.Data = e.Data
		return a, nil
	}
	mt := mediaType(e.MimeType)
	if sidecar != nil && (strings.HasPrefix(mt, "text/") || isJSON(mt)) && utf8.Valid(raw) {
		file := name + ".txt"
		if isJSON(mt) {
			file = name + ".json"
		}
		if err := sidecar(file, raw); err != nil {
			return a, err
		}
		a.Href = file
		return a, nil
	}
	a.Note = "Larger than the " + formatBytes(inline) + " inline limit; content not included."
	return a, nil
}

func formatBytes(n int64) string {
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return strconv.FormatFloat(float64(n)/1024, 'f', 1, 64) + " KiB"
	}
	return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MiB"
}

// fixed formats x ≥ 0 with f decimals as JavaScript's toFixed does: from the exact binary value,
// halves rounded up. strconv rounds exact halves to even, and scaling x first would round twice.
func fixed(x float64, f int) string {
	exact := strconv.FormatFloat(x, 'f', f+30, 64) // more digits than a float64 holds: exact
	dot := strings.IndexByte(exact, '.')
	keep := []byte(exact[:dot+1+f])
	if exact[dot+1+f] >= '5' {
		for i := len(keep) - 1; ; i-- {
			if i < 0 {
				keep = append([]byte{'1'}, keep...)
				break
			}
			if keep[i] == '.' {
				continue
			}
			if keep[i] < '9' {
				keep[i]++
				break
			}
			keep[i] = '0'
		}
	}
	return strings.TrimSuffix(string(keep), ".")
}

func below(s string, limit float64) bool {
	v, _ := strconv.ParseFloat(s, 64)
	return v < limit
}

// formatNS formats nanoseconds exactly as the page script's dur() does, so that the fallback and the
// app agree. Each branch is taken only when its rounded value stays below the next unit.
func formatNS(ns int64) string {
	if ns < 1e3 {
		return strconv.FormatInt(ns, 10) + " ns"
	}
	if us := math.Floor(float64(ns)/1e3 + 0.5); us < 1000 {
		return strconv.FormatFloat(us, 'f', 0, 64) + " µs"
	}
	ms := float64(ns) / 1e6
	if v := fixed(ms, 1); below(v, 10) {
		return v + " ms"
	}
	if v := math.Floor(ms + 0.5); v < 1000 {
		return strconv.FormatFloat(v, 'f', 0, 64) + " ms"
	}
	s := ms / 1000
	if v := fixed(s, 2); below(v, 10) {
		return v + " s"
	}
	if v := fixed(s, 1); below(v, 60) {
		return v + " s"
	}
	if t := int64(math.Floor(s + 0.5)); t < 3600 {
		return fmt.Sprintf("%dm %02ds", t/60, t%60)
	}
	m := int64(math.Floor(s/60 + 0.5))
	return fmt.Sprintf("%dh %02dm", m/60, m%60)
}
