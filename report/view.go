package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// The view model holds only plain strings and numbers; the templates do all escaping.

type statusView struct{ Key, Glyph, Label string }

var statusViews = map[cucumber.Status]statusView{
	cucumber.StatusPassed:    {"passed", "✔", "Passed"},
	cucumber.StatusFailed:    {"failed", "✘", "Failed"},
	cucumber.StatusSkipped:   {"skipped", "↷", "Skipped"},
	cucumber.StatusPending:   {"pending", "…", "Pending"},
	cucumber.StatusUndefined: {"undefined", "?", "Undefined"},
	cucumber.StatusAmbiguous: {"ambiguous", "≠", "Ambiguous"},
}

type countView struct {
	Status statusView
	N      int
}

type tileView struct {
	Kind, Label string
	Total       int
	Counts      []countView // non-zero statuses only, in display order
}

type toggleView struct {
	Key, Label string
	On         bool
}

type pageView struct {
	Head, Script   template.HTML
	Title, Name    string
	Verdict        statusView
	Facts          []Fact
	Tiles          []tileView
	Sections       []sectionView
	Charts         []chartView
	Features       []featureView
	Theme, Density string
	Compact        bool
	Expand         string
	PageSize       int
	HiddenCharts   string
	HiddenSections string
	Statuses       []statusView
	ChartToggles   []toggleView
	SectionToggles []toggleView
}

type featureView struct {
	Anchor                          string
	Keyword, Name, URI, Description string
	Tags                            []string
	Status                          statusView
	Duration                        string
	DurationNS                      int64
	Scenarios                       []scenarioView
	Passed, Failed, Other, Steps    int
	Search                          string
}

type scenarioView struct {
	Anchor                     string
	Keyword, Name, Description string
	Tags                       []string
	Status                     statusView
	Duration                   string
	Open                       bool
	Steps                      []stepView
}

type stepView struct {
	Keyword, Name, Location string
	Status                  statusView
	Duration                string
	Error                   string
	DocString               *cucumber.DocString
	Rows                    [][]string
	Attachments             []attachmentView
}

type attachmentView struct {
	ID, Name, MimeType, Size string
	// Kind is "text", "json", "binary", "invalid", "omitted" or "sidecar".
	Kind string
	Text string
	Note string
	Href string
}

// clean makes data safe to place in the document's text: invalid UTF-8 becomes U+FFFD.
func clean(s string) string { return strings.ToValidUTF8(s, "�") }

func cleanTags(tags []cucumber.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = clean(t.Name)
	}
	return out
}

func buildPage(features []cucumber.Feature, opt Options, sidecar sidecarFunc) (*pageView, error) {
	d := opt.Defaults
	v := &pageView{
		Head: head, Script: script,
		Title: clean(opt.Title), Name: clean(opt.Name),
		Theme: d.Theme, Density: d.Density, Compact: d.Density == DensityCompact, Expand: d.Expand,
		PageSize: max(d.PageSize, 0),
	}
	for _, f := range opt.RunFacts {
		v.Facts = append(v.Facts, Fact{clean(f.Label), clean(f.Value)})
	}
	for _, s := range cucumber.Statuses {
		v.Statuses = append(v.Statuses, statusViews[s])
	}

	featureCounts := map[cucumber.Status]int{}
	scenarioCounts := map[cucumber.Status]int{}
	stepCounts := map[cucumber.Status]int{}
	var all cucumber.Feature
	steps := 0
	for fi, f := range features {
		fs := f.Status()
		featureCounts[fs]++
		fv := featureView{
			Anchor: "f" + strconv.Itoa(fi+1), Keyword: clean(f.Keyword), Name: clean(f.Name), URI: clean(f.URI),
			Description: clean(strings.TrimSpace(f.Description)), Tags: cleanTags(f.Tags), Status: statusViews[fs],
			Duration: cucumber.FormatDuration(f.Duration()), DurationNS: int64(f.Duration()),
		}
		search := []string{f.Name, f.URI}
		for _, t := range f.Tags {
			search = append(search, t.Name)
		}
		for si, sc := range f.Elements {
			all.Elements = append(all.Elements, sc)
			ss := sc.Status()
			scenarioCounts[ss]++
			switch ss {
			case cucumber.StatusPassed:
				fv.Passed++
			case cucumber.StatusFailed:
				fv.Failed++
			default:
				fv.Other++
			}
			search = append(search, sc.Name)
			sv := scenarioView{
				Anchor: fmt.Sprintf("f%d-s%d", fi+1, si+1), Keyword: clean(sc.Keyword), Name: clean(sc.Name),
				Description: clean(strings.TrimSpace(sc.Description)), Tags: cleanTags(sc.Tags), Status: statusViews[ss],
				Duration: cucumber.FormatDuration(sc.Duration()),
				Open:     d.Expand == ExpandAll || (d.Expand == ExpandFailed && ss == cucumber.StatusFailed),
			}
			for ti, st := range sc.Steps {
				stepCounts[st.Result.Status]++
				steps++
				fv.Steps++
				stv, err := buildStep(st, fi, si, ti, opt.Limits.InlineAttachment, sidecar)
				if err != nil {
					return nil, err
				}
				sv.Steps = append(sv.Steps, stv)
			}
			fv.Scenarios = append(fv.Scenarios, sv)
		}
		fv.Search = strings.ToLower(clean(strings.Join(search, " ")))
		v.Features = append(v.Features, fv)
	}
	v.Verdict = statusViews[all.Status()]
	v.Tiles = []tileView{
		tile("features", "Features", len(features), featureCounts),
		tile("scenarios", "Scenarios", len(all.Elements), scenarioCounts),
		tile("steps", "Steps", steps, stepCounts),
	}

	var hiddenSections []string
	for _, s := range opt.Sections {
		on, set := d.Sections[s.ID]
		hidden := set && !on
		if hidden {
			hiddenSections = append(hiddenSections, s.ID)
		}
		v.Sections = append(v.Sections, buildSection(s, hidden))
		v.SectionToggles = append(v.SectionToggles, toggleView{Key: "section:" + s.ID, Label: clean(s.Title), On: !hidden})
	}
	v.HiddenSections = strings.Join(hiddenSections, " ")

	v.Charts = buildCharts(features, opt.Sections, scenarioCounts, stepCounts)
	var hiddenCharts []string
	for i := range v.Charts {
		c := &v.Charts[i]
		on, set := d.Charts[ChartID(c.ID)]
		c.Hidden = set && !on
		if c.Hidden {
			hiddenCharts = append(hiddenCharts, c.ID)
		}
		v.ChartToggles = append(v.ChartToggles, toggleView{Key: "chart:" + c.ID, Label: c.Title, On: !c.Hidden})
	}
	v.HiddenCharts = strings.Join(hiddenCharts, " ")
	return v, nil
}

