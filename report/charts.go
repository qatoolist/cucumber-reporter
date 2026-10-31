package report

import (
	"sort"
	"strconv"
	"time"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// Charts are inline SVG whose geometry is computed here. The SVG uses presentation attributes and
// classes only, never style attributes, so it works under the page's Content-Security-Policy.

type chartView struct {
	ID, Title string
	// Kind is "donut", "bars" or "stack".
	Kind   string
	Hidden bool
	// Summary is the text alternative of a donut or stack.
	Summary string
	Total   int
	Donut   []donutSegment
	Legend  []legendItem
	Bars    []barView
	Stack   []rectSegment
	Note    string
}

type donutSegment struct{ Class, Dash, Offset string }

type legendItem struct {
	Class, Glyph, Label string
	N                   int
	Percent             string
}

type barView struct {
	Label, Sub, Value, Href string
	Segments                []rectSegment
}

type rectSegment struct{ Class, X, Width string }

// num formats chart geometry with at most three decimals and no trailing zeros.
func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	for s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	if s == "-0" {
		return "0"
	}
	return s
}

func percent(n, total int) string {
	if total == 0 {
		return "0%"
	}
	return strconv.FormatFloat(float64(n)*100/float64(total), 'f', 1, 64) + "%"
}

// donut lays out status counts on a circle of circumference 100, starting at twelve o'clock.
func donut(id ChartID, title, unit string, counts map[cucumber.Status]int) chartView {
	c := chartView{ID: string(id), Title: title, Kind: "donut"}
	for _, s := range cucumber.Statuses {
		c.Total += counts[s]
	}
	c.Summary = title + ": " + strconv.Itoa(c.Total) + " " + unit
	done := 0.0
	for _, s := range cucumber.Statuses {
		n := counts[s]
		if n == 0 {
			continue
		}
		sv := statusViews[s]
		share := float64(n) * 100 / float64(c.Total)
		c.Donut = append(c.Donut, donutSegment{
			Class: "st-" + sv.Key, Dash: num(share) + " " + num(100-share), Offset: num(25 - done),
		})
		done += share
		c.Legend = append(c.Legend, legendItem{Class: "st-" + sv.Key, Glyph: sv.Glyph, Label: sv.Label, N: n, Percent: percent(n, c.Total)})
		c.Summary += ", " + strconv.Itoa(n) + " " + sv.Label
	}
	return c
}

// featureResults is one 100% stacked bar per feature: passed, failed, skipped and other scenarios.
func featureResults(features []cucumber.Feature) chartView {
	c := chartView{ID: string(ChartFeatureResults), Title: "Results per feature", Kind: "bars"}
	for fi, f := range features {
		var passed, failed, skipped, other int
		for _, sc := range f.Elements {
			switch sc.Status() {
			case cucumber.StatusPassed:
				passed++
			case cucumber.StatusFailed:
				failed++
			case cucumber.StatusSkipped:
				skipped++
			default:
				other++
			}
		}
		total := float64(len(f.Elements))
		b := barView{Label: clean(f.Name), Href: "#f" + strconv.Itoa(fi+1)}
		x := 0.0
		for _, part := range []struct {
			class string
			n     int
		}{{"st-passed", passed}, {"st-failed", failed}, {"st-skipped", skipped}, {"st-other", other}} {
			if part.n == 0 {
				continue
			}
			w := float64(part.n) * 100 / total
			b.Segments = append(b.Segments, rectSegment{Class: part.class, X: num(x), Width: num(w)})
			x += w
		}
		b.Value = strconv.Itoa(passed) + " passed · " + strconv.Itoa(failed) + " failed · " + strconv.Itoa(skipped+other) + " other"
		c.Bars = append(c.Bars, b)
	}
	return c
}

type timed struct {
	label, sub, href string
	d                time.Duration
}

// durationBars draws bars scaled to the longest item. With sorted set it keeps the top limit items,
// longest first, ties in input order.
func durationBars(id ChartID, title string, items []timed, sorted bool, limit int) chartView {
	c := chartView{ID: string(id), Title: title, Kind: "bars"}
	if sorted {
		sort.SliceStable(items, func(i, j int) bool { return items[i].d > items[j].d })
		if len(items) > limit {
			items = items[:limit]
		}
	}
	var longest time.Duration
	for _, it := range items {
		longest = max(longest, it.d)
	}
	if longest == 0 {
		c.Note = "No durations recorded."
		return c
	}
	for _, it := range items {
		b := barView{Label: it.label, Sub: it.sub, Href: it.href, Value: cucumber.FormatDuration(it.d)}
		if w := float64(it.d) * 100 / float64(longest); w > 0 {
			b.Segments = []rectSegment{{Class: "dur", X: "0", Width: num(w)}}
		}
		c.Bars = append(c.Bars, b)
	}
	return c
}

// sectionChart is a single stacked bar from a caller's ChartSeries.
func sectionChart(s Section) chartView {
	title := clean(s.Chart.Title)
	if title == "" {
		title = clean(s.Title)
	}
	c := chartView{ID: string(SectionChartID(s.ID)), Title: title, Kind: "stack", Summary: title}
	for _, seg := range s.Chart.Segments {
		c.Total += seg.Value
	}
	x := 0.0
	for _, seg := range s.Chart.Segments {
		class := "tone-" + toneOrDefault(seg.Tone)
		label := clean(seg.Label)
		c.Legend = append(c.Legend, legendItem{Class: class, Label: label, N: seg.Value, Percent: percent(seg.Value, c.Total)})
		c.Summary += ", " + strconv.Itoa(seg.Value) + " " + label
		if seg.Value == 0 {
			continue
		}
		w := float64(seg.Value) * 100 / float64(c.Total)
		c.Stack = append(c.Stack, rectSegment{Class: class, X: num(x), Width: num(w)})
		x += w
	}
	if c.Total == 0 {
		c.Note = "No values."
	}
	return c
}

// buildCharts returns every chart in display order: the built-in charts, with section charts after
// the slowest-item charts and before duration by feature.
func buildCharts(features []cucumber.Feature, sections []Section, scenarioCounts, stepCounts map[cucumber.Status]int) []chartView {
	var steps, scenarios, perFeature []timed
	for fi, f := range features {
		perFeature = append(perFeature, timed{label: clean(f.Name), href: "#f" + strconv.Itoa(fi+1), d: f.Duration()})
		for si, sc := range f.Elements {
			href := "#f" + strconv.Itoa(fi+1) + "-s" + strconv.Itoa(si+1)
			scenarios = append(scenarios, timed{label: clean(sc.Name), sub: clean(f.Name), href: href, d: sc.Duration()})
			for _, st := range sc.Steps {
				if st.Result.Duration != nil {
					steps = append(steps, timed{label: clean(st.Keyword + st.Name), sub: clean(f.Name + " / " + sc.Name), href: href, d: *st.Result.Duration})
				}
			}
		}
	}
	charts := []chartView{
		donut(ChartScenarioStatus, "Scenarios by status", "scenarios", scenarioCounts),
		donut(ChartStepStatus, "Steps by status", "steps", stepCounts),
		featureResults(features),
		durationBars(ChartSlowestSteps, "Slowest steps", steps, true, 10),
		durationBars(ChartSlowestScenarios, "Slowest scenarios", scenarios, true, 10),
	}
	for _, s := range sections {
		if s.Chart != nil {
			charts = append(charts, sectionChart(s))
		}
	}
	return append(charts, durationBars(ChartFeatureDuration, "Duration by feature", perFeature, false, 0))
}
