package report

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// OptionsVersion is the only Options.Version this release accepts.
const OptionsVersion = 1

// Errors returned by Render, RenderDir and RenderDirTo, in addition to the cucumber package's parse
// errors. Messages never contain report data.
var (
	// ErrOptionsVersion: Options.Version is not OptionsVersion.
	ErrOptionsVersion = errors.New("report: unsupported options version")
	// ErrInvalidOptions: an option value is outside its documented set.
	ErrInvalidOptions = errors.New("report: invalid options")
	// ErrNoInput: the run directory has no cucumber.json.
	ErrNoInput = errors.New("report: run directory has no cucumber.json")
	// ErrNotRegular: cucumber.json or a directory argument is a symbolic link, or not of the expected kind.
	ErrNotRegular = errors.New("report: input is a symbolic link or not a regular file or directory")
	// ErrEmptyOutput: rendering produced no bytes.
	ErrEmptyOutput = errors.New("report: renderer produced no report")
)

// Options configure one render. The zero value is not valid: Version must be set.
type Options struct {
	// Version is the options schema version; it must be OptionsVersion (1).
	Version int
	// Title is the document title. Default "Cucumber report".
	Title string
	// Name is the heading shown on the page. Default: Title.
	Name string
	// RunFacts are shown, in order, in the run information panel.
	RunFacts []Fact
	// Sections are caller-owned panels shown, in order, on the dashboard.
	Sections []Section
	// Defaults are the generation-time view settings. A viewer's saved preferences override them.
	Defaults ViewDefaults
	// Verdict, when set, replaces the computed headline, for example "Passed with known failures".
	// The page then also shows the result computed from the steps.
	Verdict *Verdict
	// Limits bound the input (RenderDir) and the inline attachment size (all renders).
	Limits cucumber.Limits
}

// Fact is one label and value pair.
type Fact struct {
	Label, Value string
}

// Verdict is a caller-chosen headline. Status (one of the six) sets the symbol and color; Label
// defaults to the status name; Explanation is an optional line below the headline.
type Verdict struct {
	Status             cucumber.Status
	Label, Explanation string
}

// FeatureAnchor is the stable in-page anchor (element id) of the feature at index i (zero-based) of
// the rendered features. Callers can use it to refer to a feature's card.
func FeatureAnchor(i int) string { return "f" + strconv.Itoa(i+1) }

// ScenarioAnchor is the stable in-page anchor (element id) of scenario j (zero-based) of feature i
// (zero-based).
func ScenarioAnchor(i, j int) string { return FeatureAnchor(i) + "-s" + strconv.Itoa(j+1) }

// Tone values for a Section or a Segment.
const (
	ToneInfo = "info"
	ToneOK   = "ok"
	ToneWarn = "warn"
	ToneFail = "fail"
)

// Section is a caller-owned panel. ID is a stable identifier of 1 to 48 characters from a-z, 0-9 and
// "-", starting with a letter; it names the section in settings and in ViewDefaults.Sections.
type Section struct {
	ID, Title string
	// Tone is ToneInfo (default), ToneOK, ToneWarn or ToneFail.
	Tone string
	// Summary facts are shown as a compact list at the top of the panel.
	Summary []Fact
	// Rows are shown as a two-column table.
	Rows []Fact
	// Note is a closing paragraph.
	Note string
	// Chart, when set, adds a stacked bar to the chart grid with ChartID SectionChartID(ID).
	Chart *ChartSeries
}

// ChartSeries is a caller-supplied stacked bar, for example known failures compared with failures.
type ChartSeries struct {
	Title    string
	Segments []Segment
}

// MaxSegmentValue bounds Segment.Value so that a series total cannot overflow.
const MaxSegmentValue = 1_000_000_000

// Segment is one part of a ChartSeries. Value is between 0 and MaxSegmentValue.
type Segment struct {
	Label string
	Value int
	// Tone colors the segment: ToneInfo (default), ToneOK, ToneWarn or ToneFail.
	Tone string
}

// ChartID names a chart in settings and in ViewDefaults.Charts.
type ChartID string

// Built-in charts.
const (
	ChartScenarioStatus   ChartID = "scenario-status"
	ChartStepStatus       ChartID = "step-status"
	ChartFeatureResults   ChartID = "feature-results"
	ChartSlowestSteps     ChartID = "slowest-steps"
	ChartSlowestScenarios ChartID = "slowest-scenarios"
	ChartFeatureDuration  ChartID = "feature-duration"
)