func tile(kind, label string, total int, counts map[cucumber.Status]int) tileView {
	t := tileView{Kind: kind, Label: label, Total: total}
	for _, s := range cucumber.Statuses {
		if counts[s] > 0 {
			t.Counts = append(t.Counts, countView{statusViews[s], counts[s]})
		}
	}
	return t
}

func buildStep(st cucumber.Step, fi, si, ti int, inline int64, sidecar sidecarFunc) (stepView, error) {
	v := stepView{
		Keyword: clean(strings.TrimSpace(st.Keyword)), Name: clean(st.Name), Location: clean(st.Match.Location),
		Status: statusViews[st.Result.Status], Error: clean(st.Result.Error),
	}
	if st.Result.Duration != nil {
		v.Duration = cucumber.FormatDuration(*st.Result.Duration)
	}
	if st.DocString != nil {
		v.DocString = &cucumber.DocString{Value: clean(st.DocString.Value), ContentType: clean(st.DocString.ContentType), Line: st.DocString.Line}
	}
	for _, r := range st.Rows {
		cells := make([]string, len(r.Cells))
		for i, c := range r.Cells {
			cells[i] = clean(c)
		}
		v.Rows = append(v.Rows, cells)
	}
	for ai, e := range st.Embeddings {
		id := fmt.Sprintf("attachment-f%d-s%d-t%d-a%d", fi+1, si+1, ti+1, ai+1)
		a, err := buildAttachment(e, id, inline, sidecar)
		if err != nil {
			return v, err
		}
		v.Attachments = append(v.Attachments, a)
	}
	return v, nil
}

// mediaType returns the lower-case type and subtype without parameters.
func mediaType(mime string) string {
	base, _, _ := strings.Cut(mime, ";")
	return strings.ToLower(strings.TrimSpace(base))
}

func isJSON(mt string) bool { return mt == "application/json" || strings.HasSuffix(mt, "+json") }

// buildAttachment decides how an attachment appears. Text types (text/*, including text/html, and
// JSON) that are valid UTF-8 are shown as text, JSON pretty-printed; anything else shows its name,
// type and size only. Text above the inline limit goes to a sidecar file when one can be written.
func buildAttachment(e cucumber.Embedding, id string, inline int64, sidecar sidecarFunc) (attachmentView, error) {
	a := attachmentView{ID: id, Name: clean(e.Name), MimeType: clean(e.MimeType)}
	mt := mediaType(e.MimeType)
	textual := strings.HasPrefix(mt, "text/") || isJSON(mt)
	raw, err := e.Decode(int64(len(e.Data)))
	if err != nil {
		a.Kind, a.Note = "invalid", "Attachment data is not valid base64; content not shown."
		return a, nil
	}
	a.Size = formatBytes(int64(len(raw)))
	if !textual || !utf8.Valid(raw) {
		a.Kind = "binary"
		return a, nil
	}
	if int64(len(raw)) > inline {
		if sidecar == nil {
			a.Kind, a.Note = "omitted", "Larger than the "+formatBytes(inline)+" inline limit; content not included."
			return a, nil
		}
		name := id + ".txt"
		if isJSON(mt) {
			name = id + ".json"
		}
		if err := sidecar(name, raw); err != nil {
			return a, err
		}
		a.Kind, a.Href = "sidecar", name
		return a, nil
	}
	a.Kind, a.Text = "text", string(raw)
	if isJSON(mt) {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err == nil {
			a.Kind, a.Text = "json", pretty.String()
		} else {
			a.Note = "Not valid JSON; shown as text."
		}
	}
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
