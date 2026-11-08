package report

import (
	"fmt"
	"path"
	"sort"
	"strings"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	model "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
	"github.com/qatoolist/cucumber-reporter/internal/evidence"
)

// Snapshot before mergeInputAttempts; no Status/Decide/folding helpers are used.
func rawExecution(sc input.Scenario) evidence.Execution {
	out := evidence.Execution{NoOwnSteps: sc.Type != "run_hook" && sc.Type != "orphan_background"}
	for _, step := range sc.Steps {
		origin := model.StepMetadata(&step).Origin
		if origin == "" || origin == "own" {
			out.NoOwnSteps = false
		}
	}
	var scan func([]input.Step, int)
	scan = func(steps []input.Step, depth int) {
		if depth > 64 {
			out.Results = append(out.Results, "failed")
			return
		}
		for _, s := range steps {
			out.Results = append(out.Results, string(s.Result.Status))
			scan(s.Before, depth+1)
			scan(s.After, depth+1)
		}
	}
	scan(sc.Before, 0)
	scan(sc.Steps, 0)
	scan(sc.After, 0)
	return out
}

func rawStreams(inputs []runInput) []evidence.Stream {
	out := make([]evidence.Stream, 0, len(inputs))
	for _, in := range inputs {
		stream := evidence.Stream{Name: in.Name, Cases: map[string][]evidence.Execution{}}
		// Separate raw ownership inventory: never use the production collision
		// detector or report identities to establish this invariant.
		observe := func(uri string, sc input.Scenario) {
			if strings.EqualFold(sc.Type, "background") || strings.EqualFold(sc.Type, "run_hook") {
				return
			}
			key := fmt.Sprintf("%q:%d", path.Clean(strings.ReplaceAll(uri, "\\", "/")), sc.Line)
			if sc.Line <= 0 {
				key += fmt.Sprintf(" (%q)", sc.Name)
			}
			stream.Locations = append(stream.Locations, key)
		}
		for _, a := range in.Attempts {
			observe(a.URI, a.Scenario)
		}
		for _, f := range in.Features {
			for _, sc := range f.Elements {
				observe(f.URI, sc)
			}
		}
		attempts := append([]input.MessageAttempt(nil), in.Attempts...)
		sort.SliceStable(attempts, func(i, j int) bool { return attempts[i].Number < attempts[j].Number })
		for _, a := range attempts {
			if a.Scenario.Type == "run_hook" {
				stream.Failed = stream.Failed || a.RunnerFailed
				for _, result := range rawExecution(a.Scenario).Results {
					if result == "failed" || result == "ambiguous" {
						stream.Unexplained = append(stream.Unexplained, "failed run hook")
					}
				}
				continue
			}
			state := model.ScenarioMetadata(&a.Scenario)
			if state.InferredRetryFailure || state.MissingStepResults {
				stream.Unexplained = append(stream.Unexplained, "runner omitted hook or execution results")
			}
			key := sourceLocation(a.URI, a.Scenario)
			stream.Cases[key] = append(stream.Cases[key], rawExecution(a.Scenario))
			stream.Failed = stream.Failed || a.RunnerFailed
		}
		for _, f := range in.Features {
			for _, s := range f.Elements {
				if s.Type == "background" {
					continue
				}
				key := sourceLocation(f.URI, s)
				for _, p := range model.ScenarioMetadata(&s).PreviousAttempts {
					stream.Cases[key] = append(stream.Cases[key], rawExecution(p))
				}
				stream.Cases[key] = append(stream.Cases[key], rawExecution(s))
				stream.Failed = stream.Failed || model.ScenarioMetadata(&s).RunnerFailed
			}
		}
		for _, attempts := range stream.Cases {
			if len(attempts) < 2 {
				continue
			}
			lastPassed := len(attempts[len(attempts)-1].Results) > 0
			for _, status := range attempts[len(attempts)-1].Results {
				lastPassed = lastPassed && status == "passed"
			}
			if lastPassed {
				stream.Unexplained = append(stream.Unexplained, "runner may have failed for strict-flaky")
			}
		}
		out = append(out, stream)
	}
	return out
}

func finalEvidence(features []input.Feature, statuses map[string]string) map[string]evidence.Final {
	out := map[string]evidence.Final{}
	for _, f := range features {
		for _, s := range f.Elements {
			key := sourceLocation(f.URI, s)
			v := evidence.Final{Status: statuses[key]}
			for _, p := range model.ScenarioMetadata(&s).PreviousAttempts {
				v.Attempts = append(v.Attempts, rawExecution(p))
			}
			v.Attempts = append(v.Attempts, rawExecution(s))
			out[key] = v
		}
	}
	return out
}

// Production reconciliation, kept separate from evidence.CheckOutcomes' oracle.
func unresolvedStream(stream evidence.Stream, final map[string]evidence.Final) bool {
	if !stream.Failed {
		return false
	}
	if len(stream.Unexplained) > 0 {
		return true
	}
	explained := false
	for key, attempts := range stream.Cases {
		v, ok := final[key]
		if !ok {
			return true
		}
		if len(attempts) == 0 {
			return true
		}
		last := attempts[len(attempts)-1]
		if last.NoOwnSteps || len(last.Results) == 0 {
			return true
		}
		// A known failure elsewhere cannot explain strict/pending policy for
		// this execution, even when another raw failure explains the exit too.
		for _, status := range last.Results {
			if status == "pending" || status == "undefined" || status == "unknown" {
				return true
			}
		}
		for _, status := range last.Results {
			if (status == "failed" || status == "ambiguous") && (v.Status == "failed" || v.Status == "ambiguous" || v.Status == "known") {
				explained = true
			}
		}
	}
	return !explained
}