// BuiltinCharts lists the built-in charts in display order.
var BuiltinCharts = []ChartID{ChartScenarioStatus, ChartStepStatus, ChartFeatureResults, ChartSlowestSteps, ChartSlowestScenarios, ChartFeatureDuration}

// SectionChartID is the ChartID of a section's chart.
func SectionChartID(sectionID string) ChartID { return ChartID("section-" + sectionID) }

// View setting values.
const (
	ThemeAuto  = "auto"
	ThemeLight = "light"
	ThemeDark  = "dark"

	DensityComfortable = "comfortable"
	DensityCompact     = "compact"

	ExpandFailed = "failed"
	ExpandNone   = "none"
	ExpandAll    = "all"

	// PageSizeAll shows every scenario and feature row without paging.
	PageSizeAll = -1
	// DefaultPageSize applies when ViewDefaults.PageSize is zero.
	DefaultPageSize = 50
)

// ViewDefaults are the generation-time settings. Zero values select: every chart and section shown,
// ThemeAuto, DensityComfortable, ExpandFailed and DefaultPageSize.
type ViewDefaults struct {
	// Charts hides a chart when its entry is false. Keys must name a built-in or section chart.
	Charts map[ChartID]bool
	// Sections hides a section when its entry is false. Keys must name a section in Options.Sections.
	Sections map[string]bool
	// Theme is ThemeAuto, ThemeLight or ThemeDark.
	Theme string
	// Density is DensityComfortable or DensityCompact.
	Density string
	// Expand selects which scenario cards start open: ExpandFailed, ExpandNone or ExpandAll.
	Expand string
	// PageSize is 25, 50, 100 or PageSizeAll; zero selects DefaultPageSize.
	PageSize int
}

var sectionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidOptions, fmt.Sprintf(format, args...))
}

func validTone(t string) bool {
	switch t {
	case "", ToneInfo, ToneOK, ToneWarn, ToneFail:
		return true
	}
	return false
}

// normalize validates opt and returns a copy with defaults applied. Error messages name the field
// and index only.
func normalize(opt Options) (Options, error) {
	if opt.Version != OptionsVersion {
		return opt, ErrOptionsVersion
	}
	if opt.Title == "" {
		opt.Title = "Cucumber report"
	}
	if opt.Name == "" {
		opt.Name = opt.Title
	}
	opt.Limits = opt.Limits.WithDefaults()
	if v := opt.Verdict; v != nil && !v.Status.Valid() {
		return opt, invalid("verdict.status")
	}

	charts := map[ChartID]bool{}
	for _, id := range BuiltinCharts {
		charts[id] = true
	}
	sections := map[string]bool{}
	for i, s := range opt.Sections {
		if !sectionIDPattern.MatchString(s.ID) {
			return opt, invalid("sections[%d].id", i)
		}
		if sections[s.ID] {
			return opt, invalid("sections[%d].id is a duplicate", i)
		}
		sections[s.ID] = true
		if s.Title == "" {
			return opt, invalid("sections[%d].title is empty", i)
		}
		if !validTone(s.Tone) {
			return opt, invalid("sections[%d].tone", i)
		}
		if s.Chart != nil {
			for j, seg := range s.Chart.Segments {
				if seg.Value < 0 || seg.Value > MaxSegmentValue {
					return opt, invalid("sections[%d].chart.segments[%d].value is outside 0 to MaxSegmentValue", i, j)
				}
				if !validTone(seg.Tone) {
					return opt, invalid("sections[%d].chart.segments[%d].tone", i, j)
				}
			}
			charts[SectionChartID(s.ID)] = true
		}
	}

	d := opt.Defaults
	for id := range d.Charts {
		if !charts[id] {
			return opt, invalid("defaults.charts names an unknown chart")
		}
	}
	for id := range d.Sections {
		if !sections[id] {
			return opt, invalid("defaults.sections names an unknown section")
		}
	}
	switch d.Theme {
	case "":
		d.Theme = ThemeAuto
	case ThemeAuto, ThemeLight, ThemeDark:
	default:
		return opt, invalid("defaults.theme")
	}
	switch d.Density {
	case "":
		d.Density = DensityComfortable
	case DensityComfortable, DensityCompact:
	default:
		return opt, invalid("defaults.density")
	}
	switch d.Expand {
	case "":
		d.Expand = ExpandFailed
	case ExpandFailed, ExpandNone, ExpandAll:
	default:
		return opt, invalid("defaults.expand")
	}
	switch d.PageSize {
	case 0:
		d.PageSize = DefaultPageSize
	case 25, 50, 100, PageSizeAll:
	default:
		return opt, invalid("defaults.pageSize")
	}
	opt.Defaults = d
	return opt, nil
}
