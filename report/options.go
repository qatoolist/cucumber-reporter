package report

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"
)

// OptionsVersion is the only Options.Version this release accepts.
const OptionsVersion = 2

// Errors returned by Render, RenderDir and RenderDirTo, in addition to the cucumber package's parse
// errors. Messages may name input files, feature URIs, lines and scenario names to locate a problem;
// compare errors with errors.Is rather than by text.
var (
	// ErrEvidence identifies a runtime contradiction between execution evidence and a report.
	ErrEvidence = errors.New("report: inconsistent execution evidence")
	// ErrInvalidInput identifies incompatible or inconsistent execution inputs.
	ErrInvalidInput = errors.New("report: invalid input")
	// ErrOptionsVersion: Options.Version is not OptionsVersion.
	ErrOptionsVersion = errors.New("report: unsupported options version")
	// ErrInvalidOptions: an option value is outside its documented set.
	ErrInvalidOptions = errors.New("report: invalid options")
	// ErrNoOutputDir identifies an absent output parent directory.
	ErrNoOutputDir = errors.New("report: output directory does not exist")
	// ErrNoInput: the run directory has no cucumber.json.
	ErrNoInput = errors.New("report: run directory has no cucumber.json")
	// ErrNotRegular: cucumber.json or a directory argument is a symbolic link, or not of the expected kind.
	ErrNotRegular = errors.New("report: input is a symbolic link or not a regular file or directory")
	// ErrEmptyOutput: rendering produced no bytes.
	ErrEmptyOutput = errors.New("report: renderer produced no report")
)

// renderOptions configure one render. The zero value is not valid: Version must be set. The JSON names are
// the format of the command's -options file, except the Go-only fields below.
type renderOptions struct {
	renderContext context.Context
	// Analysis is trusted run intelligence, normally produced by PrepareRun.
	// Never decode it from untrusted JSON; Go callers are responsible for its truth.
	// CLI options files refuse it.
	Analysis *RunSummary `json:"-"`
	// BundleAttachments preserves oversized binary evidence as deduplicated sidecars.
	// CLI options files refuse it; use enhanced -bundle instead.
	BundleAttachments bool `json:"-"`
	execution         map[string]ScenarioMeta

	// Version is the options schema version; it must be OptionsVersion (2).
	Version int `json:"version"`
	// Title is the document title and the brand line of the printed cover. Default "Cucumber report".
	Title string `json:"title,omitempty"`
	// Name is the run name shown in the top bar and as the printed heading. Default: Title.
	Name string `json:"name,omitempty"`
	// RunFacts are shown, in order, on the dashboard (the first six), under Run details and on the
	// printed cover.
	RunFacts []Fact `json:"runFacts,omitempty"`
	// TopFacts names, by label, up to three RunFacts whose values the top bar shows under the run name.
	// Empty: the first three RunFacts.
	TopFacts []string `json:"topFacts,omitempty"`
	// WallClock, when set, is the primary elapsed-time metric, for example "9m 35s". Cucumber JSON
	// has no start times, so the renderer cannot compute it. Surrounding whitespace is trimmed;
	// a whitespace-only value is treated as absent.
	WallClock string `json:"wallClock,omitempty"`
	// TimingSection optionally links the time card to a caller-supplied section.
	// Without it, the card opens the slowest-scenarios list.
	// It must be the ID of a section in Sections.
	TimingSection string `json:"timingSection,omitempty"`
	// Sections are caller-owned panels: compact cards on the dashboard and full panels under Run details.
	Sections []Section `json:"sections,omitempty"`
	// Annotations attach known issues, skip reasons and observation limits to scenarios.
	Annotations []Annotation `json:"annotations,omitempty"`
	// Defaults are the generation-time view settings. A viewer's saved preferences override them.
	Defaults ViewDefaults `json:"defaults"`
	// Verdict, when set, replaces the computed headline, for example "Passed with known failures".
	Verdict *Verdict `json:"verdict,omitempty"`
	// Limits bound the input size (RenderDir and RenderDirTo, which parse cucumber.json), the encoded
	// size of each attachment (Embedding: every render path, checked before decoding) and the inline
	// attachment size (all renders).
	Limits pub.Limits `json:"limits"`
}

// Fact is one label and value pair.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Tone values for a Verdict or a Section.
const (
	ToneInfo = "info"
	ToneOK   = "ok"
	ToneWarn = "warn"
	ToneFail = "fail"
)

// Verdict is a caller-chosen headline. Label is required. Tone colors the headline and picks its
// symbol (ToneFail a cross, ToneWarn a flag, otherwise a check); empty selects the tone the renderer
// computes from the results.
type Verdict struct {
	Label string `json:"label"`
	Tone  string `json:"tone,omitempty"`
}

