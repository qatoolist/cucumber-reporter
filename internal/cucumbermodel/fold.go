package cucumbermodel

import (
	"slices"
	"strconv"
	"strings"

	"github.com/qatoolist/cucumber-reporter/internal/evidence"
)

// FoldScenarioSteps assigns one execution order and scenario-owned step indices.
func FoldScenarioSteps(sc Scenario, background []Step) (Scenario, error) {
	CaptureOutcomeFloor(&sc, background)
	var out []Step
	own := 0
	var appendSteps func([]Step, string, int) error
	appendSteps = func(steps []Step, origin string, depth int) error {
		if depth > 64 {
			return ErrInvalid
		}
		for _, st := range steps {
			st.Before = markHookOwner(st.Before, own+1)
			if err := appendSteps(st.Before, "hook", depth+1); err != nil {
				return err
			}
			if StepMetadata(&st).Origin == "" {
				StepMetadata(&st).Origin = origin
			}
			if !slices.Contains([]string{"own", "background", "hook"}, StepMetadata(&st).Origin) {
				return ErrInvalid
			}
			StepMetadata(&st).OwnIndex = 0
			if StepMetadata(&st).Origin == "own" {
				own++
				StepMetadata(&st).OwnIndex = own
			}
			after := markHookOwner(st.After, own)
			st.Before = nil
			st.After = nil
			if st.stepState.Origin == "hook" {
				if st.stepState.HookStep == 0 {
					switch st.stepState.HookKind {
					case "after":
						st.stepState.HookStep = own
					case "before":
						st.stepState.HookStep = own + 1
					}
				}
				st.Embeddings = hookAttachments(st)
			}
			out = append(out, st)
			if err := appendSteps(after, "hook", depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, part := range []struct {
		steps  []Step
		origin string
	}{{sc.Before, "hook"}, {background, "background"}, {sc.Steps, "own"}, {sc.After, "hook"}} {
		if err := appendSteps(part.steps, part.origin, 0); err != nil {
			return sc, err
		}
	}
	sc.Before = nil
	sc.After = nil
	sc.Steps = out
	sc.scenarioState.AttemptBackground = nil
	sc.scenarioState.EnhancedFolded = true
	return sc, nil
}

// PlainFeatures folds standalone Background execution and makes every failing
// hook visible. Passing-only hook records keep the historical plain rendering.
func PlainFeatures(features []Feature) ([]Feature, error) {
	floor := Evidence(features)
	out := MergeLegacyRetries(features)
	for fi, f := range out {
		var background []Step
		var pending []string
		var orphanSource Scenario
		slots := map[int]int{}
		destinations := map[string]string{}
		for k, v := range f.featureState.BackgroundSlots {
			destinations[k] = v
		}
		elements := make([]Scenario, 0, len(f.Elements))
		for _, sc := range f.Elements {
			if sc.scenarioState.PlainFolded || sc.scenarioState.EnhancedFolded {
				elements = append(elements, sc)
				continue
			}
			slots[sc.Line]++
			suffix := sc.scenarioState.NavigationSuffix
			if suffix == "" {
				suffix = "-L" + strconv.Itoa(sc.Line)
			}
			if sc.scenarioState.NavigationSuffix == "" && slots[sc.Line] > 1 {
				suffix += "-" + strconv.Itoa(slots[sc.Line])
			}
			if sc.Type == "background" {
				bg, err := FoldScenarioSteps(sc, nil)
				if err != nil {
					return nil, err
				}
				for _, st := range bg.Steps {
					if st.stepState.Origin != "hook" || st.Result.Status != StatusPassed || len(st.Embeddings) > 0 {
						if st.stepState.Origin != "hook" {
							st.stepState = StepState{Origin: "background"}
						}
						background = append(background, st)
					}
				}
				pending = append(pending, suffix)
				orphanSource = sc
				continue
			}
			// Every plain scenario follows the same rules. Inline hooks keep their old
			// steps[] positions; external passing hooks are omitted before folding.
			for _, from := range pending {
				destinations[from] = suffix
			}
			pending = nil
			CaptureOutcomeFloor(&sc, background)
			own := append([]Step(nil), sc.Steps...)
			for i := range own {
				own[i].stepState.OwnIndex = i + 1
			}
			sc.Steps = own
			var flat []Step
			var logs []Embedding
			var add func([]Step, string, int) error
			add = func(steps []Step, origin string, depth int) error {
				if depth > 64 {
					return ErrInvalid
				}
				for _, st := range steps {
					effectiveOrigin := origin
					if st.stepState.Origin == "hook" && origin == "background" {
						effectiveOrigin = "hook"
					}
					st.Before = markHookOwner(st.Before, st.stepState.OwnIndex)
					if err := add(st.Before, "hook", depth+1); err != nil {
						return err
					}
					after := markHookOwner(st.After, st.stepState.OwnIndex)
					st.Before = nil
					st.After = nil
					if effectiveOrigin != "hook" || st.Result.Status != StatusPassed {
						if st.stepState.Origin == "" {
							st.stepState.Origin = origin
						}
						if origin != "own" {
							st.stepState.OwnIndex = 0
						}
						if effectiveOrigin == "hook" {
							st.Embeddings = hookAttachments(st)
						}
						st.Embeddings = append(append([]Embedding(nil), logs...), st.Embeddings...)
						logs = nil
						flat = append(flat, st)
					}
					if effectiveOrigin == "hook" && st.Result.Status == StatusPassed {
						named := hookAttachments(st)
						if st.stepState.HookKind == "after" && len(flat) > 0 {
							flat[len(flat)-1].Embeddings = append(flat[len(flat)-1].Embeddings, named...)
						} else {
							logs = append(logs, named...)
						}
					}
					if err := add(after, "hook", depth+1); err != nil {
						return err
					}
				}
				return nil
			}
			for _, part := range []struct {
				steps  []Step
				origin string
			}{{sc.Before, "hook"}, {background, "background"}, {sc.Steps, "own"}, {sc.After, "hook"}} {
				if err := add(part.steps, part.origin, 0); err != nil {
					return nil, err
				}
			}
			if len(logs) > 0 {
				if len(flat) > 0 {
					flat[len(flat)-1].Embeddings = append(flat[len(flat)-1].Embeddings, logs...)
				} else {
					flat = append(flat, Step{Name: "Runner output", Result: Result{Status: StatusPassed}, Embeddings: logs, stepState: StepState{Origin: "hook"}})
				}
			}
			sc.Before = nil
			sc.After = nil
			sc.Steps = flat
			sc.scenarioState.PlainFolded = true
			sc.scenarioState.NavigationSuffix = suffix
			background = nil
			elements = append(elements, sc)
		}
		// Orphan Backgrounds have no scenario to contribute to. Validation reports
		// their source location; they never create a passing scenario.
		if len(pending) > 0 {
			nonpassed := false
			for _, st := range background {
				nonpassed = nonpassed || st.Result.Status != StatusPassed
			}
			if nonpassed {
				orphan := orphanSource
				orphan.Type = "orphan_background"
				orphan.Steps = background
				orphan.Before = nil
				orphan.After = nil
				orphan.scenarioState = ScenarioState{PlainFolded: true, NavigationSuffix: pending[len(pending)-1]}
				elements = append(elements, orphan)
			}
		}
		out[fi].Elements = elements
		if len(pending) > 0 {
			out[fi].featureState.OrphanBackground = true
		}
		if len(destinations) > 0 {
			out[fi].featureState.BackgroundSlots = destinations
		}
	}
	if err := evidence.Check(floor, Evidence(out)); err != nil {
		return nil, err
	}
	return out, nil
}

// MergeLegacyRetries recognizes the two captured Ruby shapes. Navigation slots
// are assigned before any removal, including every Background and retry attempt.
func MergeLegacyRetries(features []Feature) []Feature {
	out := append([]Feature(nil), features...)
	for fi, f := range out {
		slots := map[int]int{}
		source := append([]Scenario(nil), f.Elements...)
		redirects := map[string]string{}
		for k, v := range f.featureState.BackgroundSlots {
			redirects[k] = v
		}
		for i := range source {
			sc := &source[i]
			slots[sc.Line]++
			if sc.scenarioState.NavigationSuffix == "" {
				suffix := "-L" + strconv.Itoa(sc.Line)
				if slots[sc.Line] > 1 {
					suffix += "-" + strconv.Itoa(slots[sc.Line])
				}
				sc.scenarioState.NavigationSuffix = suffix
			}
		}
		var elements, pending []Scenario
		for _, sc := range source {
			if sc.scenarioState.PlainFolded || sc.scenarioState.EnhancedFolded {
				elements = append(elements, sc)
				continue
			}
			if sc.Type == "background" {
				pending = append(pending, sc)
				continue
			}
			var bgSteps []Step
			for _, bg := range pending {
				flat, _ := FoldScenarioSteps(bg, nil)
				for _, st := range flat.Steps {
					if st.stepState.Origin == "own" {
						st.stepState.Origin = "background"
					}
					st.stepState.OwnIndex = 0
					bgSteps = append(bgSteps, st)
				}
			}
			sc.scenarioState.AttemptBackground = bgSteps
			n := len(elements)
			if n > 0 && sc.Line > 0 {
				prev := elements[n-1]
				confirmed := rubyExecution(prev) && rubyExecution(sc)
				if confirmed && prev.Type != "background" && prev.Line == sc.Line && prev.Name == sc.Name && prev.ID == sc.ID {
					history := append([]Scenario(nil), prev.scenarioState.PreviousAttempts...)
					prev.scenarioState.PreviousAttempts = nil
					sc.scenarioState.PreviousAttempts = append(history, prev)
					redirects[prev.scenarioState.NavigationSuffix] = sc.scenarioState.NavigationSuffix
					start := n - 1
					for start > 0 && elements[start-1].Type == "background" {
						start--
					}
					elements = elements[:start]
				}
			}
			for _, bg := range pending {
				redirects[bg.scenarioState.NavigationSuffix] = sc.scenarioState.NavigationSuffix
			}
			elements = append(elements, pending...)
			pending = nil
			elements = append(elements, sc)
		}
		elements = append(elements, pending...)
		for from, to := range redirects {
			for n := 0; n < len(redirects); n++ {
				next, ok := redirects[to]
				if !ok || next == to {
					break
				}
				to = next
			}
			redirects[from] = to
		}
		out[fi].Elements = elements
		out[fi].featureState.BackgroundSlots = redirects
	}
	return out
}

func rubyExecution(sc Scenario) bool {
	var has func([]Step) bool
	has = func(steps []Step) bool {
		for _, st := range steps {
			if strings.Contains(st.Match.Location, ".rb:") || has(st.Before) || has(st.After) {
				return true
			}
		}
		return false
	}
	return has(sc.Steps) || has(sc.Before) || has(sc.After)
}

// ExecutionStatus is the raw final attempt outcome, excluding retry history.
func ExecutionStatus(sc Scenario) Status { return executionStatus(sc) }

func markHookOwner(steps []Step, owner int) []Step {
	out := slices.Clone(steps)
	for i := range out {
		out[i].stepState.HookStep = owner
	}
	return out
}

func hookAttachments(st Step) []Embedding {
	out := slices.Clone(st.Embeddings)
	kind := st.stepState.HookKind
	if kind == "" {
		kind = "runner"
	}
	prefix := strings.ToUpper(kind[:1]) + kind[1:] + " hook"
	for i := range out {
		label := prefix
		if strings.HasPrefix(out[i].Name, prefix+" ") {
			continue
		}
		if strings.HasPrefix(out[i].MimeType, "image/") {
			label += " screenshot"
		} else {
			label += " output"
		}
		if out[i].Name != "" {
			out[i].Name = label + ": " + out[i].Name
		} else {
			out[i].Name = label
		}
	}
	return out
}
