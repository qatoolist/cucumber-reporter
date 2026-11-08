package report

import (
	"fmt"
	"regexp"
	"time"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	model "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
	"github.com/qatoolist/cucumber-reporter/internal/evidence"
)

func checkDisplayedEvidence(features []cucumber.Feature, page *pageData) error {
	if err := checkScenarioOutcomes(features, page, nil, nil); err != nil {
		return err
	}
	raw := model.Evidence(features)
	var shown evidence.Counts
	for _, f := range page.Features {
		for _, sc := range f.Elements {
			strictPass := len(sc.Steps) > 0
			for _, st := range sc.Steps {
				shown.Add(st.Status)
				strictPass = strictPass && st.Status == "passed"
			}
			priorFailure := false
			for i, attempt := range page.Execution[sc.ID] {
				if i == len(page.Execution[sc.ID])-1 || attempt.UseScenarioSteps {
					continue
				}
				for _, st := range attempt.Steps {
					shown.Add(st.Status)
					priorFailure = priorFailure || st.Status == "failed"
				}
			}
			if priorFailure && !strictPass && sc.Status == "passed" {
				return evidence.Floorf("evidence floor: failed retry was not rescued by a strictly passing final attempt; internal consistency check failed, so the report was refused (please report this with the input)")
			}
		}
	}
	return evidence.Check(raw, shown)
}

// checkScenarioOutcomes refuses scenario outcomes that the raw evidence does not
// support. cfg, when the run configuration is available, lets the floor re-derive
// which known-issue annotations its issue policies exclude. orig, when redaction
// ran, supplies the unmasked step evidence and annotations for those checks.
func checkScenarioOutcomes(features []cucumber.Feature, page *pageData, cfg *RunConfig, orig *originalEvidence) error {
	if page.Analysis != nil && len(page.Aliases) > 0 {
		return evidence.Floorf("evidence floor: enhanced report contains plain navigation aliases; internal consistency check failed, so the report was refused (please report this with the input)")
	}

	if page.Total == 0 && page.VerdictSource != "caller" && page.Verdict.Tone == ToneOK {
		return evidence.Floorf("evidence floor: empty execution cannot be green; internal consistency check failed, so the report was refused (please report this with the input)")
	}
	if model.RunnerFailed(features) && page.Verdict.Tone != ToneFail {
		return evidence.Floorf("evidence floor: runner failure signal was lost; internal consistency check failed, so the report was refused (please report this with the input)")
	}
	ids := scenarioIDs(features)
	source := map[string]struct {
		uri string
		sc  cucumber.Scenario
	}{}
	for i, f := range features {
		for j, sc := range f.Elements {
			source[ids[i][j]] = struct {
				uri string
				sc  cucumber.Scenario
			}{f.URI, sc}
		}
	}
	annotations := make([]Annotation, len(page.Annotations))
	for i, a := range page.Annotations {
		annotations[i] = a.Annotation
	}
	if orig != nil {
		annotations = orig.annotations
	}
	for _, f := range page.Features {
		for _, sc := range f.Elements {
			original, ok := source[sc.ID]
			if !ok {
				return evidence.Floorf("evidence floor: unknown scenario %s; internal consistency check failed, so the report was refused (please report this with the input)", sc.ID)
			}
			raw := original.sc
			if orig != nil {
				if raw, ok = orig.scenarios[sc.ID]; !ok || len(raw.Steps) != len(original.sc.Steps) {
					return evidence.Floorf("evidence floor: no unmasked evidence for scenario %s; internal consistency check failed, so the report was refused (please report this with the input)", sc.ID)
				}
			}
			// Coverage is re-derived from the annotations and the unmasked step text
			// and error, not taken from the page's own Known marks.
			covered := annotatedKnownSteps(annotations, sc.ID, raw, cfg)
			known := map[int]bool{}
			for i, st := range sc.Steps {
				if st.Known && covered[i+1] {
					known[i+1] = true
				}
			}
			if err := model.CheckOutcome(raw, sc.Status, known); err != nil {
				return fmt.Errorf("%s:%d: %w", original.uri, original.sc.Line, err)
			}
		}
	}
	if page.VerdictSource != "caller" && page.Verdict.Tone == ToneOK {
		if page.Analysis != nil && (page.Analysis.RunnerFailed || len(page.Analysis.GateFailures) > 0) {
			return evidence.Floorf("evidence floor: green verdict despite runner or gate failure; internal consistency check failed, so the report was refused (please report this with the input)")
		}
		for _, f := range page.Features {
			for _, sc := range f.Elements {
				if failedOutcome(sc.Status) {
					return evidence.Floorf("%s:%d: evidence floor: green run despite failed scenario; internal consistency check failed, so the report was refused (please report this with the input)", f.URI, sc.Line)
				}
			}
		}
	}
	return nil
}