// Section roles. A role tells the page that a section summarizes data it also receives as annotations.
const (
	// RoleKnownIssues: the section's summary facts and note appear on the Known issues page instead of
	// a dashboard card. At most one section may have this role.
	RoleKnownIssues = "known"
	// RoleSkipped: the section lists skipped scenarios.
	RoleSkipped = "skip"
	// RoleLimits: the section lists observation limits (scenarios not fully verified).
	RoleLimits = "limit"
)

// Section is a caller-owned panel. ID is a stable identifier of 1 to 48 characters from a-z, 0-9 and
// "-", starting with a letter; links to Run details use it (#/run?section=<id>).
type Section struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Tone is ToneInfo (default), ToneOK, ToneWarn or ToneFail.
	Tone string `json:"tone,omitempty"`
	// Role is empty, RoleKnownIssues, RoleSkipped or RoleLimits. Sections with a role are left out of
	// the printed scope sections, because their entries are printed from the annotations.
	Role string `json:"role,omitempty"`
	// Summary facts are shown as a compact list at the top of the panel.
	Summary []Fact `json:"summary,omitempty"`
	// Rows are shown as a two-column table.
	Rows []Fact `json:"rows,omitempty"`
	// Note is a closing paragraph; a dashboard card without summary or rows shows it instead.
	Note string `json:"note,omitempty"`
}

// Annotation kinds.
const (
	// KindKnown: a known issue covers the scenario.
	KindKnown = "known"
	// KindSkip: the reason a scenario was skipped.
	KindSkip = "skip"
	// KindLimit: part of what the scenario claims cannot be observed in the environment.
	KindLimit = "limit"
)

// Annotation attaches caller knowledge to one scenario, named by its ScenarioIDs or RunScenarioIDs value.
//
// Known issues follow these rules, applied by the renderer so that callers need no post-processing.
// A known issue covers only its declared step; any other unexpected result still fails.
//   - At most one KindKnown annotation per scenario and Ref.
//   - Step is the declared step, numbered from 1 over the scenario's steps (0: none declared), and
//     StepText is that step's text; it is required with Step. When the step at Step has a different
//     text, the annotation is shown with a warning and covers nothing.
//   - Observed reports that an expected-failure outcome was recorded at Step; it requires Step.
//   - A failed scenario is known only when every failure is covered by a matching declared
//     step. An unrelated failed step or hook remains unexpected; the annotation is still
//     shown as context. Earlier retry failures must satisfy the same coverage rule.
//   - A passed scenario with an observed declared step is a known failure (the expected failure was
//     recorded while the step passed). A passed scenario with only unobserved issues stays passed and
//     is flagged "the issue may be fixed", unless every outcome of the record is NotApplicable: then
//     it reads "not applicable in this run". Ambiguous, undefined, pending and skipped are unchanged.
//   - The known-failure marker sits on the declared step only: the failed step, or the observed
//     declared steps of a passed scenario. Outcomes are listed as detail and never mark a step.
//   - The default headline and the "As expected" figure count known failures as expected.
type Annotation struct {
	// BackgroundStep selects a folded Background step by its one-based position.
	// It is mutually exclusive with Step and requires StepText.
	BackgroundStep int `json:"backgroundStep,omitempty"`
	// Structured details are optional; Note remains supported verbatim.
	Details      []Fact `json:"details,omitempty"`
	Owner        string `json:"owner,omitempty"`
	Expires      string `json:"expires,omitempty"`
	ReviewDate   string `json:"reviewDate,omitempty"`
	PolicyReason string `json:"policyReason,omitempty"`

	Scenario string `json:"scenario"`
	// Kind is KindKnown, KindSkip or KindLimit.
	Kind string `json:"kind"`
	// Ref is the issue key of a known issue, for example "EX-101".
	Ref   string `json:"ref,omitempty"`
	Title string `json:"title,omitempty"`
	Note  string `json:"note,omitempty"`
	// Step is the declared step (KindKnown), from 1; 0 when none is declared.
	Step int `json:"step,omitempty"`
	// StepText is the declared step's text, without its keyword; required with Step.
	StepText string `json:"stepText,omitempty"`
	// Observed: an expected-failure outcome was recorded at Step (KindKnown only).
	Observed bool `json:"observed,omitempty"`
	// Outcomes are every outcome recorded for this issue in this scenario, shown as detail.
	Outcomes []Outcome `json:"outcomes,omitempty"`
}

// Outcome is one recorded known-issue outcome. Step is numbered from 1, 0 when it has no step.
// NotApplicable marks an outcome whose condition lay outside the run's scope. When the scenario passed
// and every outcome of its record is NotApplicable, the page shows "not applicable in this run" instead
// of suggesting that the issue may be fixed.
type Outcome struct {
	Step          int    `json:"step,omitempty"`
	Outcome       string `json:"outcome"`
	Text          string `json:"text,omitempty"`
	NotApplicable bool   `json:"notApplicable,omitempty"`
}

