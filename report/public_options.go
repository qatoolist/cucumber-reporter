package report

import (
	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// Options configure an offline report. All fields are explicit public values;
// Go-only execution/analysis fields preserve prepared rendering state when copied.
// Version must be OptionsVersion.
type Options struct {
	// Execution contains prepared attempt evidence. It is trusted Go-only state.
	Execution map[string]ScenarioMeta `json:"-"`
	// PreparedDigest binds prepared analysis to its exact feature values. PrepareRun
	// sets it; do not edit it. Render refuses a mismatched snapshot.
	PreparedDigest string `json:"-"`
	// Analysis is trusted run intelligence, normally produced by PrepareRun.
	// Never decode it from untrusted JSON; Go callers are responsible for its truth.
	// CLI options files refuse it.
	Analysis *RunSummary `json:"-"`
	// BundleAttachments preserves oversized binary evidence as deduplicated sidecars.
	// CLI options files refuse it; use enhanced -bundle instead.
	BundleAttachments bool `json:"-"`

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
	Limits cucumber.Limits `json:"limits"`
}

func internalOptions(o Options) renderOptions {
	out := renderOptions{Analysis: o.Analysis, BundleAttachments: o.BundleAttachments, Version: o.Version, Title: o.Title, Name: o.Name, RunFacts: o.RunFacts, TopFacts: o.TopFacts, WallClock: o.WallClock, TimingSection: o.TimingSection, Sections: o.Sections, Annotations: o.Annotations, Defaults: o.Defaults, Verdict: o.Verdict, Limits: o.Limits}
	out.execution = o.Execution
	return out
}

func publicOptions(o renderOptions) Options {
	return Options{Analysis: o.Analysis, BundleAttachments: o.BundleAttachments, Version: o.Version, Title: o.Title, Name: o.Name, RunFacts: o.RunFacts, TopFacts: o.TopFacts, WallClock: o.WallClock, TimingSection: o.TimingSection, Sections: o.Sections, Annotations: o.Annotations, Defaults: o.Defaults, Verdict: o.Verdict, Limits: o.Limits}
}
