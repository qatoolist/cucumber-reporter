package cucumber

import (
	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

func internalFeatures(features []Feature) []modelmeta.Feature {
	if features == nil {
		return nil
	}
	out := make([]modelmeta.Feature, len(features))
	for i, f := range features {
		out[i] = modelmeta.Feature{URI: f.URI, ID: f.ID, Keyword: f.Keyword, Name: f.Name, Description: f.Description, Line: f.Line, Tags: internalTags(f.Tags)}
		*modelmeta.FeatureMetadata(&out[i]) = internalFeatureProvenance(f.Provenance)
		if f.Elements != nil {
			out[i].Elements = make([]modelmeta.Scenario, len(f.Elements))
		}
		for j, s := range f.Elements {
			out[i].Elements[j] = internalScenario(s)
		}
	}
	return out
}

func internalTags(tags []Tag) []modelmeta.Tag {
	if tags == nil {
		return nil
	}
	out := make([]modelmeta.Tag, len(tags))
	for i, t := range tags {
		out[i] = modelmeta.Tag(t)
	}
	return out
}

func internalScenario(s Scenario) modelmeta.Scenario {
	out := modelmeta.Scenario{ID: s.ID, Keyword: s.Keyword, Name: s.Name, Description: s.Description, Type: s.Type, Line: s.Line, Tags: internalTags(s.Tags), Steps: internalSteps(s.Steps, 0), Before: internalSteps(s.Before, 0), After: internalSteps(s.After, 0)}
	*modelmeta.ScenarioMetadata(&out) = internalScenarioProvenance(s.Provenance)
	return out
}

func internalSteps(steps []Step, depth int) []modelmeta.Step {
	if steps == nil {
		return nil
	}
	if depth > 64 {
		return []modelmeta.Step{{Result: modelmeta.Result{Status: modelmeta.StatusFailed, Error: "Hook nesting exceeds limit"}}}
	}
	out := make([]modelmeta.Step, len(steps))
	for i, s := range steps {
		t := modelmeta.Step{Keyword: s.Keyword, Name: s.Name, Line: s.Line, Match: modelmeta.Match(s.Match), Result: modelmeta.Result{Status: modelmeta.Status(s.Result.Status), Error: s.Result.Error, Duration: s.Result.Duration}, Before: internalSteps(s.Before, depth+1), After: internalSteps(s.After, depth+1)}
		if s.Hidden {
			modelmeta.StepMetadata(&t).Origin = "hook"
		}
		if s.DocString != nil {
			d := modelmeta.DocString(*s.DocString)
			t.DocString = &d
		}
		if s.Rows != nil {
			t.Rows = make([]modelmeta.Row, len(s.Rows))
			for j, r := range s.Rows {
				t.Rows[j] = modelmeta.Row(r)
			}
		}
		if s.Embeddings != nil {
			t.Embeddings = make([]modelmeta.Embedding, len(s.Embeddings))
			for j, e := range s.Embeddings {
				t.Embeddings[j] = modelmeta.Embedding(e)
			}
		}
		out[i] = t
		if s.Provenance != nil {
			*modelmeta.StepMetadata(&out[i]) = internalStepProvenance(s.Provenance)
		}
	}
	return out
}