// View setting values.
const (
	ThemeAuto  = "auto"
	ThemeLight = "light"
	ThemeDark  = "dark"

	DensityComfortable = "comfortable"
	DensityCompact     = "compact"

	// ExpandFailed opens the first attachment of a failed step; ExpandAll opens every attachment;
	// ExpandNone opens none.
	ExpandFailed = "failed"
	ExpandAll    = "all"
	ExpandNone   = "none"

	ViewDashboard = "dashboard"
	ViewFeatures  = "features"
	ViewScenarios = "scenarios"
	ViewFailures  = "failures"
)

// Dashboard widgets, in display order. ViewDefaults.Widgets keys must be one of these.
const (
	WidgetHero      = "hero"
	WidgetKPIs      = "kpis"
	WidgetStatus    = "status"
	WidgetFeatures  = "features"
	WidgetAttention = "attention"
	WidgetKnown     = "known"
	WidgetDuration  = "duration"
	WidgetSections  = "sections"
)

// Widgets lists every dashboard widget in display order. Treat this compatibility
// slice as read-only; changing it does not alter validation.
var Widgets = []string{WidgetHero, WidgetKPIs, WidgetStatus, WidgetFeatures, WidgetAttention, WidgetKnown, WidgetDuration, WidgetSections}

// Navigation sections, in display order. ViewDefaults.Nav keys must be one of these; NavDashboard
// cannot be hidden.
const (
	NavDashboard = "dashboard"
	NavFeatures  = "features"
	NavScenarios = "scenarios"
	NavFailures  = "failures"
	NavKnown     = "known"
	NavSkipped   = "skipped"
	NavTimeline  = "timeline"
	NavRun       = "run"
)

// NavSections lists every navigation section in display order. Treat this
// compatibility slice as read-only; changing it does not alter validation.
var NavSections = []string{NavDashboard, NavFeatures, NavScenarios, NavFailures, NavKnown, NavSkipped, NavTimeline, NavRun}

// ViewDefaults are the generation-time settings. Zero values select ThemeAuto, DensityComfortable,
// ExpandFailed, ViewDashboard and every widget and navigation section shown.
type ViewDefaults struct {
	// Theme is ThemeAuto, ThemeLight or ThemeDark.
	Theme string `json:"theme,omitempty"`
	// Density is DensityComfortable or DensityCompact.
	Density string `json:"density,omitempty"`
	// Expand selects which attachments start open: ExpandFailed, ExpandAll or ExpandNone.
	Expand string `json:"expand,omitempty"`
	// View is the start page: ViewDashboard, ViewFeatures, ViewScenarios or ViewFailures.
	View string `json:"view,omitempty"`
	// Widgets hides a dashboard widget when its entry is false.
	Widgets map[string]bool `json:"widgets,omitempty"`
	// Nav hides a navigation section when its entry is false.
	Nav map[string]bool `json:"nav,omitempty"`
}

var sectionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidOptions, fmt.Sprintf(format, args...))
}

func validTone(t string) bool { return oneOf(t, "", ToneInfo, ToneOK, ToneWarn, ToneFail) }