// annotatedKnownSteps lists the raw step positions that an applicable known-issue
// annotation names for scenario id with the declared step text.
func annotatedKnownSteps(annotations []Annotation, id string, sc cucumber.Scenario, cfg *RunConfig) map[int]bool {
	out := map[int]bool{}
	for _, a := range annotations {
		if a.Kind != KindKnown || a.Scenario != id || a.PolicyReason != "" || (a.Step == 0 && a.BackgroundStep == 0) {
			continue
		}
		position := a.Step
		if enhancedSteps(sc) {
			position = annotationSourcePosition(sc, a)
		}
		if position < 1 || position > len(sc.Steps) || clean(sc.Steps[position-1].Name) != clean(a.StepText) {
			continue
		}
		if cfg != nil && issuePolicyExcludes(*cfg, a.Ref, sc.Steps[position-1].Result.Error) {
			continue
		}
		out[position] = true
	}
	return out
}

// issuePolicyExcludes independently re-derives, from the raw run configuration,
// whether an issue policy for ref makes the annotation inapplicable: another
// environment, an exception that expired before the run was recorded, or a raw
// step error that does not match the expected signature. It deliberately does
// not read the PolicyReason written by applyIssuePolicies.
func issuePolicyExcludes(cfg RunConfig, ref, rawError string) bool {
	for _, p := range cfg.Issues {
		if p.Ref != ref && clean(p.Ref) != ref {
			continue
		}
		if len(p.Environments) > 0 {
			listed := false
			for _, env := range p.Environments {
				listed = listed || env == cfg.Environment
			}
			if !listed {
				return true
			}
		}
		if p.Expires != "" {
			expires, err := time.Parse("2006-01-02", p.Expires)
			recorded, rerr := time.Parse(time.RFC3339Nano, cfg.RecordedAt)
			// A run recorded on a later calendar day (in its own offset) is past expiry.
			if err != nil || rerr != nil || recorded.Format("2006-01-02") > expires.Format("2006-01-02") {
				return true
			}
		}
		if p.ErrorPattern != "" {
			re, err := regexp.Compile(p.ErrorPattern)
			if err != nil || !re.MatchString(rawError) {
				return true
			}
		}
	}
	return false
}

// The policy floor is independent of evaluateGates and checks the actual scenario
// outcomes, not summary counts. Disabling a gate rule must not publish green.
func checkPolicyFloor(summary RunSummary, policy GatePolicy) error {
	gateRequired := policy.Unexpected && summary.RunnerFailed
	scenarios := 0
	for _, s := range summary.Scenarios {
		if !s.RunHook {
			scenarios++
		}
		gateRequired = gateRequired || (policy.Unexpected && (s.Status == "failed" || (s.RunHook && (s.Status == "ambiguous" || s.Status == "undefined")))) || (policy.Pending && s.Status == "pending") || (policy.Undefined && (s.Status == "undefined" || s.Status == "ambiguous"))
		if policy.Flaky && len(s.Attempts) > 1 && s.Attempts[len(s.Attempts)-1].Status == "passed" {
			for _, a := range s.Attempts {
				gateRequired = gateRequired || a.Status != "passed"
			}
		}
	}
	if scenarios == 0 && summary.Computed.Tone == ToneOK {
		return evidence.Floorf("evidence floor: empty run cannot have a green verdict; internal consistency check failed, so the report was refused (please report this with the input)")
	}
	gateRequired = gateRequired || (policy.Empty && scenarios == 0) || (policy.Incomplete && summary.Completeness != "complete")
	if policy.NewFailures {
		gateRequired = gateRequired || (summary.Comparison == nil && !policy.AllowFirstRun) || (summary.Comparison != nil && len(summary.Comparison.NewFailures) > 0)
	}
	if policy.MaxRegressionPercent > 0 {
		if summary.Comparison == nil && !policy.AllowFirstRun {
			gateRequired = true
		}
		for _, sc := range summary.Scenarios {
			if sc.DurationComparable && durationRegressed(sc.DurationDelta, sc.Duration-sc.DurationDelta, policy) {
				gateRequired = true
			}
		}
	}
	if gateRequired && len(summary.GateFailures) == 0 {
		return evidence.Floorf("evidence floor: required quality gate did not fail; internal consistency check failed, so the report was refused (please report this with the input)")
	}
	if (summary.RunnerFailed || gateRequired) && summary.Computed.Tone != ToneFail {
		return evidence.Floorf("evidence floor: run verdict contradicts runner or quality policy; internal consistency check failed, so the report was refused (please report this with the input)")
	}
	return nil
}
