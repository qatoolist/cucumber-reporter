package report

import (
	"fmt"
	"path"
	"sort"
	"strings"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

func normalizedURI(uri string) string { return path.Clean(strings.ReplaceAll(uri, "\\", "/")) }
func sourceLocation(uri string, sc cucumber.Scenario) string {
	return fmt.Sprintf("%s\x00%d\x00%s", normalizedURI(uri), sc.Line, sc.Name)
}

// runScenarioIDs returns enhanced-mode references independent of shard order and
// subsets. Plain scenarioIDs retains its v0.2.0 contract.
func runScenarioIDs(features []cucumber.Feature) [][]string {
	copyFeatures := append([]cucumber.Feature(nil), features...)
	setRunNavigation(copyFeatures)
	return scenarioIDs(copyFeatures)
}

func setRunNavigation(features []cucumber.Feature) {
	for i := range features {
		uri := normalizedURI(features[i].URI)
		slug := featureSlug(uri)
		if uri != slug+".feature" {
			slug += "-" + strings.TrimPrefix(Identity(uri, "navigation"), "test-")[:12]
		}
		if modelmeta.FeatureMetadata(&features[i]).SyntheticRunHooks {
			slug = "run-hooks-" + strings.TrimPrefix(Identity(uri, "navigation"), "test-")[:12]
		}

		modelmeta.FeatureMetadata(&features[i]).ReportID = slug
	}
}

// Normalize runner attempts once, at the library boundary. The CLI only parses.
func mergeInputAttempts(inputs []runInput) ([]runInput, map[string]ScenarioMeta, error) {
	out := append([]runInput(nil), inputs...)
	metadata := map[string]ScenarioMeta{}
	for i, in := range out {
		out[i].Features = modelmeta.MergeLegacyRetries(in.Features)
		for fi, f := range out[i].Features {
			if reservedHookURI(f.URI) {
				return nil, nil, invalidInput("inputs[%d] uses the reserved synthetic run-hook namespace", i)
			}
			modelmeta.FeatureMetadata(&out[i].Features[fi]).SyntheticRunHooks = false
		}
		if len(in.Attempts) == 0 {
			for _, f := range out[i].Features {
				for _, sc := range f.Elements {
					previous := modelmeta.ScenarioMetadata(&sc).PreviousAttempts
					if sc.Type == "background" {
						continue
					}
					m := ScenarioMeta{}
					for n, attempt := range append(append([]cucumber.Scenario(nil), previous...), sc) {
						folded, e := foldScenarioSteps(attempt, modelmeta.ScenarioMetadata(&attempt).AttemptBackground)
						if e != nil {
							return nil, nil, e
						}
						m.Attempts = append(m.Attempts, Attempt{Number: n, Status: string(modelmeta.ExecutionStatus(folded)), Steps: publicSteps(folded.Steps, 0)})
					}
					metadata[sourceLocation(f.URI, sc)] = appendAttemptMetadata(metadata[sourceLocation(f.URI, sc)], m)
				}
			}
			continue
		}
		if len(in.Features) > 0 {
			return nil, nil, invalid("input cannot contain both features and attempts")
		}
		finalRaw := map[string]cucumber.Scenario{}
		failedSignal := false
		for _, a := range in.Attempts {
			finalRaw[sourceLocation(a.URI, a.Scenario)] = a.Scenario
			failedSignal = failedSignal || a.RunnerFailed || modelmeta.ScenarioMetadata(&a.Scenario).RunnerFailed
		}
		explained := false
		for _, s := range finalRaw {
			status := modelmeta.ExecutionStatus(s)
			explained = explained || status == cucumber.StatusFailed || status == cucumber.StatusAmbiguous
		}
		unresolved := failedSignal && !explained
		groups := map[string][]cucumber.MessageAttempt{}
		var order []string
		for _, a := range in.Attempts {
			if a.Scenario.Type != "run_hook" && reservedHookURI(a.URI) {
				return nil, nil, invalidInput("inputs[%d] uses the reserved synthetic run-hook namespace", i)
			}
			var err error
			modelmeta.ScenarioMetadata(&a.Scenario).RunnerFailed = unresolved
			a.Scenario, err = foldScenarioSteps(a.Scenario, nil)
			if err != nil {
				return nil, nil, err
			}
			if a.Scenario.Type == "run_hook" {
				a.URI = "__run_hooks__/" + strings.TrimPrefix(Identity(in.Name, "stream"), "test-") + ".feature"
				a.FeatureName = "Run hooks"
			}
			key := normalizedURI(a.URI) + "\x00" + a.Scenario.ID
			if _, ok := groups[key]; !ok {
				order = append(order, key)
			}
			groups[key] = append(groups[key], a)
		}
		featureIndex := map[string]int{}
		for _, key := range order {
			attempts := groups[key]
			sort.Slice(attempts, func(a, b int) bool { return attempts[a].Number < attempts[b].Number })
			last := attempts[len(attempts)-1]
			var floor []modelmeta.ExecutionEvidence
			for _, a := range attempts {
				floor = append(floor, modelmeta.ScenarioMetadata(&a.Scenario).OutcomeFloor...)
			}
			modelmeta.ScenarioMetadata(&last.Scenario).OutcomeFloor = floor
			for _, a := range attempts {
				modelmeta.ScenarioMetadata(&last.Scenario).InferredRetryFailure = modelmeta.ScenarioMetadata(&last.Scenario).InferredRetryFailure || modelmeta.ScenarioMetadata(&a.Scenario).InferredRetryFailure
				modelmeta.ScenarioMetadata(&last.Scenario).CaseAttachments = modelmeta.ScenarioMetadata(&last.Scenario).CaseAttachments || modelmeta.ScenarioMetadata(&a.Scenario).CaseAttachments
			}
			for _, a := range attempts[:len(attempts)-1] {
				modelmeta.ScenarioMetadata(&last.Scenario).PreviousAttempts = append(modelmeta.ScenarioMetadata(&last.Scenario).PreviousAttempts, a.Scenario)
			}
			uri := normalizedURI(last.URI)
			fi, ok := featureIndex[uri]
			if !ok {
				fi = len(out[i].Features)
				featureIndex[uri] = fi
				out[i].Features = append(out[i].Features, cucumber.Feature{URI: uri, Name: last.FeatureName, Description: last.FeatureDescription, Tags: last.FeatureTags})
				modelmeta.FeatureMetadata(&out[i].Features[fi]).SyntheticRunHooks = last.Scenario.Type == "run_hook"
				modelmeta.FeatureMetadata(&out[i].Features[fi]).IdentityCatalog = modelmeta.AttemptCatalog(&last)
			}
			out[i].Features[fi].Elements = append(out[i].Features[fi].Elements, last.Scenario)
			loc := sourceLocation(uri, last.Scenario)

			m := ScenarioMeta{Worker: last.Worker, Started: last.Started, Finished: last.Finished}
			for _, a := range attempts {
				if sourceLocation(a.URI, a.Scenario) != loc {
					return nil, nil, invalidInput("retry source location changed at %q:%d", a.URI, a.Scenario.Line)
				}
				m.Attempts = append(m.Attempts, Attempt{Number: a.Number, Status: string(modelmeta.ExecutionStatus(a.Scenario)), Started: a.Started, Finished: a.Finished, Worker: a.Worker, Steps: publicSteps(a.Scenario.Steps, 0)})
			}
			metadata[loc] = appendAttemptMetadata(metadata[loc], m)
		}
	}
	var err error
	metadata, err = cloneMetadata(metadata)
	if err != nil {
		return nil, nil, err
	}

	return out, metadata, nil
}

func reservedHookURI(uri string) bool { return strings.HasPrefix(normalizedURI(uri), "__run_hooks__/") }

func cloneMetadata(metadata map[string]ScenarioMeta) (map[string]ScenarioMeta, error) {
	if metadata == nil {
		return nil, nil
	}
	out := make(map[string]ScenarioMeta, len(metadata))
	for id, m := range metadata {
		m.Attempts = append([]Attempt(nil), m.Attempts...)
		for i := range m.Attempts {
			var err error
			var cloned []cucumber.Step
			cloned, err = modelmeta.CloneSteps(internalSteps(m.Attempts[i].Steps, 0))
			m.Attempts[i].Steps = publicSteps(cloned, 0)
			if err != nil {
				return nil, err
			}
		}
		out[id] = m
	}
	return out, nil
}

func appendAttemptMetadata(previous, next ScenarioMeta) ScenarioMeta {
	next.Attempts = append(previous.Attempts, next.Attempts...)
	for i := range next.Attempts {
		next.Attempts[i].Number = i
	}
	return next
}