func oneOf(v string, set ...string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// choose validates a setting and applies its default, the first value of set.
func choose(v *string, field string, set ...string) error {
	if *v == "" {
		*v = set[0]
		return nil
	}
	if !oneOf(*v, set...) {
		return invalid("defaults.%s", field)
	}
	return nil
}

// switches validates a widget or navigation map and returns it with every key present.
func switches(in map[string]bool, field string, keys []string) (map[string]bool, error) {
	for k := range in {
		if !oneOf(k, keys...) {
			return nil, invalid("defaults.%s names an unknown entry", field)
		}
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		on, set := in[k]
		out[k] = !set || on
	}
	return out, nil
}

// normalize validates opt without the features and returns a copy with defaults applied. Annotations
// are checked against the features by checkAnnotations. Error messages name the field and index only.
func normalize(opt renderOptions) (renderOptions, error) {
	if opt.Version != OptionsVersion {
		return opt, ErrOptionsVersion
	}
	if opt.Title == "" {
		opt.Title = "Cucumber report"
	}
	if opt.Name == "" {
		opt.Name = opt.Title
	}
	if opt.Analysis != nil {
		if err := validateSummary(*opt.Analysis); err != nil {
			return opt, err
		}
	}
	opt.WallClock = strings.TrimSpace(opt.WallClock)
	opt.Limits = opt.Limits.WithDefaults()
	if v := opt.Verdict; v != nil {
		if v.Label == "" {
			return opt, invalid("verdict.label is empty")
		}
		if !validTone(v.Tone) {
			return opt, invalid("verdict.tone")
		}
	}
	if len(opt.TopFacts) > 3 {
		return opt, invalid("topFacts names more than three facts")
	}
	for i, label := range opt.TopFacts {
		found := false
		for _, f := range opt.RunFacts {
			found = found || f.Label == label
		}
		if !found {
			return opt, invalid("topFacts[%d] names no run fact", i)
		}
	}
	ids := map[string]bool{}
	knownSections := 0
	for i, s := range opt.Sections {
		if !sectionIDPattern.MatchString(s.ID) {
			return opt, invalid("sections[%d].id", i)
		}
		if ids[s.ID] {
			return opt, invalid("sections[%d].id is a duplicate", i)
		}
		ids[s.ID] = true
		if s.Title == "" {
			return opt, invalid("sections[%d].title is empty", i)
		}
		if !validTone(s.Tone) {
			return opt, invalid("sections[%d].tone", i)
		}
		if !oneOf(s.Role, "", RoleKnownIssues, RoleSkipped, RoleLimits) {
			return opt, invalid("sections[%d].role", i)
		}
		if s.Role == RoleKnownIssues {
			knownSections++
		}
	}
	if opt.TimingSection != "" && !ids[opt.TimingSection] {
		return opt, invalid("timingSection must name an existing section")
	}
	if knownSections > 1 {
		return opt, invalid("more than one section has the known-issues role")
	}
	for i, a := range opt.Annotations {
		if !oneOf(a.Kind, KindKnown, KindSkip, KindLimit) {
			return opt, invalid("annotations[%d].kind", i)
		}
		if a.Kind != KindKnown && (a.Step != 0 || a.BackgroundStep != 0 || a.StepText != "" || a.Observed || len(a.Outcomes) > 0) {
			return opt, invalid("annotations[%d]: step, stepText, observed and outcomes apply to known issues only", i)
		}
		if a.BackgroundStep < 0 || (a.Step != 0 && a.BackgroundStep != 0) {
			return opt, invalid("annotations[%d]: select either step or backgroundStep", i)
		}
		if (a.Step != 0 || a.BackgroundStep != 0) != (a.StepText != "") {
			return opt, invalid("annotations[%d]: step and stepText go together", i)
		}
		if a.Observed && a.Step == 0 && a.BackgroundStep == 0 {
			return opt, invalid("annotations[%d].observed needs a declared step", i)
		}
	}

	d := opt.Defaults
	for _, c := range []struct {
		v     *string
		field string
		set   []string
	}{
		{&d.Theme, "theme", []string{ThemeAuto, ThemeLight, ThemeDark}},
		{&d.Density, "density", []string{DensityComfortable, DensityCompact}},
		{&d.Expand, "expand", []string{ExpandFailed, ExpandAll, ExpandNone}},
		{&d.View, "view", []string{ViewDashboard, ViewFeatures, ViewScenarios, ViewFailures}},
	} {
		if err := choose(c.v, c.field, c.set...); err != nil {
			return opt, err
		}
	}
	var err error
	if d.Widgets, err = switches(d.Widgets, "widgets", []string{WidgetHero, WidgetKPIs, WidgetStatus, WidgetFeatures, WidgetAttention, WidgetKnown, WidgetDuration, WidgetSections}); err != nil {
		return opt, err
	}
	if d.Nav, err = switches(d.Nav, "nav", []string{NavDashboard, NavFeatures, NavScenarios, NavFailures, NavKnown, NavSkipped, NavTimeline, NavRun}); err != nil {
		return opt, err
	}
	if !d.Nav[NavDashboard] {
		return opt, invalid("defaults.nav cannot hide the dashboard")
	}
	opt.Defaults = d
	return opt, nil
}

// checkAnnotations validates the annotations against the scenarios: steps maps each scenario ID to
// its number of steps.
func checkAnnotations(anns []Annotation, steps map[string]int) error {
	type key struct{ scenario, ref string }
	seen := map[key]bool{}
	for i, a := range anns {
		n, ok := steps[a.Scenario]
		if !ok {
			return invalid("annotations[%d].scenario names no scenario", i)
		}
		if a.Step < 0 || a.Step > n {
			return invalid("annotations[%d].step is outside the scenario's steps", i)
		}
		for j, o := range a.Outcomes {
			if o.Step < 0 || o.Step > n {
				return invalid("annotations[%d].outcomes[%d].step is outside the scenario's steps", i, j)
			}
		}
		if a.Kind == KindKnown {
			k := key{a.Scenario, a.Ref}
			if seen[k] {
				return invalid("annotations[%d] repeats a known issue for the same scenario", i)
			}
			seen[k] = true
		}
	}
	return nil
}

func invalidInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, args...))
}
